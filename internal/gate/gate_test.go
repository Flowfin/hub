package gate

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// pass and refuse are Executors that do not run anything, so the runner's
// ordering and its report are tested without a toolchain in the loop.
func pass(Leg) error { return nil }

func refuse(name string) Executor {
	return func(l Leg) error {
		if l.Name == name {
			return errors.New("planted refusal")
		}
		return nil
	}
}

func TestVerdictRefusesGofmtListingAFileDespiteExitZero(t *testing.T) {
	// gofmt -l exits zero whether or not it printed a file name, so this is the
	// one leg whose exit status carries no verdict. Removing the
	// OutputIsTheVerdict branch in Verdict makes this test the one that reds.
	format, ok := Lookup(Legs(), "format")
	if !ok {
		t.Fatal("no format leg")
	}
	if !format.OutputIsTheVerdict {
		t.Fatal("the format leg no longer declares that its output is its verdict")
	}

	err := Verdict(format, "manifest/manifest.go\n", nil)
	if err == nil {
		t.Fatal("a file name on standard output with exit zero was read as a pass")
	}
	if !strings.Contains(err.Error(), "manifest/manifest.go") {
		t.Fatalf("the refusal does not name the file it is about: %v", err)
	}
}

func TestVerdictPassesGofmtPrintingNothing(t *testing.T) {
	format, _ := Lookup(Legs(), "format")
	for _, out := range []string{"", "\n", "  \n\n"} {
		if err := Verdict(format, out, nil); err != nil {
			t.Fatalf("empty output %q was read as a refusal: %v", out, err)
		}
	}
}

func TestVerdictRefusesANonZeroExit(t *testing.T) {
	build, _ := Lookup(Legs(), "build")
	err := Verdict(build, "", errors.New("exit status 1"))
	if err == nil {
		t.Fatal("a non-zero exit was read as a pass")
	}
	if !strings.Contains(err.Error(), "go build ./...") {
		t.Fatalf("the refusal does not name the command that produced it: %v", err)
	}
}

func TestVerdictIgnoresOutputFromALegThatIsJudgedByItsStatus(t *testing.T) {
	// go test prints on the way past. Reading that as a refusal would red every
	// run, so only a leg that declares it is judged this way is judged this way.
	test, _ := Lookup(Legs(), "test")
	if err := Verdict(test, "ok  \tflowfin.dev/hub/manifest\t0.2s\n", nil); err != nil {
		t.Fatalf("ordinary output was read as a refusal: %v", err)
	}
}

func TestRunStopsAtTheFirstFailure(t *testing.T) {
	var out strings.Builder
	results, err := Run(&out, Legs(), nil, refuse("build"))
	if err == nil {
		t.Fatal("a failing leg did not fail the run")
	}

	got := map[string]Outcome{}
	for _, r := range results {
		got[r.Leg.Name] = r.Outcome
	}
	want := map[string]Outcome{"build": Failed, "test": NotReached, "format": NotReached}
	for name, w := range want {
		if got[name] != w {
			t.Fatalf("leg %s: outcome %v, want %v", name, got[name], w)
		}
	}
}

func TestRunSaysHowManyLegsItExamined(t *testing.T) {
	// The property the issue is about: a run that covered less than the whole
	// set may not read as one that covered it and found nothing.
	var stopped strings.Builder
	if _, err := Run(&stopped, Legs(), nil, refuse("test")); err == nil {
		t.Fatal("a failing leg did not fail the run")
	}
	if !strings.Contains(stopped.String(), fmt.Sprintf("gate examined 2 of %d legs.", len(Legs()))) {
		t.Fatalf("a run stopped after two legs does not say so:\n%s", stopped.String())
	}
	if !strings.Contains(stopped.String(), "because test failed first") {
		t.Fatalf("a leg that was not reached does not say why:\n%s", stopped.String())
	}

	var whole strings.Builder
	if _, err := Run(&whole, Legs(), nil, pass); err != nil {
		t.Fatalf("a clean run failed: %v", err)
	}
	if !strings.Contains(whole.String(), fmt.Sprintf("gate examined %d of %d legs.", len(Legs()), len(Legs()))) {
		t.Fatalf("a whole run does not say so:\n%s", whole.String())
	}
}

func TestRunNamesTheLegsItWasNotAskedFor(t *testing.T) {
	// Each job runs one leg, so this is the shape of every run in CI. Its output
	// must not be readable as the whole gate.
	var out strings.Builder
	if _, err := Run(&out, Legs(), []string{"format"}, pass); err != nil {
		t.Fatalf("a clean single-leg run failed: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, fmt.Sprintf("gate examined 1 of %d legs.", len(Legs()))) {
		t.Fatalf("a one-leg run does not say how many it left:\n%s", text)
	}
	for _, name := range Names(Legs()) {
		if name == "format" {
			continue
		}
		if !reportSays(text, name, "not asked for") {
			t.Fatalf("leg %s is missing from the report of a run that skipped it:\n%s", name, text)
		}
	}
}

// reportSays finds the summary line for one leg and checks what it says about
// it, without depending on the column the report pads names to.
func reportSays(report, leg, outcome string) bool {
	for _, line := range strings.Split(report, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == leg {
			return strings.Contains(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), leg)), outcome)
		}
	}
	return false
}

func TestRunOnlyRunsTheLegItWasAskedFor(t *testing.T) {
	var ran []string
	record := func(l Leg) error {
		ran = append(ran, l.Name)
		return nil
	}
	if _, err := Run(&strings.Builder{}, Legs(), []string{"test"}, record); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(ran) != 1 || ran[0] != "test" {
		t.Fatalf("asked for test, ran %v", ran)
	}
}

