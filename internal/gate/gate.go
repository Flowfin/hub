// Package gate holds the legs of the merge gate, the runner that executes them
// in order, and the reader that keeps the workflow file's job list tied to the
// leg list.
//
// There is one entry point rather than one workflow file per idea, so the same
// legs run from a shell before a push and from the job afterwards, and there is
// one procedure to keep correct instead of two that drift. The two ends of that
// promise are held together by a test rather than by this sentence: JobsIn reads
// the jobs the workflow declares, and the suite refuses a leg with no job and a
// job with no leg.
//
// What each leg is made of comes from decisions/means.md, which is why every
// Argv below is the toolchain and nothing installed beside it. Legs is the
// authority for the list; a run prints it, and no comment here repeats it.
package gate

import (
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"

	"flowfin.dev/hub/internal/coverage"
)

// Leg is one thing the gate checks.
//
// Name is what a contributor types. The check run a pull request shows is that
// name behind JobNamePrefix, which is why the two are not the same string.
type Leg struct {
	Name string

	// Argv is the command the leg runs. It is data rather than a closure so
	// that the report can print what a leg would run without running it.
	Argv []string

	// Refuses says what failure this leg prevents, in the words the report
	// prints when the leg is announced.
	Refuses string

	// OutputIsTheVerdict marks a leg whose exit status does not carry its
	// finding. Verdict is where that is acted on and where the reason is.
	OutputIsTheVerdict bool

	// Judge, when set, decides the leg from what it printed rather than from
	// the fact that it printed anything. OutputIsTheVerdict covers the one
	// shape where any output at all is the refusal; a leg whose output is a
	// measurement needs the measurement read, and this is where that reading
	// is supplied. It is a function while Argv is data on purpose: the report
	// prints what a leg would run without running it, and a judgement has
	// nothing to print.
	Judge func(stdout string) error
}

// JobNamePrefix is what a leg's name carries in front of it to become the name
// of the check run on a pull request.
//
// The bare words are not available. This repository already shows check runs
// named build, deploy and report-build-status on main, produced by the Pages
// deployment, which declares no file under .github/workflows:
//
//	gh api repos/Flowfin/hub/actions/workflows --jq '.workflows[] | "\(.name)\t\(.path)"'
//
// A leg named build would therefore report under a name this tree does not
// control, and the ruleset in #48 would require that one instead of this one.
const JobNamePrefix = "Gate: "

// CheckRunName is the name the leg's job reports under.
func (l Leg) CheckRunName() string { return JobNamePrefix + l.Name }

// ReadsItsOwnOutput reports whether the runner has to keep what the leg printed
// rather than passing it straight through.
//
// Both ways a leg can be decided by its output need the same thing from the
// runner, and forgetting one of them is silent: the judgement then reads an
// empty string, which for the coverage leg means it judged no package and for
// gofmt would mean it listed no file. Naming the question once is what keeps the
// two answers from drifting apart.
func (l Leg) ReadsItsOwnOutput() bool { return l.OutputIsTheVerdict || l.Judge != nil }

