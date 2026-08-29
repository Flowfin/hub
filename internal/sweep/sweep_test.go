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

	want := []string{WorkflowDir + "/nightly.yml"}
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

func TestATriggerBlockOnOneLineIsReadRatherThanRefused(t *testing.T) {
	// The form is legal and this tree does not use it. It can never declare a
	// schedule, because a schedule carries a cron and so has to be a key; what
	// matters is that the file is read at all, since one this reader refused
	// would be a file outside the watch.
	body, err := os.ReadFile(fixtureTree + "/" + WorkflowDir + "/inline-trigger-list.yml")
	if err != nil {
		t.Fatal(err)
	}

	scheduled, err := DeclaresSchedule(strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("a trigger block on one line was refused: %v", err)
	}
	if scheduled {
		t.Error("a trigger block on one line was read as declaring a schedule")
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

func TestAFailureALaterScheduledRunRecoveredFromIsNotSelected(t *testing.T) {
	watched := []string{".github/workflows/nightly.yml"}
	failed := run(watched[0], "failure", 18)

	// The failure on its own is what the sweep exists to report, so the filter
	// below is not one that reports nothing.
	if got := Select(watched, []Run{failed}, "main"); len(got) != 1 {
		t.Fatalf("a failure with no later run selected %d failure(s), want 1", len(got))
	}

	recovered := run(watched[0], "success", 19)
	if got := Select(watched, []Run{failed, recovered}, "main"); len(got) != 0 {
		t.Errorf("a failure a later scheduled run recovered from is still selected, so closing its issue raises the same one again: %+v", got)
	}

	// Three runs that are not a recovery, each for a different reason. A
	// success before the failure says nothing about it, and a run off the
	// schedule or off the default branch is outside the population this sweep
	// reports on at all.
	for _, c := range []struct {
		name string
		in   Run
	}{
		{"a success older than the failure", run(watched[0], "success", 17)},
		{"a success somebody asked for", askedFor(run(watched[0], "success", 20))},
		{"a success off the default branch", offBranch(run(watched[0], "success", 21))},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := Select(watched, []Run{failed, c.in}, "main"); len(got) != 1 {
				t.Errorf("selected %d failure(s), want 1: %+v", len(got), got)
			}
		})
	}
}

func askedFor(r Run) Run { r.Event = "workflow_dispatch"; return r }

func offBranch(r Run) Run { r.Branch = "a-branch"; return r }

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

// The two tests below are the pair that holds Select's recovery predicate and
// the sentence the raised issue states about it together. One walks the
// operator, the other walks the text, and both walk the same list, so a
// condition that exists on one side and not on the other has nowhere to hide.

// breakers pairs each recovery condition with a run that fails that one and
// satisfies every other. The pairing is checked rather than trusted: a mutation
// that broke two conditions would prove the wrong thing about the second.
var breakers = []struct {
	name   string
	breaks func(Run) Run
}{
	{"a success somebody dispatched", askedFor},
	{"a success off the default branch", offBranch},
	{"a run that has not ended", func(r Run) Run { r.Status = "in_progress"; r.Conclusion = ""; return r }},
}

func TestSelectRefusesAsARecoveryEveryRunTheBodyExcludes(t *testing.T) {
	watched := []string{".github/workflows/nightly.yml"}
	failed := run(watched[0], "failure", 30)

	// A run satisfying every condition does clear the failure, so the cases
	// below are a filter refusing things rather than one that reports nothing.
	clean := run(watched[0], "success", 31)
	if !recovers(clean, "main") {
		t.Fatalf("a scheduled success on the default branch is not read as a recovery: %+v", clean)
	}
	if got := Select(watched, []Run{failed, clean}, "main"); len(got) != 0 {
		t.Fatalf("a scheduled success on the default branch did not clear the failure: %+v", got)
	}

	if len(breakers) != len(recoveryConditions) {
		t.Fatalf("%d condition(s) are stated and %d are proven, so one of them is refused by nothing",
			len(recoveryConditions), len(breakers))
	}

	for i, b := range breakers {
		t.Run(b.name, func(t *testing.T) {
			later := b.breaks(run(watched[0], "success", 31))

			for j, c := range recoveryConditions {
				holds := c.Holds(later, "main")
				if j == i && holds {
					t.Fatalf("the run still satisfies the condition it is meant to break: %q", c.Says)
				}
				if j != i && !holds {
					t.Fatalf("the run breaks a second condition as well: %q", c.Says)
				}
			}

			if recovers(later, "main") {
				t.Errorf("a run breaking %q is read as a recovery", recoveryConditions[i].Says)
			}
			if got := Select(watched, []Run{failed, later}, "main"); len(got) != 1 {
				t.Errorf("selected %d failure(s), want 1: a run breaking %q cleared the key",
					len(got), recoveryConditions[i].Says)
			}
		})
	}
}

func TestTheRaisedBodyStatesEveryConditionSelectApplies(t *testing.T) {
	f := Failure{
		Workflow:   ".github/workflows/nightly.yml",
		Conclusion: "failure",
		Runs:       []Run{run(".github/workflows/nightly.yml", "failure", 32)},
	}
	body := f.Body()

	// Printed so the sentence a raised issue carries can be read without one
	// being raised, which is the whole subject of this test.
	t.Logf("the body a raised issue carries:\n%s", body)

	for _, c := range recoveryConditions {
		if !strings.Contains(body, c.Says) {
			t.Errorf("the body does not state the condition %q, so a reader is told less than Select applies", c.Says)
		}
	}

	// The one the drift cost a reader: the body has to say in its own words
	// that a dispatched green run is not the recovery, because that is the
	// thing somebody does after reading it.
	if !strings.Contains(body, "Dispatching this workflow by hand") {
		t.Errorf("the body does not say that a dispatched run does not clear the key: %q", body)
	}
}