func TestUnknownLegNameIsRefused(t *testing.T) {
	// A mistyped leg name must not be a run of nothing that exits zero.
	if got := Unknown(Legs(), []string{"formatting", "test"}); len(got) != 1 || got[0] != "formatting" {
		t.Fatalf("Unknown returned %v", got)
	}
	if got := Unknown(Legs(), Names(Legs())); len(got) != 0 {
		t.Fatalf("every leg name should be known, got %v", got)
	}
}

func TestLegNamesAreUniqueAndNonEmpty(t *testing.T) {
	seen := map[string]bool{}
	for _, l := range Legs() {
		if l.Name == "" {
			t.Fatal("a leg has no name")
		}
		if len(l.Argv) == 0 {
			t.Fatalf("leg %s runs nothing", l.Name)
		}
		if l.Refuses == "" {
			t.Fatalf("leg %s does not say what it refuses", l.Name)
		}
		if seen[l.Name] {
			t.Fatalf("two legs are named %s", l.Name)
		}
		seen[l.Name] = true
	}
}

// takenOnMain are the check-run names the Pages deployment already produces on
// this repository. Measured, not assumed:
//
//	gh api repos/Flowfin/hub/commits/main/check-runs --jq '[.check_runs[].name] | sort'
//
// Run 2026-08-08 against main at b94c2fa.
var takenOnMain = []string{"build", "deploy", "report-build-status"}

func TestNoLegReportsUnderANameSomethingElseAlreadyUses(t *testing.T) {
	for _, l := range Legs() {
		for _, taken := range takenOnMain {
			if l.CheckRunName() == taken {
				t.Fatalf("leg %s reports as %q, which the Pages deployment already produces on main; "+
					"a ruleset requiring that name would be requiring the wrong check",
					l.Name, taken)
			}
		}
	}
}

func TestWorkflowDeclaresOneJobPerLeg(t *testing.T) {
	// This is the guard behind the sentence "one named leg per thing it checks".
	// Delete a job from the workflow file, or add one, and this reds.
	f, err := os.Open("../../" + WorkflowPath)
	if err != nil {
		t.Fatalf("opening the gate workflow: %v", err)
	}
	defer f.Close()

	jobs, err := JobsIn(f)
	if err != nil {
		t.Fatalf("reading %s: %v", WorkflowPath, err)
	}

	declared := map[string]bool{}
	for _, j := range jobs {
		declared[j.Name] = true
	}
	wanted := map[string]bool{}
	for _, l := range Legs() {
		wanted[l.CheckRunName()] = true
		if !declared[l.CheckRunName()] {
			t.Errorf("leg %s has no job in %s reporting as %q", l.Name, WorkflowPath, l.CheckRunName())
		}
	}
	for _, j := range jobs {
		if !wanted[j.Name] {
			t.Errorf("job %s in %s reports as %q, which is no leg of the gate", j.ID, WorkflowPath, j.Name)
		}
	}
}

func TestEveryGateJobRunsTheEntryPointAndNothingElse(t *testing.T) {
	// The other half of "one entry point": a job that inlines its own command is
	// a second procedure, and the two drift.
	body, err := os.ReadFile("../../" + WorkflowPath)
	if err != nil {
		t.Fatalf("reading the gate workflow: %v", err)
	}
	text := string(body)
	for _, l := range Legs() {
		want := fmt.Sprintf("run: go run . gate %s", l.Name)
		if !strings.Contains(text, want) {
			t.Errorf("%s does not carry a step running %q", WorkflowPath, want)
		}
	}
	// A step may report on the environment, and one does. What it may not do is
	// decide anything: every step that runs the toolchain is the entry point.
	toolchainSteps := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "run: go ") {
			toolchainSteps++
			if !strings.Contains(line, "run: go run . gate ") {
				t.Errorf("%s runs the toolchain outside the entry point: %s",
					WorkflowPath, strings.TrimSpace(line))
			}
		}
	}
	if toolchainSteps != len(Legs()) {
		t.Errorf("%s has %d toolchain steps and %d legs; a leg run twice or a check inlined in a job is a second procedure",
			WorkflowPath, toolchainSteps, len(Legs()))
	}
}

func TestJobsInReadsIdsAndNames(t *testing.T) {
	const src = `name: Gate

on:
  push:
    branches: ["**"]

permissions:
  contents: read

jobs:
  build:
    name: "Gate: build"
    runs-on: ubuntu-latest
    steps:
      - name: a step name is not a job name
        run: true
  format:
    name: Gate: format
    steps:
      - run: true
`
	jobs, err := JobsIn(strings.NewReader(src))
	if err != nil {
		t.Fatalf("JobsIn: %v", err)
	}
	got := make([]string, 0, len(jobs))
	for _, j := range jobs {
		got = append(got, j.ID+"="+j.Name)
	}
	sort.Strings(got)
	want := []string{"build=Gate: build", "format=Gate: format"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("JobsIn read %v, want %v", got, want)
	}
}

func TestJobsInRefusesAJobWithNoName(t *testing.T) {
	const src = `jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: true
`
	if _, err := JobsIn(strings.NewReader(src)); err == nil {
		t.Fatal("a job declaring no name was accepted; its check run would be called by its key")
	}
}

func TestJobsInReadsNothingOutsideTheJobsBlock(t *testing.T) {
	const src = `on:
  push:
    branches: ["**"]

jobs:
  build:
    name: "Gate: build"
    steps:
      - run: true

permissions:
  contents: read
`
	jobs, err := JobsIn(strings.NewReader(src))
	if err != nil {
		t.Fatalf("JobsIn: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != "build" {
		t.Fatalf("JobsIn read %v; a key outside jobs: is not a job", jobs)
	}
}
