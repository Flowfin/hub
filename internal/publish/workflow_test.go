package publish_test

import (
	"os"
	"sort"
	"strings"
	"testing"

	"flowfin.dev/hub/internal/harness"
	"flowfin.dev/hub/internal/publish"
)

// root is where the workflow file sits from inside this package.
const root = "../../"

// concurrencyIn is the reader run over a workflow written inline, which is how
// every near miss below is planted.
func concurrencyIn(t *testing.T, text string) publish.Concurrency {
	t.Helper()
	c, err := publish.ConcurrencyIn(strings.NewReader(text))
	if err != nil {
		t.Fatalf("reading %q: %v", text, err)
	}
	return c
}

func TestThePublicationIsScheduledAndCanAlsoBeAskedFor(t *testing.T) {
	// Both halves of the issue's first sentence, and neither on its own. A file
	// with only the schedule cannot be run the moment a plugin releases, and one
	// with only the request is the button somebody has to remember.
	f, err := os.Open(root + publish.WorkflowPath)
	if err != nil {
		t.Fatalf("opening the publication workflow: %v", err)
	}
	defer f.Close()

	declared, err := harness.Triggers(f)
	if err != nil {
		t.Fatalf("reading the triggers of %s: %v", publish.WorkflowPath, err)
	}
	sort.Strings(declared)

	want := []string{"schedule", "workflow_dispatch"}
	if strings.Join(declared, ",") != strings.Join(want, ",") {
		// Anything beyond the two is refused as well as anything missing. A
		// push or pull_request trigger here would run the publication on every
		// branch somebody is still writing, against the live sources.
		t.Fatalf("%s runs on %v, and the publication runs on %v",
			publish.WorkflowPath, declared, want)
	}
}

func TestRunsWritingThePublishedFileQueueAndAreNotCancelled(t *testing.T) {
	// The failure the issue is mostly about: two runs over one destination at
	// once, and a run losing the write it was in the middle of.
	f, err := os.Open(root + publish.WorkflowPath)
	if err != nil {
		t.Fatalf("opening the publication workflow: %v", err)
	}
	defer f.Close()

	declared, err := publish.ConcurrencyIn(f)
	if err != nil {
		t.Fatalf("reading the concurrency of %s: %v", publish.WorkflowPath, err)
	}
	if err := declared.Serialised(publish.Stable); err != nil {
		t.Fatalf("%s: %v", publish.WorkflowPath, err)
	}
}

func TestTheWorkflowRunsThePublicationAndNotSomethingBesideIt(t *testing.T) {
	// A file with the queue exactly right, running a verb that publishes
	// nothing, passes every other check here.
	body, err := os.ReadFile(root + publish.WorkflowPath)
	if err != nil {
		t.Fatalf("reading the publication workflow: %v", err)
	}
	if want := "run: go run . publish"; !strings.Contains(string(body), want) {
		t.Errorf("%s carries no step running %q", publish.WorkflowPath, want)
	}
}

func TestTheGroupNamesWhereTheBytesGo(t *testing.T) {
	// The group is derived so that moving the destination moves the queue with
	// it. Two targets that differ anywhere belong in different queues, and one
	// target always answers with the same name.
	if got, want := publish.Stable.Group(), "publish-docs/manifest.json"; got != want {
		t.Errorf("the stable target queues in %q, and the workflow declares %q", got, want)
	}
	elsewhere := publish.Target{Dir: publish.Stable.Dir, Name: "unstable.json"}
	if elsewhere.Group() == publish.Stable.Group() {
		t.Errorf("two targets under %s share the queue %q", publish.Stable.Dir, elsewhere.Group())
	}
}

func TestSerialisedRefusesTheThreeWaysAQueueFails(t *testing.T) {
	for _, c := range []struct {
		name  string
		block publish.Concurrency
	}{
		{"no block at all", publish.Concurrency{}},
		{
			// The one somebody actually writes, because it is what gate.yml
			// next door declares and copying it is one keystroke. Every part of
			// it is wrong here: the ref splits a scheduled run away from a run
			// started by hand from a branch, and the cancellation takes the
			// write with it.
			"the gate's block copied over",
			publish.Concurrency{Declared: true, Group: "${{ github.workflow }}-${{ github.ref }}", Cancels: true},
		},
		{
			"the right group, cancelled",
			publish.Concurrency{Declared: true, Group: publish.Stable.Group(), Cancels: true},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if err := c.block.Serialised(publish.Stable); err == nil {
				t.Errorf("%s is accepted as serialising the runs over %s", c.name, publish.Stable.Name)
			}
		})
	}
}

func TestConcurrencyInReadsTheFormsAWorkflowMayUse(t *testing.T) {
	for _, c := range []struct {
		name string
		text string
		want publish.Concurrency
	}{
		{
			"the block form",
			"name: A\nconcurrency:\n  group: one\n  cancel-in-progress: false\njobs:\n  a:\n",
			publish.Concurrency{Declared: true, Group: "one"},
		},
		{
			"the short form, which cancels nothing",
			"name: A\nconcurrency: one\njobs:\n  a:\n",
			publish.Concurrency{Declared: true, Group: "one"},
		},
		{
			"a quoted group with a comment after it",
			"concurrency:\n  group: \"one\" # named for the file\n  cancel-in-progress: true\n",
			publish.Concurrency{Declared: true, Group: "one", Cancels: true},
		},
		{
			// The block below belongs to a job rather than to the workflow, so
			// it serialises that job and leaves the workflow unqueued. A reader
			// that took it would report a file with no queue as having one.
			"a block indented under a job",
			"name: A\njobs:\n  a:\n    concurrency:\n      group: one\n",
			publish.Concurrency{},
		},
		{"no block anywhere", "name: A\non:\n  push:\n", publish.Concurrency{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := concurrencyIn(t, c.text); got != c.want {
				t.Errorf("read %+v, and the file declares %+v", got, c.want)
			}
		})
	}
}

func TestConcurrencyInRefusesWhatItCannotRead(t *testing.T) {
	for _, c := range []struct {
		name string
		text string
	}{
		{"two blocks", "concurrency:\n  group: one\nconcurrency:\n  group: two\n"},
		{"a setting under no name it knows", "concurrency:\n  cancel: false\n"},
		{"a cancellation that is neither true nor false", "concurrency:\n  cancel-in-progress: maybe\n"},
		{"a setting nested deeper than the block", "concurrency:\n  group:\n    name: one\n"},
		{"a line that is not a key", "concurrency:\n  group\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, err := publish.ConcurrencyIn(strings.NewReader(c.text)); err == nil {
				t.Errorf("%s was read rather than refused", c.name)
			}
		})
	}
}
