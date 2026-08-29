package carry

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultAPI is where the repository is read and written. It is a field on
// Client rather than a constant used directly, so a test can point the same
// code at a server on the loopback interface and the writing half is exercised
// without anything reaching a repository.
const DefaultAPI = "https://api.github.com"

// ContentLimit is the size above which the contents endpoint stops returning a
// file's bytes.
//
// It is here because this package compares content rather than a digest, and
// that comparison silently becomes "these differ" for any file past the limit.
// The catalogue is three kilobytes, so the bound is a long way off; it is stated
// so that a target pointed at something large is a thing somebody read about
// rather than a request reopened every night.
const ContentLimit = 1 << 20

// Client reads and writes one repository through its API.
//
// The repository is a field read from the environment by the caller rather than
// a name written here: no-hardcoded-names refuses an account or organisation
// name in this source, and the reason it does is the same reason this is a
// field, which is that pointing the tool at a second repository should not be an
// edit to a Go file.
type Client struct {
	HTTP       *http.Client
	API        string
	Token      string
	Repository string
}

// New returns a client with a timeout, because a request with no deadline is a
// publication run that hangs rather than one that fails and says so.
func New() *Client {
	return &Client{
		HTTP: &http.Client{Timeout: 30 * time.Second},
		API:  DefaultAPI,
	}
}

// DefaultBranch reads the branch the site is served from.
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

// Head reads the commit a branch points at.
//
// The single-ref endpoint is used rather than the matching one. The matching
// endpoint answers with a list and returns 200 and an empty list for a branch
// that does not exist, which is the same shape as a successful read and is how
// an absent branch becomes a nil error somewhere further down.
func (c *Client) Head(ctx context.Context, branch string) (string, error) {
	var body struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := c.get(ctx, c.repoURL("/git/ref/"+refPath(branch)), &body); err != nil {
		if errors.Is(err, errNotFound) {
			return "", fmt.Errorf("%s: %w", branch, ErrNoBranch)
		}
		return "", err
	}
	if body.Object.SHA == "" {
		return "", fmt.Errorf("%s resolves to no commit", branch)
	}
	return body.Object.SHA, nil
}

// File reads one path on one branch.
//
// An absent file and an absent branch answer alike here, and both come back as
// no content and no blob. That is deliberate: every caller of this asks the
// question "what does that branch carry at that path", and for a branch that
// does not exist the answer is nothing. Whether the branch exists is Head's
// question and is asked separately, where the distinction is acted on.
func (c *Client) File(ctx context.Context, branch, path string) ([]byte, string, error) {
	var body struct {
		SHA      string `json:"sha"`
		Size     int    `json:"size"`
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
		Type     string `json:"type"`
	}
	address := c.repoURL("/contents/" + escapePath(path) + "?ref=" + url.QueryEscape(branch))
	if err := c.get(ctx, address, &body); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, "", nil
		}
		return nil, "", err
	}
	if body.Type != "file" {
		return nil, "", fmt.Errorf("%s carries a %s at %s rather than a file", branch, body.Type, path)
	}
	// A file past the limit answers with its blob and no bytes, and comparing
	// against those absent bytes would report every such file as different on
	// every run. It is refused rather than guessed at.
	if body.Encoding != "base64" {
		return nil, "", fmt.Errorf("%s at %s came back %s-encoded, which is what a file over %d bytes answers with",
			path, branch, body.Encoding, ContentLimit)
	}
	content, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(body.Content, "\n", ""))
	if err != nil {
		return nil, "", fmt.Errorf("decoding %s at %s: %w", path, branch, err)
	}
	return content, body.SHA, nil
}

// OpenPull is the number of the open request from head, or zero.
//
// The head is qualified with the owner, which the endpoint requires and which
// is taken off the repository this client was pointed at rather than written
// here.
func (c *Client) OpenPull(ctx context.Context, head string) (int, error) {
	var body []struct {
		Number int `json:"number"`
	}
	address := c.repoURL("/pulls?state=open&head=" + url.QueryEscape(c.owner()+":"+head))
	if err := c.get(ctx, address, &body); err != nil {
		return 0, err
	}
	if len(body) == 0 {
		return 0, nil
	}
	return body[0].Number, nil
}

