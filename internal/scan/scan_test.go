package scan

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func dir(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", name)
}

func judge(t *testing.T, name string) (reports, tools []string, found []Finding, err error) {
	t.Helper()
	return Judge(os.DirFS(dir(t, name)))
}

func refusal(t *testing.T, err error) *Refusal {
	t.Helper()
	if err == nil {
		t.Fatal("wanted a refusal and got none")
	}
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("wanted a *Refusal and got %T: %v", err, err)
	}
	return r
}

func TestAReportWithNoResultPasses(t *testing.T) {
	reports, tools, found, err := judge(t, "clean")
	if err != nil {
		t.Fatalf("judging a clean report: %v", err)
	}
	if len(found) != 0 {
		t.Errorf("found %d thing(s) in a report with no result: %v", len(found), found)
	}

	var out bytes.Buffer
	if err := Report(&out, "sarif-results", reports, tools, found); err != nil {
		t.Fatalf("reporting a clean run: %v", err)
	}
	// The run says what it read, so a verdict over one report cannot be read as
	// a verdict over two.
	for _, want := range []string{"go.sarif", "CodeQL", "1 report(s)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the report does not carry %q:\n%s", want, out.String())
		}
	}
}

// The whole point of the package. The analysis step exits zero having found
// this, so without the reading below the job is green and the finding is a row
// in a tab nobody opens.
func TestOneFindingRefusesTheRun(t *testing.T) {
	reports, tools, found, err := judge(t, "one-finding")
	if err != nil {
		t.Fatalf("judging: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("found %d thing(s), want 1", len(found))
	}

	var out bytes.Buffer
	err = Report(&out, "sarif-results", reports, tools, found)
	r := refusal(t, err)
	if r.Reason != Findings {
		t.Errorf("reason is %s, want %s", r.Reason, Findings)
	}

	// The line a person acts on: the rule, where it is, and what it says.
	for _, want := range []string{"go/path-injection", "internal/example/read.go:42", "error", "user-provided value"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the report does not carry %q:\n%s", want, out.String())
		}
	}
}

// An analysis that wrote no report and an analysis that found nothing produce
// the same exit status from the step above this one, and they are opposite
// statements.
func TestADirectoryWithNoReportIsRefused(t *testing.T) {
	_, _, _, err := judge(t, "empty")
	r := refusal(t, err)
	if r.Reason != NoReport {
		t.Errorf("reason is %s, want %s", r.Reason, NoReport)
	}
	if !strings.Contains(r.Detail, Suffix) {
		t.Errorf("the refusal does not say what it looked for: %s", r.Detail)
	}
}

func TestADirectoryThatIsNotThereIsRefused(t *testing.T) {
	_, _, _, err := Judge(os.DirFS(filepath.Join("testdata", "no-such-directory")))
	r := refusal(t, err)
	if r.Reason != NoReport {
		t.Errorf("reason is %s, want %s", r.Reason, NoReport)
	}
}

// A step that died halfway leaves a file behind, and it is not the empty report
// it would be mistaken for.
func TestAReportThatIsNotJSONIsRefused(t *testing.T) {
	_, _, _, err := judge(t, "not-json")
	r := refusal(t, err)
	if r.Reason != Unreadable {
		t.Errorf("reason is %s, want %s", r.Reason, Unreadable)
	}
}

// An analysis configured for a language the tree does not hold writes this, and
// it carries exactly as many findings as a clean one.
func TestAReportCarryingNoRunIsRefused(t *testing.T) {
	_, _, _, err := judge(t, "no-run")
	r := refusal(t, err)
	if r.Reason != NothingAnalysed {
		t.Errorf("reason is %s, want %s", r.Reason, NothingAnalysed)
	}
}

func TestARunNamingNoToolIsRefused(t *testing.T) {
	_, _, _, err := judge(t, "unsigned")
	r := refusal(t, err)
	if r.Reason != NothingAnalysed {
		t.Errorf("reason is %s, want %s", r.Reason, NothingAnalysed)
	}
}

// A result with no location is a finding about the whole tree rather than a
// finding to drop, and a reader has to be able to tell that from a location this
// reader lost.
func TestAFindingWithNoLocationIsStillAFinding(t *testing.T) {
	found, _, err := Read("go.sarif", []byte(`{"runs":[{"tool":{"driver":{"name":"CodeQL"}},
		"results":[{"ruleId":"go/whole-tree","message":{"text":"something"}}]}]}`))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("found %d, want 1", len(found))
	}
	if !strings.Contains(found[0].String(), "no location") {
		t.Errorf("the line does not say the finding carries no location: %s", found[0])
	}
	if !strings.Contains(found[0].String(), "unstated") {
		t.Errorf("a result with no level prints as if it had none: %s", found[0])
	}
}

// Two runs in one report is what an analysis over two languages writes, and a
// reader that stopped at the first would pass every finding in the second.
func TestEveryRunInAReportIsRead(t *testing.T) {
	found, tools, err := Read("go.sarif", []byte(`{"runs":[
		{"tool":{"driver":{"name":"CodeQL"}},"results":[]},
		{"tool":{"driver":{"name":"CodeQL/second"}},"results":[
			{"ruleId":"go/second","message":{"text":"in the second run"}}]}]}`))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if len(found) != 1 {
		t.Errorf("found %d thing(s), want the one in the second run", len(found))
	}
	if len(tools) != 2 {
		t.Errorf("named %d tool(s), want 2", len(tools))
	}
}
