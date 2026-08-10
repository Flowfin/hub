package sweep

import (
	"os"
	"strings"
	"testing"
)

// fixtureTree is shaped like a repository root, so the derivation is exercised
// through the same path it uses in a checkout.
const fixtureTree = "testdata/workflows"

func TestScheduledDerivesTheSetFromTheTriggerBlocks(t *testing.T) {
	got, err := Scheduled(os.DirFS(fixtureTree))
	if err != nil {
		t.Fatalf("Scheduled: %v", err)
	}

	want := []string{
		WorkflowDir + "/inline-trigger-list.yml",
		WorkflowDir + "/nightly.yml",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("watched set is %v, want %v", got, want)
	}
}

// The near-misses, one test each, because each is a different one-character
// reading and a table would let one of them pass on the other's fixture.

func TestTheWordInACommentIsNotATrigger(t *testing.T) {
	body, err := os.ReadFile(fixtureTree + "/" + WorkflowDir + "/says-schedule-in-a-comment.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "schedule") {
		t.Fatal("the fixture no longer carries the word, so it proves nothing")
	}

	scheduled, err := DeclaresSchedule(strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("DeclaresSchedule: %v", err)
	}
	if scheduled {
		t.Error("a file whose only mention of a schedule is a comment was read as scheduled")
	}
}

func TestAKeyBelowATriggerIsNotATrigger(t *testing.T) {
	body, err := os.ReadFile(fixtureTree + "/" + WorkflowDir + "/cron-under-another-trigger.yml")
	if err != nil {
		t.Fatal(err)
	}

	scheduled, err := DeclaresSchedule(strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("DeclaresSchedule: %v", err)
	}
	if scheduled {
		t.Error("an input named schedule was read as a schedule trigger")
	}
}

func TestAFileWithNoTriggerBlockIsRefusedRatherThanCalledUnscheduled(t *testing.T) {
	_, err := DeclaresSchedule(strings.NewReader("name: Nothing\n\njobs:\n  a:\n    name: A\n"))
	if err == nil {
		t.Fatal("a workflow declaring no trigger was accepted, so nothing says when it runs")
	}
	if !strings.Contains(err.Error(), "trigger") {
		t.Errorf("the refusal does not name what is missing: %v", err)
	}
}

// TestTheSweepDerivesItselfIntoItsOwnWatch judges this repository rather than a
// fixture, and it is the one test here that does.
//
// What it proves is a property of the tree and not of the reader: the sweep's
// own workflow is scheduled, so a sweep run that dies is reported by the next
// one that lives. Nothing else here would notice that file losing its schedule.
func TestTheSweepDerivesItselfIntoItsOwnWatch(t *testing.T) {
	watched, err := Scheduled(os.DirFS("../.."))
	if err != nil {
		t.Fatalf("Scheduled over this tree: %v", err)
	}
	for _, w := range watched {
		if w == OwnWorkflow {
			return
		}
	}
	t.Errorf("%s is not in the watched set %v, so a sweep run that failed would be reported by nothing", OwnWorkflow, watched)
}

func run(workflow, conclusion string, number int) Run {
	return Run{
		Workflow:   workflow,
		Number:     number,
		Event:      "schedule",
		Status:     "completed",
		Conclusion: conclusion,
		Branch:     "main",
		URL:        "https://example.com/run",
		StartedAt:  "2026-08-10T05:17:00Z",
	}
}

func TestSelectTakesOnlyAnEndedScheduledRunOnTheDefaultBranchThatDidNotSucceed(t *testing.T) {
	watched := []string{".github/workflows/nightly.yml"}

	stillGoing := run(watched[0], "", 5)
	stillGoing.Status = "in_progress"

	askedFor := run(watched[0], "failure", 6)
	askedFor.Event = "workflow_dispatch"

	elsewhere := run(watched[0], "failure", 7)
	elsewhere.Branch = "a-branch"

	unwatched := run(".github/workflows/gate.yml", "failure", 8)

	for _, c := range []struct {
		name string
		in   Run
	}{
		{"a run still going has no verdict to report", stillGoing},
		{"a run somebody asked for has somebody looking at it", askedFor},
		{"a run off the default branch is not a scheduled run of it", elsewhere},
		{"a workflow outside the derived set is not watched", unwatched},
		{"a run that succeeded is not a failure", run(watched[0], "success", 9)},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := Select(watched, []Run{c.in}, "main"); len(got) != 0 {
				t.Errorf("selected %d failure(s), want none: %+v", len(got), got)
			}
		})
	}

	got := Select(watched, []Run{run(watched[0], "timed_out", 10)}, "main")
	if len(got) != 1 || got[0].Conclusion != "timed_out" {
		t.Fatalf("a timed-out scheduled run was not selected: %+v", got)
	}
}

