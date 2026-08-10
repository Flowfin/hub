// Package sweep reports a scheduled run that ended in something other than
// success, so that a broken publish is not first noticed by an operator asking
// why a version is old.
//
// Everything that runs on a schedule here fails silently by construction.
// Nobody is watching at the time it runs, the red sits in a tab nobody opens,
// and the run leaves no mark anywhere a person looks. The freshness check reads
// the published file from outside and covers one specific way that goes wrong;
// this covers the case where the check itself never ran.
//
// Two properties decide whether such a sweep is useful or noise, and both are
// held by tests in this package rather than by this comment.
//
// It derives the set it watches from the workflow files rather than carrying a
// list. A list means the day somebody adds a scheduled job it is outside the
// watch and nothing says so, which is the same silence one rung up. The
// derivation is a line reader over the trigger block and not a YAML parser, for
// the reason internal/gate gives for the same choice: a parser would be the
// first entry in a dependency set CONTRIBUTING.md spends a section on, for
// files this repository writes itself. It refuses what it cannot read rather
// than guessing.
//
// It raises one tracking issue per distinct failure rather than one per run.
// A workflow that has been failing every night for a week is one thing to fix,
// and an issue per run is the thing everybody filters out, after which the
// sweep is worse than nothing because it looks like cover.
//
// What this package does not reach: the runs and the tracker themselves, which
// are in github.go behind a client, so every judgement below is decided against
// a fixture.
package sweep

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// WorkflowDir is where the workflow files live, relative to the repository
// root. It is the directory rather than a list of files, because the list is
// what this package refuses to carry.
const WorkflowDir = ".github/workflows"

// OwnWorkflow is the sweep's own workflow file.
//
// It is named here for one property a test pins: the sweep is scheduled, so it
// derives itself into the set it watches. A sweep run that dies cannot report
// its own death, and the next run that lives is what reports it. That is the
// whole of the answer to who watches this, and it is a delay rather than a gap.
const OwnWorkflow = WorkflowDir + "/sweep.yml"

// KeyPrefix opens the line a raised issue carries so that a later sweep can
// tell a failure it has already reported from a new one.
//
// The marker is in the body rather than in the title, because a title is a
// thing people rewrite and a run that reported a failure twice because somebody
// improved the wording is the noise this exists against.
const KeyPrefix = "Sweep-key:"

// Scheduled reads the workflow directory and returns the paths of the workflows
// that declare a schedule trigger, in path order.
//
// An empty result is not an error. A tree with nothing scheduled has nothing to
// watch, and the report says that in those words rather than passing quietly.
func Scheduled(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, WorkflowDir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", WorkflowDir, err)
	}

	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if ext := path.Ext(name); ext != ".yml" && ext != ".yaml" {
			continue
		}
		full := WorkflowDir + "/" + name

		file, err := fsys.Open(full)
		if err != nil {
			return nil, fmt.Errorf("opening %s: %w", full, err)
		}
		scheduled, err := DeclaresSchedule(file)
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", full, err)
		}
		if scheduled {
			out = append(out, full)
		}
	}
	sort.Strings(out)
	return out, nil
}

// DeclaresSchedule reads one workflow file and reports whether its trigger
// block declares a schedule.
//
// The near-miss this is written against is in this tree already: a workflow
// whose comment explains that a schedule is deliberately excluded. A reader
// that greps for the word watches that file forever and reports nothing about
// it, because it never runs on a schedule and therefore never fails on one. So
// comments are dropped before anything is decided, and the word is only read as
// a trigger where a trigger can stand.
func DeclaresSchedule(r io.Reader) (bool, error) {
	var (
		inTriggers bool
		sawTrigger bool
		scheduled  bool
	)

	scan := bufio.NewScanner(r)
	line := 0
	for scan.Scan() {
		line++
		text := strings.TrimRight(scan.Text(), "\r")
		if cut := strings.IndexByte(text, '#'); cut == 0 {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// A key at column zero ends whatever block was open and may start the
		// trigger block. YAML reads a bare `on` as true, so the key is written
		// quoted in some trees and bare in this one, and both are the same key.
		if !strings.HasPrefix(text, " ") && !strings.HasPrefix(text, "\t") {
			key, rest, ok := strings.Cut(trimmed, ":")
			if !ok {
				return false, fmt.Errorf("line %d: %q is not a key", line, trimmed)
			}
			inTriggers = unquote(strings.TrimSpace(key)) == "on"
			if !inTriggers {
				continue
			}
			if sawTrigger {
				return false, fmt.Errorf("line %d: the trigger block is opened twice", line)
			}
			sawTrigger = true

			// `on: push` and `on: [push, workflow_dispatch]` put the whole block
			// on one line, which is legal and which this tree does not use.
			// Such a block never declares a schedule: a schedule is the trigger
			// that carries its own settings, so it has to be a key with a cron
			// under it, and the server refuses the word in a flow sequence.
			// Reading the form is still worth it, because a file this reader
			// refused would be a file outside the watch.
			if inline := strings.TrimSpace(stripComment(rest)); inline != "" {
				inTriggers = false
			}
			continue
		}
		if !inTriggers {
			continue
		}

		// Inside the block, a trigger is a key at the first indent level. A
		// deeper key belongs to a trigger's own settings, and `cron` under a
		// schedule is the one that would otherwise be read as a second trigger.
		if indent(text) != 2 {
			continue
		}
		key, _, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) == "schedule" {
			scheduled = true
		}
	}
	if err := scan.Err(); err != nil {
		return false, err
	}
	if !sawTrigger {
		return false, fmt.Errorf("declares no trigger block, so nothing here can say when it runs")
	}
	return scheduled, nil
}

