package harness

import (
	"os"
	"strings"
	"testing"

	"flowfin.dev/hub/internal/gate"
	"flowfin.dev/hub/internal/reach"
)

// root is this package's path to the repository root.
const root = "../../"

func TestRequirementNamesAreUniqueAndSayWhatTheyCost(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Requirements() {
		if r.Name == "" {
			t.Fatal("a requirement has no name, so nothing states what it needs")
		}
		if r.Reaches == "" || r.Costs == "" {
			t.Fatalf("requirement %s does not say what it reaches or what asking costs; "+
				"the disclosure beside the gate is made of those two sentences", r.Name)
		}
		if seen[r.Name] {
			t.Fatalf("two requirements are named %s", r.Name)
		}
		seen[r.Name] = true
	}
}

func TestEveryRequirementTagIsSparedByTheGateCheck(t *testing.T) {
	// gate-tests-reach-nothing spares a test file whose constraint carries the
	// prefix, because the harness holds exactly the tests it refuses. A
	// requirement whose tag misses the prefix would have its own tests refused
	// by the gate, which is the harness unable to contain anything.
	for _, r := range Requirements() {
		if !strings.HasPrefix(r.Tag(), reach.HarnessTagPrefix) {
			t.Errorf("requirement %s has tag %q, which does not begin with %q, so the gate would refuse its own harness tests",
				r.Name, r.Tag(), reach.HarnessTagPrefix)
		}
		if strings.Contains(r.Tag(), "-") {
			t.Errorf("tag %q carries a hyphen, which no Go build constraint can", r.Tag())
		}
	}
}

func TestTheGateDependsOnNoRequirement(t *testing.T) {
	// The half decisions/headless-and-unelevated.md calls the trade: a red run
	// in the harness may not block a merge. A leg that ran a harness tag, or a
	// leg named after a requirement, would be exactly that.
	names := map[string]bool{}
	tags := map[string]bool{}
	for _, r := range Requirements() {
		names[r.Name] = true
		tags[r.Tag()] = true
	}
	for _, l := range gate.Legs() {
		if names[l.Name] {
			t.Errorf("gate leg %s is a harness requirement, so a merge would wait on it", l.Name)
		}
		for _, arg := range l.Argv {
			if tags[arg] || strings.HasPrefix(arg, reach.HarnessTagPrefix) {
				t.Errorf("gate leg %s runs %q, which is a harness tag", l.Name, arg)
			}
		}
	}
}

func TestEveryRequirementHasAJobAndEveryJobARequirement(t *testing.T) {
	// The same shape as the gate's job-per-leg guard, and for the same reason:
	// a requirement with no job is one nobody can ask for, and a job naming no
	// requirement runs a name this tree does not declare.
	f, err := os.Open(root + WorkflowPath)
	if err != nil {
		t.Fatalf("opening the harness workflow: %v", err)
	}
	defer f.Close()

	jobs, err := gate.JobsIn(f)
	if err != nil {
		t.Fatalf("reading %s: %v", WorkflowPath, err)
	}

	declared := map[string]bool{}
	for _, j := range jobs {
		declared[j.Name] = true
	}
	wanted := map[string]bool{}
	for _, r := range Requirements() {
		wanted[r.Name] = true
		if !declared[r.Name] {
			t.Errorf("requirement %s has no job in %s reporting under its own name", r.Name, WorkflowPath)
		}
	}
	for _, j := range jobs {
		if !wanted[j.Name] {
			t.Errorf("job %s in %s reports as %q, which is no requirement of the harness", j.ID, WorkflowPath, j.Name)
		}
	}
}

func TestEveryHarnessJobRunsTheEntryPointForItsOwnRequirement(t *testing.T) {
	body, err := os.ReadFile(root + WorkflowPath)
	if err != nil {
		t.Fatalf("reading the harness workflow: %v", err)
	}
	text := string(body)
	for _, r := range Requirements() {
		want := "run: go run . harness " + r.Name
		if !strings.Contains(text, want) {
			t.Errorf("%s does not carry a step running %q", WorkflowPath, want)
		}
	}
	steps := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "run: go ") {
			steps++
			if !strings.Contains(line, "run: go run . harness ") {
				t.Errorf("%s runs the toolchain outside the entry point: %s",
					WorkflowPath, strings.TrimSpace(line))
			}
		}
	}
	if steps != len(Requirements()) {
		t.Errorf("%s has %d toolchain steps and %d requirements", WorkflowPath, steps, len(Requirements()))
	}
}

