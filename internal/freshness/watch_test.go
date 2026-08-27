// What asks this check when nobody does, and what reports it when it refuses.
//
// The package's own suite decides whether a body is current. This file decides
// the route around that reading: that something asks it unattended, and that a
// refusal reaches a person rather than sitting in a tab. Both halves are read
// off the tree they are about, so a workflow edited to run something else, or a
// schedule dropped from it, reddens here.
package freshness_test

import (
	"os"
	"sort"
	"strings"
	"testing"

	"flowfin.dev/hub/internal/freshness"
	"flowfin.dev/hub/internal/harness"
	"flowfin.dev/hub/internal/sweep"
)

// root is where the workflow files sit from inside this package.
const root = "../../"

func TestTheWatchRunsOnAScheduleAndCanAlsoBeAskedFor(t *testing.T) {
	// The schedule is the whole point: the state this refuses arrives while
	// nobody is looking. The request is what a person uses to see the watch go
	// green again after a merge, without waiting a day to find out.
	f, err := os.Open(root + freshness.WorkflowPath)
	if err != nil {
		t.Fatalf("opening the freshness workflow: %v", err)
	}
	defer f.Close()

	declared, err := harness.Triggers(f)
	if err != nil {
		t.Fatalf("reading the triggers of %s: %v", freshness.WorkflowPath, err)
	}
	sort.Strings(declared)

	want := []string{"schedule", "workflow_dispatch"}
	if strings.Join(declared, ",") != strings.Join(want, ",") {
		// Anything beyond the two is refused as well as anything missing. A
		// push or pull_request trigger here would read the published address on
		// every branch somebody is still writing, and redden a merge on what
		// the world was doing at the time.
		t.Fatalf("%s runs on %v, and the watch runs on %v",
			freshness.WorkflowPath, declared, want)
	}
}

func TestTheWatchRunsTheCheckAndNotSomethingBesideIt(t *testing.T) {
	// A file with the schedule exactly right, running a verb that judges no
	// catalogue, passes every other reading here and reports nothing forever.
	body, err := os.ReadFile(root + freshness.WorkflowPath)
	if err != nil {
		t.Fatalf("reading the freshness workflow: %v", err)
	}
	if want := "run: " + freshness.Verb; !strings.Contains(string(body), want) {
		t.Errorf("%s carries no step running %q", freshness.WorkflowPath, want)
	}
}

func TestTheSweepWatchesTheFreshnessWorkflow(t *testing.T) {
	// Derived from this tree rather than asserted about it. Dropping the
	// schedule from the workflow takes it out of the sweep's set, which is a
	// watch reporting nothing with nothing saying so.
	watched, err := sweep.Scheduled(os.DirFS(root))
	if err != nil {
		t.Fatalf("Scheduled over this tree: %v", err)
	}
	for _, w := range watched {
		if w == freshness.WorkflowPath {
			return
		}
	}
	t.Errorf("%s is not in the watched set %v, so a stale catalogue would be refused and reported to nobody",
		freshness.WorkflowPath, watched)
}

func TestAStaleCatalogueIsItsOwnReportableFailure(t *testing.T) {
	// The refusal above arrives at the sweep as a scheduled run that failed.
	// What is asked here is that it becomes a failure of its own rather than
	// being folded into the publication's: the two fail for different reasons
	// and are fixed by different people, and one issue covering both is the
	// issue everybody stops reading.
	watched := []string{freshness.WorkflowPath, ".github/workflows/publish.yml"}
	runs := []sweep.Run{
		{
			Workflow: freshness.WorkflowPath, Number: 4, Event: "schedule",
			Status: "completed", Conclusion: "failure", Branch: "main",
			URL: "https://example.com/run/4", StartedAt: "2026-08-26T05:53:00Z",
		},
		{
			Workflow: watched[1], Number: 5, Event: "schedule",
			Status: "completed", Conclusion: "failure", Branch: "main",
			URL: "https://example.com/run/5", StartedAt: "2026-08-26T05:23:00Z",
		},
	}

	failures := sweep.Select(watched, runs, "main")
	if len(failures) != 2 {
		t.Fatalf("two workflows failing became %d thing(s) to fix: %+v", len(failures), failures)
	}

	var mine sweep.Failure
	for _, f := range failures {
		if f.Workflow == freshness.WorkflowPath {
			mine = f
		}
	}
	if mine.Workflow == "" {
		t.Fatalf("the freshness workflow's failure is not among %+v", failures)
	}
	for _, other := range failures {
		if other.Workflow != mine.Workflow && other.Key() == mine.Key() {
			t.Errorf("the freshness failure shares the key %q with %s, so one issue would hold both",
				mine.Key(), other.Workflow)
		}
	}

	// An open issue holding one of them must not silence the other, which is
	// the same distinctness read from the other end.
	fresh, held := sweep.Unreported(failures, []sweep.Issue{{Number: 1, Body: failures[0].Body()}})
	if len(fresh) != 1 || len(held) != 1 {
		t.Fatalf("with one of the two already tracked, fresh=%d held=%d, want 1 and 1", len(fresh), len(held))
	}

	if !strings.Contains(mine.Body(), freshness.WorkflowPath) {
		t.Errorf("the raised body does not name the workflow that failed:\n%s", mine.Body())
	}
	if !strings.Contains(mine.Title(), "freshness.yml") {
		t.Errorf("the raised title does not say which schedule went red: %q", mine.Title())
	}
}
