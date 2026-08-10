package sweep

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultAPI is where the runs and the tracker are read. It is a field on
// Client rather than a constant used directly, so a test can point the same
// code at a server on the loopback interface and the raising half is exercised
// without anything being written to a tracker.
const DefaultAPI = "https://api.github.com"

// Recent is how many runs of each watched workflow are read.
//
// A window rather than the whole history, because the question is whether the
// thing is failing now. A failure that scrolled out of the window is one whose
// tracking issue was already raised while it was inside it, and if it was not,
// the next failing run brings it back.
const Recent = 20

// errNotFound is what a read of an address the server does not know answers
// with, so that one caller can read it as a state and every other caller keeps
// treating it as a read that did not happen.
var errNotFound = errors.New("not found")

// Client reads workflow runs and the tracker, and raises an issue.
//
// The repository is a field read from the environment by the caller rather than
// a name written here: no-hardcoded-names refuses an account or organisation
// name in this source, and the reason it does is the same reason this is a
// field, which is that pointing the tool at a second repository should not be
// an edit to a Go file.
type Client struct {
	HTTP       *http.Client
	API        string
	Token      string
	Repository string
}

// New returns a client with a timeout, because a request with no deadline is a
// sweep that hangs rather than one that fails and says so.
func New() *Client {
	return &Client{
		HTTP: &http.Client{Timeout: 30 * time.Second},
		API:  DefaultAPI,
	}
}

// DefaultBranch reads the branch a scheduled run runs on.
//
// It is read rather than assumed. A sweep carrying the name of the default
// branch would go silent on the day it is renamed, and going silent is the
// failure this whole package exists against.
func (c *Client) DefaultBranch(ctx context.Context) (string, error) {
	var body struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := c.get(ctx, c.repoURL(""), &body); err != nil {
		return "", err
	}
	if body.DefaultBranch == "" {
		return "", fmt.Errorf("the repository names no default branch")
	}
	return body.DefaultBranch, nil
}

type apiRun struct {
	RunNumber  int    `json:"run_number"`
	Event      string `json:"event"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HeadBranch string `json:"head_branch"`
	HTMLURL    string `json:"html_url"`
	RunStarted string `json:"run_started_at"`
	Path       string `json:"path"`
}

// ErrNoHistory is what a workflow the server has no run history for answers
// with.
//
// It is a state rather than an error in the reading. A file that landed and has
// not reached its first cron yet is not registered on the server at all, and so
// is one whose schedule never fires. Treating that as a failed read would stop
// the whole sweep on the day a scheduled workflow is added, which is the
// silence this exists against, one rung up.
var ErrNoHistory = errors.New("the server holds no run history for this workflow")

// Runs reads the recent runs of one workflow, named by its path.
//
// The request narrows to the schedule event and the API answers with what it
// has. The same narrowing is applied again in Select, which is where it is
// tested: a filter that lives only in a query string is one no fixture can trip,
// and the day the parameter is dropped or renamed the sweep would start raising
// issues against runs somebody asked for.
func (c *Client) Runs(ctx context.Context, workflow string) ([]Run, error) {
	var body struct {
		Runs []apiRun `json:"workflow_runs"`
	}
	address := c.repoURL(fmt.Sprintf("/actions/workflows/%s/runs?event=schedule&per_page=%d",
		url.PathEscape(pathBase(workflow)), Recent))
	if err := c.get(ctx, address, &body); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, fmt.Errorf("%s: %w", workflow, ErrNoHistory)
		}
		return nil, err
	}

	out := make([]Run, 0, len(body.Runs))
	for _, r := range body.Runs {
		// The path the API reports is authoritative for which file declared the
		// run. Taking the workflow that was asked for would make a renamed file
		// answer under its old name for as long as old runs are in the window.
		declared := r.Path
		if declared == "" {
			declared = workflow
		}
		out = append(out, Run{
			Workflow:   declared,
			Number:     r.RunNumber,
			Event:      r.Event,
			Status:     r.Status,
			Conclusion: r.Conclusion,
			Branch:     r.HeadBranch,
			URL:        r.HTMLURL,
			StartedAt:  r.RunStarted,
		})
	}
	return out, nil
}

// OpenIssues reads the open issues so that a failure already tracked is not
// raised a second time.
//
// Pull requests are dropped. The tracker returns them from the same endpoint,
// and a pull request body quoting a key would silence the sweep for that
// workflow.
func (c *Client) OpenIssues(ctx context.Context) ([]Issue, error) {
	var out []Issue
	for page := 1; page <= 10; page++ {
		var body []struct {
			Number      int             `json:"number"`
			Body        string          `json:"body"`
			PullRequest json.RawMessage `json:"pull_request"`
		}
		address := c.repoURL(fmt.Sprintf("/issues?state=open&per_page=100&page=%d", page))
		if err := c.get(ctx, address, &body); err != nil {
			return nil, err
		}
		if len(body) == 0 {
			return out, nil
		}
		for _, i := range body {
			if len(i.PullRequest) > 0 && string(i.PullRequest) != "null" {
				continue
			}
			out = append(out, Issue{Number: i.Number, Body: i.Body})
		}
	}
	return out, nil
}

// Raise opens the tracking issue for one failure and returns its number.
//
// The label is carried because an issue with no label is one nobody filters to.
// There is no assignee and no milestone, and that is a limit rather than an
// oversight: a milestone is a plan and a run that failed last night is not part
// of one, and the login that would be assigned is a person's name, which this
// tree keeps out of its source. Whoever triages the label is who holds it.
func (c *Client) Raise(ctx context.Context, f Failure, label string) (int, error) {
	payload := map[string]any{
		"title": f.Title(),
		"body":  f.Body(),
	}
	if label != "" {
		payload["labels"] = []string{label}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.repoURL("/issues"), bytes.NewReader(encoded))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.headers(req)

	resp, err := c.do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("raising an issue for %s answered %s", f.Key(), resp.Status)
	}
	var created struct {
		Number int `json:"number"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return 0, fmt.Errorf("reading the raised issue: %w", err)
	}
	return created.Number, nil
}

func (c *Client) repoURL(suffix string) string {
	return fmt.Sprintf("%s/repos/%s%s", strings.TrimRight(c.API, "/"), c.Repository, suffix)
}

func (c *Client) get(ctx context.Context, address string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	c.headers(req)

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode != http.StatusOK {
		// Every other status is a read that did not happen. A sweep that read
		// nothing and a sweep that read a clean history print the same thing if
		// this is soft, and the second one is the state it exists to report.
		return fmt.Errorf("%s answered %s", address, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

func (c *Client) headers(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
}

// pathBase is the file name of a workflow path. The runs endpoint is addressed
// by file name rather than by the path the tree carries.
func pathBase(workflow string) string {
	if cut := strings.LastIndexByte(workflow, '/'); cut >= 0 {
		return workflow[cut+1:]
	}
	return workflow
}
