package sweep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Every test here runs against a server on the loopback interface, which is
// what decisions/headless-and-unelevated.md requires of anything the gate
// compiles. It is also what lets the raising half be exercised at all: a test
// that proved this by opening a real issue would prove it once and leave the
// issue behind.

// tracker is the part of the API this package reads and writes, standing up as
// a server. The runs it serves are supplied per workflow file name, so a test
// says which workflow failed and nothing else.
type tracker struct {
	repository string
	runs       map[string]string
	unknown    map[string]bool
	open       string
	raised     []map[string]any
	nextNumber int
}

func (tr *tracker) handler(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	base := "/repos/" + tr.repository

	mux.HandleFunc(base+"/issues", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("the raised issue does not decode: %v", err)
			}
			tr.raised = append(tr.raised, payload)
			tr.nextNumber++
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"number": %d}`, tr.nextNumber)
			return
		}
		if r.URL.Query().Get("page") != "1" {
			io.WriteString(w, `[]`)
			return
		}
		io.WriteString(w, tr.open)
	})

	mux.HandleFunc(base+"/actions/workflows/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, base+"/actions/workflows/"), "/runs")
		if tr.unknown[name] {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		if body, ok := tr.runs[name]; ok {
			io.WriteString(w, body)
			return
		}
		io.WriteString(w, `{"workflow_runs": []}`)
	})

	mux.HandleFunc(base, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"default_branch": "main"}`)
	})
	return mux
}

func clientFor(t *testing.T, tr *tracker) *Client {
	t.Helper()
	if tr.open == "" {
		tr.open = `[]`
	}
	if tr.nextNumber == 0 {
		// The first issue this tracker hands out is #100, which is what the
		// report is then read for.
		tr.nextNumber = 99
	}
	server := httptest.NewServer(tr.handler(t))
	t.Cleanup(server.Close)
	return &Client{HTTP: server.Client(), API: server.URL, Repository: tr.repository}
}

// aDeliberatelyFailedRun is the planted input: one scheduled run of the
// fixture's nightly workflow, on the default branch, that ended in failure.
const aDeliberatelyFailedRun = `{"workflow_runs": [
  {"run_number": 31, "event": "schedule", "status": "completed", "conclusion": "failure",
   "head_branch": "main", "html_url": "https://example.com/run/31",
   "run_started_at": "2026-08-10T05:17:00Z", "path": ".github/workflows/nightly.yml"}
]}`

func TestRaisesExactlyOneIssueAgainstADeliberatelyFailedRun(t *testing.T) {
	tr := &tracker{
		repository: "an-account/a-repository",
		runs:       map[string]string{"nightly.yml": aDeliberatelyFailedRun},
	}
	client := clientFor(t, tr)

	var out bytes.Buffer
	if err := Sweep(context.Background(), &out, os.DirFS(fixtureTree), client, client); err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	if len(tr.raised) != 1 {
		t.Fatalf("the sweep raised %d issue(s) against one failed run, want 1", len(tr.raised))
	}
	body, _ := tr.raised[0]["body"].(string)
	if keys := KeysIn(body); len(keys) != 1 || keys[0] != ".github/workflows/nightly.yml failure" {
		t.Errorf("the raised issue carries the keys %v", keys)
	}
	if labels, ok := tr.raised[0]["labels"].([]any); !ok || len(labels) != 1 || labels[0] != Label {
		t.Errorf("the raised issue carries the labels %v, want [%s]", tr.raised[0]["labels"], Label)
	}
	if !strings.Contains(out.String(), "raised as #100") {
		t.Errorf("the report does not say what was raised:\n%s", out.String())
	}
}

// TestTheSameFailedRunRaisesNothingTheSecondTime is the other half of the
// guard. A run that raises an issue every night is the noise this exists
// against, and the only thing standing between the two is the marker.
func TestTheSameFailedRunRaisesNothingTheSecondTime(t *testing.T) {
	first := &tracker{
		repository: "an-account/a-repository",
		runs:       map[string]string{"nightly.yml": aDeliberatelyFailedRun},
	}
	client := clientFor(t, first)
	if err := Sweep(context.Background(), io.Discard, os.DirFS(fixtureTree), client, client); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if len(first.raised) != 1 {
		t.Fatalf("the first sweep raised %d issue(s)", len(first.raised))
	}

	// The tracker now holds what the first sweep wrote, which is the state the
	// second sweep arrives in.
	held, err := json.Marshal([]map[string]any{{"number": 100, "body": first.raised[0]["body"]}})
	if err != nil {
		t.Fatal(err)
	}
	second := &tracker{
		repository: "an-account/a-repository",
		runs:       map[string]string{"nightly.yml": aDeliberatelyFailedRun},
		open:       string(held),
	}
	client = clientFor(t, second)

	var out bytes.Buffer
	if err := Sweep(context.Background(), &out, os.DirFS(fixtureTree), client, client); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if len(second.raised) != 0 {
		t.Errorf("the second sweep raised %d issue(s) for a failure already tracked", len(second.raised))
	}
	if !strings.Contains(out.String(), "already tracked") {
		t.Errorf("the report does not say the failure is held:\n%s", out.String())
	}
}