func TestTheHarnessIsTriggeredDeliberatelyAndNoOtherWay(t *testing.T) {
	// The property the issue is about. A push or pull_request trigger here puts
	// the harness in front of every merge, which is the thing the decision
	// refuses; a schedule produces a red nobody asked for and nobody holds.
	f, err := os.Open(root + WorkflowPath)
	if err != nil {
		t.Fatalf("opening the harness workflow: %v", err)
	}
	defer f.Close()

	declared, err := Triggers(f)
	if err != nil {
		t.Fatalf("reading the triggers of %s: %v", WorkflowPath, err)
	}
	if len(declared) == 0 {
		t.Fatalf("%s declares no trigger at all, so nobody can ask for it", WorkflowPath)
	}
	if automatic := AutomaticTriggers(declared); len(automatic) > 0 {
		t.Fatalf("%s runs on %s; the harness is asked for and not scheduled",
			WorkflowPath, strings.Join(automatic, ", "))
	}
}

func TestTheGateWorkflowIsNotAskedForDeliberately(t *testing.T) {
	// The other direction, so the reader above cannot pass by reading nothing:
	// the gate's workflow declares the automatic triggers this one may not.
	f, err := os.Open(root + gate.WorkflowPath)
	if err != nil {
		t.Fatalf("opening the gate workflow: %v", err)
	}
	defer f.Close()

	declared, err := Triggers(f)
	if err != nil {
		t.Fatalf("reading the triggers of %s: %v", gate.WorkflowPath, err)
	}
	if len(AutomaticTriggers(declared)) == 0 {
		t.Fatalf("%s declares only %s, so the trigger reader is not reading anything",
			gate.WorkflowPath, strings.Join(declared, ", "))
	}
}

func TestTheTreeItselfPasses(t *testing.T) {
	// Every harness tag a test file in this tree carries is one a requirement
	// declares. A tag nobody declared is spared by gate-tests-reach-nothing,
	// which reads only the prefix, and run by no job here, which names only the
	// three. The file runs nowhere and nothing says so.
	tagged, err := TaggedFiles(root)
	if err != nil {
		t.Fatalf("reading the harness tags: %v", err)
	}
	declared := map[string]bool{}
	for _, r := range Requirements() {
		declared[r.Tag()] = true
	}
	for tag, files := range tagged {
		if !declared[tag] {
			t.Errorf("%s carries the tag %q, which no requirement declares, so nothing runs it and nothing says so",
				strings.Join(files, ", "), tag)
		}
	}
}