// Legs is the gate, in the order it runs.
//
// Build comes before test because a compile error reported as a test failure
// sends a reader to the wrong file, and format comes last because it is the one
// failure that never changes what the other two would have said.
func Legs() []Leg {
	return []Leg{
		{
			Name:    "build",
			Argv:    []string{"go", "build", "./..."},
			Refuses: "a tree that does not compile",
		},
		{
			Name:    "test",
			Argv:    []string{"go", "test", "./..."},
			Refuses: "a suite with a failing test",
		},
		{
			Name:               "format",
			Argv:               []string{"gofmt", "-l", "."},
			Refuses:            "a Go file gofmt would rewrite",
			OutputIsTheVerdict: true,
		},
		{
			// editorconfig, the half of formatting gofmt does not reach: the
			// HTML, the JSON, the workflow YAML and the prose, which are most
			// of the tree. Its own leg rather than a widening of format
			// because the two are decided by different things and fail for
			// different reasons - one is the toolchain's formatter, the other
			// is three properties .editorconfig states - and because a leg
			// runs one command.
			Name:    "editorconfig",
			Argv:    []string{"go", "test", "./internal/format", "-run", "TestTrackedTreeIsFormatted", "-count=1"},
			Refuses: "a tracked file with no final newline, with trailing whitespace, or indented with a tab outside Go",
		},
		{
			// gate-tests-reach-nothing, which decisions/headless-and-unelevated.md
			// names as a leg of this gate. It is its own leg rather than a test
			// inside the test leg because the decision asks for a name a reader
			// sees, and because a red here means something different from a red
			// test: the suite is asking the runner for something the gate has
			// decided it may not have.
			Name:    "tests-reach-nothing",
			Argv:    []string{"go", "test", "./internal/reach", "-run", "TestTheTreeItselfPasses", "-count=1"},
			Refuses: "a gate test reaching for the network, a display, elevation or a privileged port",
		},
		{
			// no-hardcoded-names, which decisions/names-are-data.md names as a
			// leg of this gate. Its own leg for the same reason as the one above:
			// a red here is not a failing test, it is a name that belongs in the
			// declarations having been written into the tree instead.
			Name:    "no-hardcoded-names",
			Argv:    []string{"go", "test", "./internal/names", "-run", "TestTheTreeItselfPasses", "-count=1"},
			Refuses: "an account or organisation name written into source or into a workflow step",
		},
		{
			// site-fetches-nothing-outside, which decisions/data-posture.md asks
			// for. Its own leg because a red here is not a broken page: it is the
			// site having grown a request to somebody else, which is a disclosure
			// obligation for every operator rather than a defect in a build.
			Name:    "site-fetches-nothing-outside",
			Argv:    []string{"go", "test", "./internal/site", "-run", "TestTheServedPagesFetchNothingFromAnybodyElse", "-count=1"},
			Refuses: "a served page that would load from another host, or that carries no restrictive content policy",
		},
		{
			// site-links-resolve, the half of #36 that needs nothing running.
			// Its own leg rather than a widening of the one above because the
			// two ask different questions of the same attribute: that one
			// refuses a page for fetching from somebody else, this one refuses
			// a page for pointing at a file the site does not carry, and an
			// anchor is invisible to the first and is the second's main case.
			//
			// Its subject is the served directory and stops there. The links in
			// the Markdown at the repository root rot the same way and are not
			// the site; internal/links says so where the scope is set, so a
			// green here is not read as covering them.
			Name:    "site-links-resolve",
			Argv:    []string{"go", "test", "./internal/links", "-run", "TestTheServedSitesLinksResolve", "-count=1"},
			Refuses: "a link in a served page pointing at a file the site does not carry",
		},
		{
			// coverage, adopted in decisions/gate-parity.md. It runs the suite
			// a second time rather than sharing the test leg's run, because a
			// leg is one command and a leg that depended on another leg's
			// output would stop being runnable on its own.
			//
			// Its exit status says the suite passed and says nothing about the
			// numbers, which is why it carries a Judge: `go test -cover` prints
			// a percentage per package and exits zero over a package that ran
			// none of its statements.
			Name:    "coverage",
			Argv:    []string{"go", "test", "./...", "-cover", "-count=1"},
			Refuses: "a package under the coverage floor, or one carrying no test file at all",
			Judge:   func(stdout string) error { return coverage.Judge(stdout, coverage.Floor) },
		},
		{
			// site-declares-its-language, which decisions/site-language.md asks
			// for. Its own leg because a red here is neither a broken page nor a
			// wrong link: it is a page a screen reader will pronounce with the
			// wrong voice, or a page left behind in the language the site used
			// to publish in, and neither is visible to anybody reading it on a
			// screen.
			Name:    "site-declares-its-language",
			Argv:    []string{"go", "test", "./internal/lang", "-run", "TestTheServedPagesDeclareTheirLanguage", "-count=1"},
			Refuses: "a served page that declares no language, or one the site does not publish in",
		},
	}
}

// Names lists the leg names in run order.
func Names(legs []Leg) []string {
	out := make([]string, 0, len(legs))
	for _, l := range legs {
		out = append(out, l.Name)
	}
	return out
}

// Lookup finds a leg by name.
func Lookup(legs []Leg, name string) (Leg, bool) {
	for _, l := range legs {
		if l.Name == name {
			return l, true
		}
	}
	return Leg{}, false
}

// Verdict turns one leg's standard output and exit status into a pass or a
// refusal.
//
// It is its own function, and takes the output as well as the status, because
// one leg cannot be judged by its status at all. gofmt -l prints the files it
// would rewrite and exits zero whether it printed any or not, so a runner that
// reads only the status reports a tree gofmt would rewrite as a formatted one.
// That is the whole reason OutputIsTheVerdict exists, and deleting the branch
// below turns the format leg green on an unformatted tree.
func Verdict(l Leg, stdout string, exitErr error) error {
	if exitErr != nil {
		return fmt.Errorf("%s: %w", strings.Join(l.Argv, " "), exitErr)
	}
	if l.Judge != nil {
		if err := l.Judge(stdout); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(l.Argv, " "), err)
		}
		return nil
	}
	if !l.OutputIsTheVerdict {
		return nil
	}
	listed := listedFiles(stdout)
	if len(listed) == 0 {
		return nil
	}
	return fmt.Errorf("%s named %d file(s): %s",
		strings.Join(l.Argv, " "), len(listed), strings.Join(listed, ", "))
}