// TestAReportingSweepWritesNothing is why the verb takes a word. Handing the
// run a nil Raiser leaves nothing there to call, so the reporting half cannot
// write by a mistake in a branch somebody edits later.
func TestAReportingSweepWritesNothing(t *testing.T) {
	tr := &tracker{
		repository: "an-account/a-repository",
		runs:       map[string]string{"nightly.yml": aDeliberatelyFailedRun},
	}
	client := clientFor(t, tr)

	var out bytes.Buffer
	if err := Sweep(context.Background(), &out, os.DirFS(fixtureTree), client, nil); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(tr.raised) != 0 {
		t.Fatalf("a reporting run wrote %d issue(s)", len(tr.raised))
	}
	if !strings.Contains(out.String(), "would be raised") || !strings.Contains(out.String(), "wrote nothing") {
		t.Errorf("the report does not say that it wrote nothing:\n%s", out.String())
	}
}

func TestASweepOverACleanHistorySaysWhatItExamined(t *testing.T) {
	tr := &tracker{repository: "an-account/a-repository"}
	client := clientFor(t, tr)

	var out bytes.Buffer
	if err := Sweep(context.Background(), &out, os.DirFS(fixtureTree), client, client); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(tr.raised) != 0 {
		t.Errorf("a clean history raised %d issue(s)", len(tr.raised))
	}
	report := out.String()
	for _, want := range []string{"watched 1 scheduled workflow(s)", "nightly.yml", "outside this watch",
		"no watched workflow ended in anything other than success"} {
		if !strings.Contains(report, want) {
			t.Errorf("the report does not carry %q:\n%s", want, report)
		}
	}
}

func TestAReadThatDidNotHappenIsAnErrorRatherThanACleanSweep(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusForbidden)
	}))
	t.Cleanup(server.Close)
	client := &Client{HTTP: server.Client(), API: server.URL, Repository: "an-account/a-repository"}

	var out bytes.Buffer
	if err := Sweep(context.Background(), &out, os.DirFS(fixtureTree), client, client); err == nil {
		t.Fatal("a sweep that could read nothing reported a clean history")
	}
}

// TestAWorkflowTheServerHasNeverRunIsNamedRatherThanStoppingTheSweep is the
// state this landed in. A scheduled workflow that has just been added is not
// registered on the server until its first run, and the runs endpoint answers
// with a not-found. Stopping there would mean the sweep reports nothing at all
// on the day a scheduled workflow is added, which is the silence it exists
// against, one rung up.
func TestAWorkflowTheServerHasNeverRunIsNamedRatherThanStoppingTheSweep(t *testing.T) {
	tr := &tracker{
		repository: "an-account/a-repository",
		unknown:    map[string]bool{"nightly.yml": true},
	}
	client := clientFor(t, tr)

	var out bytes.Buffer
	if err := Sweep(context.Background(), &out, os.DirFS(fixtureTree), client, client); err != nil {
		t.Fatalf("a workflow with no run history stopped the sweep: %v", err)
	}
	report := out.String()
	if !strings.Contains(report, "nightly.yml has no scheduled run on the server yet") {
		t.Errorf("the report does not name the workflow with no history:\n%s", report)
	}
	if !strings.Contains(report, "no watched workflow ended in anything other than success") {
		t.Errorf("the sweep did not go on to report the rest:\n%s", report)
	}
}

func TestAPullRequestQuotingAKeyIsNotAnOpenIssue(t *testing.T) {
	tr := &tracker{
		repository: "an-account/a-repository",
		runs:       map[string]string{"nightly.yml": aDeliberatelyFailedRun},
		open: `[{"number": 9, "body": "Sweep-key: .github/workflows/nightly.yml failure",
                  "pull_request": {"url": "https://example.com/pull/9"}}]`,
	}
	client := clientFor(t, tr)

	if err := Sweep(context.Background(), io.Discard, os.DirFS(fixtureTree), client, client); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(tr.raised) != 1 {
		t.Errorf("a pull request quoting the key silenced the sweep: %d issue(s) raised", len(tr.raised))
	}
}