// Branch creates a branch at an existing commit.
func (c *Client) Branch(ctx context.Context, name, at string) error {
	payload := map[string]any{"ref": "refs/heads/" + name, "sha": at}
	return c.write(ctx, http.MethodPost, c.repoURL("/git/refs"), payload, http.StatusCreated, nil)
}

// Commit places content at path on branch and returns the commit it made.
//
// The contents endpoint is used rather than a blob, a tree, a commit and a ref
// update. Four calls where one does is the smaller reason; the larger one is
// that this endpoint moves the ref only where the file's current blob is the one
// named, so a branch that moved between the read and the write is refused by the
// server rather than overwritten. Nothing here can rewrite a history, which is
// the property this package promises and would otherwise be holding by care.
//
// The commit it makes is signed. That is not a convenience: the ruleset in front
// of the default branch requires verified signatures with no bypass, so a commit
// pushed from a runner with a key the runner does not hold could never be
// merged, and a mechanism that produced one would be building an unmergeable
// request every night.
func (c *Client) Commit(ctx context.Context, branch, path, message string, content []byte, replacing string) (string, error) {
	payload := map[string]any{
		"message": message,
		"content": base64.StdEncoding.EncodeToString(content),
		"branch":  branch,
	}
	if replacing != "" {
		payload["sha"] = replacing
	}

	var made struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	address := c.repoURL("/contents/" + escapePath(path))
	// Creating a file answers 201 and replacing one answers 200, and which of
	// the two it is here is not this method's question.
	if err := c.write(ctx, http.MethodPut, address, payload, 0, &made); err != nil {
		return "", err
	}
	if made.Commit.SHA == "" {
		return "", fmt.Errorf("the write to %s answered with no commit", path)
	}
	return made.Commit.SHA, nil
}

// Pull opens a request and returns its number.
func (c *Client) Pull(ctx context.Context, head, base, title, body string) (int, error) {
	payload := map[string]any{"title": title, "head": head, "base": base, "body": body}
	var opened struct {
		Number int `json:"number"`
	}
	if err := c.write(ctx, http.MethodPost, c.repoURL("/pulls"), payload, http.StatusCreated, &opened); err != nil {
		return 0, err
	}
	if opened.Number == 0 {
		return 0, fmt.Errorf("the request against %s was opened and came back with no number", base)
	}
	return opened.Number, nil
}

// errNotFound is what a read of an address the server does not know answers
// with, so that the two callers that read it as a state can, and every other
// one keeps treating it as a read that did not happen.
var errNotFound = errors.New("not found")

func (c *Client) owner() string {
	owner, _, _ := strings.Cut(c.Repository, "/")
	return owner
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
		// Every other status is a read that did not happen. A run that read
		// nothing and a run that read a current catalogue would otherwise print
		// the same thing, and the second one is the state this exists to
		// report.
		return fmt.Errorf("%s answered %s", address, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

// write sends a payload and refuses any status other than the one expected.
//
// want is zero where two statuses are both success for the same call, and there
// the 2xx range is what is accepted. It is a named status everywhere else,
// because a create that answered 200 is a call that did something other than
// what was asked.
func (c *Client) write(ctx context.Context, method, address string, payload any, want int, into any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, address, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.headers(req)

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	ok := resp.StatusCode == want
	if want == 0 {
		ok = resp.StatusCode >= 200 && resp.StatusCode < 300
	}
	if !ok {
		return fmt.Errorf("%s %s answered %s", method, address, resp.Status)
	}
	if into == nil {
		return nil
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

// refPath is a branch name as the single-ref endpoint addresses it.
//
// The slashes inside a branch name are part of the address rather than escaped,
// which is what that endpoint expects, and the standing branch this package
// derives carries two of them.
func refPath(branch string) string { return "heads/" + escapePath(branch) }

// escapePath escapes each segment of a slash-separated path and leaves the
// slashes alone.
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