// listedFiles splits a leg's output into the file names it printed, ignoring
// blank lines and the carriage return a Windows shell leaves on each one.
func listedFiles(stdout string) []string {
	var out []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// Outcome is what a run did with one leg.
type Outcome int

const (
	// Passed means the leg ran and refused nothing.
	Passed Outcome = iota
	// Failed means the leg ran and refused something.
	Failed
	// NotReached means the leg was asked for and an earlier leg failed first.
	NotReached
	// NotAsked means this run was asked for a subset that excluded the leg.
	NotAsked
)

func (o Outcome) String() string {
	switch o {
	case Passed:
		return "passed"
	case Failed:
		return "FAILED"
	case NotReached:
		return "not reached"
	case NotAsked:
		return "not asked for"
	}
	return "unknown"
}

// Result is one leg's line in the report.
type Result struct {
	Leg     Leg
	Outcome Outcome
	Err     error
}

// Executor runs one leg and returns nil when the leg refuses nothing.
type Executor func(Leg) error

// Run executes the asked-for legs in order, stops at the first failure, and
// writes a report saying what it examined.
//
// The report is the point of the last paragraph as much as the running is. A
// run that covered two legs of three, because it was asked for two or because
// the second failed, must not read as a run that covered three and found
// nothing, so every leg appears in the summary with what happened to it and the
// count is stated rather than left to be counted.
//
// It returns the per-leg results and an error when any asked-for leg failed.
func Run(out io.Writer, legs []Leg, asked []string, run Executor) ([]Result, error) {
	wanted := map[string]bool{}
	for _, name := range asked {
		wanted[name] = true
	}
	all := len(asked) == 0

	results := make([]Result, 0, len(legs))
	stopped := ""
	for _, l := range legs {
		switch {
		case !all && !wanted[l.Name]:
			results = append(results, Result{Leg: l, Outcome: NotAsked})
		case stopped != "":
			results = append(results, Result{Leg: l, Outcome: NotReached})
		default:
			fmt.Fprintf(out, "gate: %s: %s (%s)\n", l.Name, l.Refuses, strings.Join(l.Argv, " "))
			err := run(l)
			if err != nil {
				results = append(results, Result{Leg: l, Outcome: Failed, Err: err})
				stopped = l.Name
				continue
			}
			results = append(results, Result{Leg: l, Outcome: Passed})
		}
	}

	report(out, results, stopped)

	var failed []string
	for _, r := range results {
		if r.Outcome == Failed {
			failed = append(failed, r.Leg.Name)
		}
	}
	if len(failed) > 0 {
		return results, fmt.Errorf("gate refused: %s", strings.Join(failed, ", "))
	}
	return results, nil
}

// report writes the summary. Every leg is named, so the set of legs this
// repository has is readable from the output of any run, however small.
func report(out io.Writer, results []Result, stopped string) {
	ran := 0
	for _, r := range results {
		if r.Outcome == Passed || r.Outcome == Failed {
			ran++
		}
	}

	width := 0
	for _, r := range results {
		if n := len(r.Leg.Name); n > width {
			width = n
		}
	}

	fmt.Fprintf(out, "\ngate examined %d of %d legs.\n", ran, len(results))
	for _, r := range results {
		line := fmt.Sprintf("  %-*s  %s", width, r.Leg.Name, r.Outcome)
		switch {
		case r.Outcome == NotReached && stopped != "":
			line += fmt.Sprintf(", because %s failed first", stopped)
		case r.Outcome == Failed && r.Err != nil:
			line += ": " + r.Err.Error()
		}
		fmt.Fprintln(out, line)
	}
}

// Shell is the Executor that actually runs a leg, in dir.
//
// Standard output is captured rather than streamed for a leg whose output is
// its verdict, and echoed afterwards, so that the file names gofmt printed are
// in the log under the leg that printed them.
func Shell(out io.Writer, dir string) Executor {
	return func(l Leg) error {
		cmd := exec.Command(l.Argv[0], l.Argv[1:]...)
		cmd.Dir = dir
		cmd.Stderr = out

		if !l.ReadsItsOwnOutput() {
			cmd.Stdout = out
			return Verdict(l, "", cmd.Run())
		}

		var captured strings.Builder
		cmd.Stdout = &captured
		err := cmd.Run()
		if captured.Len() > 0 {
			fmt.Fprint(out, captured.String())
		}
		return Verdict(l, captured.String(), err)
	}
}

// Unknown lists the asked-for names that are not legs, sorted, so a typed leg
// name that matches nothing is refused rather than silently running nothing.
func Unknown(legs []Leg, asked []string) []string {
	var out []string
	for _, name := range asked {
		if _, ok := Lookup(legs, name); !ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