// Run is the part of a workflow run this package reads.
type Run struct {
	// Workflow is the path of the file that declared it, which is what ties a
	// run back to the derived set rather than to a display name somebody can
	// rename without meaning to.
	Workflow string

	Number     int
	Event      string
	Status     string
	Conclusion string
	Branch     string
	URL        string
	StartedAt  string
}

// Ended reports whether the run reached a verdict. A run still going has no
// conclusion, and reading an absent conclusion as a failure would raise an
// issue against every sweep that overlapped one.
func (r Run) Ended() bool { return r.Status == "completed" && r.Conclusion != "" }

// Failure is one distinct failure: a watched workflow and the verdict it
// reached, with the runs that showed it.
type Failure struct {
	Workflow   string
	Conclusion string

	// Runs are the runs that reached this verdict, newest first. They are the
	// evidence in the raised issue and they are not what the issue is keyed on.
	Runs []Run
}

// Key is what a raised issue carries so a later sweep can recognise it.
//
// The workflow and the verdict, and nothing about the run. A workflow failing
// again tomorrow night for the same reason is the same thing to fix, and a key
// carrying a run id would raise an issue every night.
func (f Failure) Key() string { return f.Workflow + " " + f.Conclusion }

// Select reduces the runs to the distinct failures worth raising.
//
// Three filters, and each of them is a way this would otherwise be noise. Only
// a watched workflow, so a run of something nobody scheduled is not swept up.
// Only the schedule event, because a run somebody asked for has somebody
// looking at it and this exists for the runs nobody asked for. Only the default
// branch, because a scheduled run is a run of that branch and anything else
// arriving here is a fact about the reader rather than about the tree.
func Select(watched []string, runs []Run, defaultBranch string) []Failure {
	inSet := map[string]bool{}
	for _, w := range watched {
		inSet[w] = true
	}

	grouped := map[string]*Failure{}
	for _, r := range runs {
		switch {
		case !inSet[r.Workflow], r.Event != "schedule", r.Branch != defaultBranch:
			continue
		case !r.Ended(), r.Conclusion == "success":
			continue
		}

		f, ok := grouped[r.Workflow+" "+r.Conclusion]
		if !ok {
			f = &Failure{Workflow: r.Workflow, Conclusion: r.Conclusion}
			grouped[f.Key()] = f
		}
		f.Runs = append(f.Runs, r)
	}

	out := make([]Failure, 0, len(grouped))
	for _, f := range grouped {
		sort.SliceStable(f.Runs, func(i, j int) bool { return f.Runs[i].Number > f.Runs[j].Number })
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}

// Issue is an open tracking issue as this package reads it.
type Issue struct {
	Number int
	Body   string
}

// KeysIn reads the sweep keys an issue body carries.
//
// The marker is read at column zero only. A key quoted inside a sentence, or
// inside the fenced block of an issue explaining how this works, is not a
// report of that failure, and reading one would silence the sweep for a
// workflow nobody is holding.
func KeysIn(body string) []string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, "\r")
		if !strings.HasPrefix(line, KeyPrefix) {
			continue
		}
		if key := strings.TrimSpace(strings.TrimPrefix(line, KeyPrefix)); key != "" {
			out = append(out, key)
		}
	}
	return out
}