func TestOneFailurePerWorkflowAndVerdictRatherThanOnePerRun(t *testing.T) {
	watched := []string{".github/workflows/nightly.yml"}
	runs := []Run{
		run(watched[0], "failure", 11),
		run(watched[0], "failure", 12),
		run(watched[0], "failure", 13),
		run(watched[0], "cancelled", 14),
	}

	got := Select(watched, runs, "main")
	if len(got) != 2 {
		t.Fatalf("three failures and one cancellation became %d thing(s) to fix: %+v", len(got), got)
	}
	for _, f := range got {
		if f.Conclusion == "failure" && len(f.Runs) != 3 {
			t.Errorf("the failure carries %d run(s) as evidence, want 3", len(f.Runs))
		}
		if f.Runs[0].Number < f.Runs[len(f.Runs)-1].Number {
			t.Error("the runs are not newest first, so the issue leads with the oldest evidence")
		}
	}
}

func TestAFailureAnOpenIssueAlreadyHoldsIsNotRaisedAgain(t *testing.T) {
	failures := Select(
		[]string{".github/workflows/nightly.yml"},
		[]Run{run(".github/workflows/nightly.yml", "failure", 15)},
		"main",
	)
	if len(failures) != 1 {
		t.Fatalf("the fixture produced %d failure(s)", len(failures))
	}

	open := []Issue{{Number: 42, Body: failures[0].Body()}}
	fresh, held := Unreported(failures, open)
	if len(fresh) != 0 {
		t.Errorf("a failure an open issue already holds was raised again: %+v", fresh)
	}
	if len(held) != 1 {
		t.Errorf("the held failure is not reported as held: %+v", held)
	}

	fresh, held = Unreported(failures, nil)
	if len(fresh) != 1 || len(held) != 0 {
		t.Errorf("with nothing open, fresh=%d held=%d, want 1 and 0", len(fresh), len(held))
	}
}

func TestAKeyQuotedInsideASentenceDoesNotSilenceTheSweep(t *testing.T) {
	failures := Select(
		[]string{".github/workflows/nightly.yml"},
		[]Run{run(".github/workflows/nightly.yml", "failure", 16)},
		"main",
	)

	body := "The marker looks like `" + KeyPrefix + " " + failures[0].Key() + "`, indented here:\n\n    " +
		KeyPrefix + " " + failures[0].Key() + "\n"
	fresh, _ := Unreported(failures, []Issue{{Number: 7, Body: body}})
	if len(fresh) != 1 {
		t.Error("an issue merely describing the marker was read as tracking the failure")
	}
}

func TestTheRaisedBodyCarriesTheKeyAtColumnZero(t *testing.T) {
	f := Failure{Workflow: ".github/workflows/nightly.yml", Conclusion: "failure", Runs: []Run{run(".github/workflows/nightly.yml", "failure", 17)}}
	keys := KeysIn(f.Body())
	if len(keys) != 1 || keys[0] != f.Key() {
		t.Fatalf("the body carries %v, want exactly [%s]", keys, f.Key())
	}
	if !strings.Contains(f.Title(), "nightly.yml") {
		t.Errorf("the title does not name the workflow: %q", f.Title())
	}
}

func TestTheReportTellsAnEmptyWatchFromAQuietOne(t *testing.T) {
	nothingWatched := Report(nil, []string{".github/workflows/gate.yml"}, nil, nil, nil, nil)
	if !strings.Contains(nothingWatched, "watched nothing") {
		t.Errorf("a run that watched nothing does not say so: %q", nothingWatched)
	}

	quiet := Report([]string{".github/workflows/nightly.yml"}, nil, nil, nil, nil, nil)
	if !strings.Contains(quiet, "no watched workflow ended in anything other than success") {
		t.Errorf("a clean run does not say what it examined: %q", quiet)
	}
	if strings.Contains(quiet, "watched nothing") {
		t.Error("a clean run reads as a run that watched nothing")
	}
}