func TestTaggedFilesFindsTheTagAndNotTheProse(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(dir+"/"+name, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("carrier_test.go", "//go:build needs_network\n\npackage p\n")
	write("both_test.go", "//go:build needs_browser && needs_jellyfin\n\npackage p\n")
	write("prose_test.go", "package p\n\n// needs_network is named here and nothing more.\nfunc f() {}\n")
	write("late_test.go", "package p\n\n//go:build needs_network\n")
	write("notatest.go", "//go:build needs_network\n\npackage p\n")

	tagged, err := TaggedFiles(dir)
	if err != nil {
		t.Fatalf("TaggedFiles: %v", err)
	}
	if got := strings.Join(tagged["needs_network"], ","); got != "carrier_test.go" {
		t.Errorf("needs_network carried by %q; a mention in prose, a constraint below the package clause and a non-test file are not carriers", got)
	}
	for _, tag := range []string{"needs_browser", "needs_jellyfin"} {
		if got := strings.Join(tagged[tag], ","); got != "both_test.go" {
			t.Errorf("%s carried by %q, want both_test.go; a file needing two things carries both", tag, got)
		}
	}
}

func TestAskRefusesARequirementNothingCarries(t *testing.T) {
	// A green job that compiled nothing is the absent result read as a clean
	// one. `go test` over an uncarried tag exits zero, so the refusal has to be
	// here and not in the exit status.
	r, ok := Lookup(Requirements(), "needs-browser")
	if !ok {
		t.Fatal("no needs-browser requirement")
	}
	var out strings.Builder
	err := Ask(&out, ".", r, map[string][]string{})
	if err == nil {
		t.Fatal("asking for a requirement nothing carries exited zero")
	}
	if !strings.Contains(err.Error(), r.Tag()) {
		t.Fatalf("the refusal does not name the tag: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("something ran before the refusal:\n%s", out.String())
	}
}

func TestDisclosureSaysNothingRanAndWhatAskingCosts(t *testing.T) {
	var out strings.Builder
	Disclosure(&out, Requirements(), map[string][]string{"needs_network": {"internal/sources/world_test.go"}})
	text := out.String()

	if !strings.Contains(text, "did not run") {
		t.Fatalf("the disclosure does not say the harness did not run:\n%s", text)
	}
	for _, r := range Requirements() {
		if !strings.Contains(text, r.Name) {
			t.Errorf("the disclosure does not name %s:\n%s", r.Name, text)
		}
		if !strings.Contains(text, r.Costs) {
			t.Errorf("the disclosure does not say what asking for %s costs:\n%s", r.Name, text)
		}
	}
	if !strings.Contains(text, "internal/sources/world_test.go") {
		t.Errorf("the disclosure does not say which file a carried requirement would run:\n%s", text)
	}
	if !strings.Contains(text, "no check carries it yet") {
		t.Errorf("the disclosure does not distinguish a requirement nothing carries:\n%s", text)
	}
	// A negative disclosure that could be read as a positive one is the failure
	// this whole block exists against, so no word here may report an outcome.
	// Whole words, because "ok" inside "looking" is not a verdict.
	forbidden := map[string]bool{"ok": true, "pass": true, "passed": true, "green": true, "clean": true, "success": true}
	for _, word := range strings.FieldsFunc(text, func(r rune) bool { return !('a' <= r && r <= 'z') }) {
		if forbidden[word] {
			t.Errorf("the disclosure contains the word %q, which reads as a result:\n%s", word, text)
		}
	}
}

func TestTriggersReadsTheThreeSpellings(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want string
	}{
		{"block", "name: x\non:\n  workflow_dispatch:\n    inputs:\n      a:\n        type: string\n\njobs:\n  j:\n    name: j\n", "workflow_dispatch"},
		{"block with two", "on:\n  push:\n    branches: [\"**\"]\n  pull_request:\n", "push,pull_request"},
		{"inline", "on: workflow_dispatch\n", "workflow_dispatch"},
		{"inline list", "on: [push, workflow_dispatch]\n", "push,workflow_dispatch"},
		{"list items", "on:\n  - push\n  - schedule\n", "push,schedule"},
	} {
		got, err := Triggers(strings.NewReader(c.src))
		if err != nil {
			t.Errorf("%s: Triggers: %v", c.name, err)
			continue
		}
		if strings.Join(got, ",") != c.want {
			t.Errorf("%s: read %v, want %s", c.name, got, c.want)
		}
	}
}

func TestTriggersRefusesWhatItCannotRead(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		{"no on block", "name: x\njobs:\n  j:\n    name: j\n"},
		{"two on blocks", "on:\n  push:\non: workflow_dispatch\n"},
		{"a shape this reader does not know", "on:\n  push branches\n"},
	} {
		if _, err := Triggers(strings.NewReader(c.src)); err == nil {
			t.Errorf("%s: accepted, and a trigger this reader missed is the harness running on every merge", c.name)
		}
	}
}

func TestArgvRunsTheRequirementsOwnTagAndCountsFromZero(t *testing.T) {
	for _, r := range Requirements() {
		argv := strings.Join(Argv(r), " ")
		if !strings.Contains(argv, "-tags "+r.Tag()) {
			t.Errorf("%s runs %q, which does not carry its own tag", r.Name, argv)
		}
		if !strings.Contains(argv, "-count=1") {
			t.Errorf("%s runs %q; a cached pass is not a run against the world", r.Name, argv)
		}
	}
}