// Unreported returns the failures no open issue is already tracking, and the
// ones an open issue holds.
//
// Both halves are returned because a run that raised nothing and a run that
// found nothing are different states, and the report says which.
func Unreported(failures []Failure, open []Issue) (fresh, held []Failure) {
	tracked := map[string]int{}
	for _, i := range open {
		for _, key := range KeysIn(i.Body) {
			tracked[key] = i.Number
		}
	}
	for _, f := range failures {
		if _, ok := tracked[f.Key()]; ok {
			held = append(held, f)
			continue
		}
		fresh = append(fresh, f)
	}
	return fresh, held
}

// Title is what the raised issue is called.
func (f Failure) Title() string {
	return fmt.Sprintf("%s ended in %s on its schedule, and nobody was watching",
		path.Base(f.Workflow), f.Conclusion)
}

// Body is what the raised issue says.
//
// It carries the key at column zero, the runs that showed the failure, and what
// closing it means, because a tracking issue whose closing condition is unwritten
// is one somebody closes to make the list shorter.
func (f Failure) Body() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n\n", KeyPrefix, f.Key())
	fmt.Fprintf(&b, "`%s` runs on a schedule, and its last run on the default branch ended in %s.\n",
		f.Workflow, f.Conclusion)
	b.WriteString("A scheduled run has nobody in front of it at the time it runs, ")
	b.WriteString("so this is raised here rather than left in a tab.\n\n")

	fmt.Fprintf(&b, "## The runs that reached %s\n\n", f.Conclusion)
	for _, r := range f.Runs {
		fmt.Fprintf(&b, "- run %d, started %s: %s\n", r.Number, r.StartedAt, r.URL)
	}

	b.WriteString("\n## What closes this\n\n")
	b.WriteString("A run of the same workflow that ends in success, and whatever ")
	b.WriteString("change made it do so. Closing this while the workflow is still ")
	b.WriteString("failing raises it again on the next sweep, under the same key, ")
	b.WriteString("which is the behaviour rather than a defect.\n\n")
	b.WriteString("The line at the top is how a later sweep recognises this issue. ")
	b.WriteString("Removing it makes the next failure raise a second issue.\n")
	return b.String()
}

// Report says what the sweep watched and what it found, in the words a reader
// gets whether or not anything was raised.
//
// A run that watched nothing and a run that watched everything and was content
// print differently, which is the same obligation the gate's own report carries.
func Report(watched, unwatched, never []string, fresh, held []Failure, raised map[string]int) string {
	var b strings.Builder

	if len(watched) == 0 {
		b.WriteString("no workflow in this tree declares a schedule, so this run watched nothing.\n")
	} else {
		fmt.Fprintf(&b, "watched %d scheduled workflow(s):\n", len(watched))
		for _, w := range watched {
			fmt.Fprintf(&b, "  %s\n", w)
		}
	}
	for _, w := range never {
		fmt.Fprintf(&b, "  %s has no scheduled run on the server yet, so nothing here says whether its schedule fires\n", w)
	}
	if len(unwatched) > 0 {
		fmt.Fprintf(&b, "%d workflow(s) declare no schedule and are outside this watch: %s\n",
			len(unwatched), strings.Join(unwatched, ", "))
	}

	if len(fresh) == 0 && len(held) == 0 {
		b.WriteString("no watched workflow ended in anything other than success.\n")
		return b.String()
	}

	for _, f := range held {
		fmt.Fprintf(&b, "%s: %s, already tracked by an open issue\n", f.Workflow, f.Conclusion)
	}
	for _, f := range fresh {
		number, ok := raised[f.Key()]
		if !ok {
			fmt.Fprintf(&b, "%s: %s, would be raised (%d run(s)); nothing was written\n",
				f.Workflow, f.Conclusion, len(f.Runs))
			continue
		}
		fmt.Fprintf(&b, "%s: %s, raised as #%d (%d run(s))\n",
			f.Workflow, f.Conclusion, number, len(f.Runs))
	}
	return b.String()
}

// stripComment removes a trailing comment from a value.
//
// It cuts at the first hash rather than parsing quoting, which is the bound
// worth stating: a hash inside a quoted trigger name would be cut too. No
// trigger name carries one, and the alternative is a quoting parser for a value
// that is a word or a list of words.
func stripComment(s string) string {
	if cut := strings.IndexByte(s, '#'); cut >= 0 {
		return s[:cut]
	}
	return s
}

func indent(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}
