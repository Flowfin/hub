// Package harness holds the environment-bound checks: the ones
// decisions/headless-and-unelevated.md keeps out of the merge gate because they
// need something a clean runner does not have.
//
// A requirement is named for what it needs rather than for what it tests, so a
// reader of a job list can tell which runs would go red because the internet was
// down without opening a file. That name is the whole apparatus: it is the build
// tag on the test file, the name of the job that runs it, and the word a person
// types to ask for it.
//
// Two properties come with the harness and both are held by tests rather than by
// this comment. The gate never depends on it, so no leg of the gate runs a
// harness tag and no leg reads a harness result. And a requirement that did not
// run says so, with what asking for it would cost, because a green gate beside a
// silent harness reads as everything passing and that reading is wrong in exactly
// the cases the harness exists for.
package harness

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"flowfin.dev/hub/internal/reach"
)

// WorkflowPath is the harness's workflow file, relative to the repository root.
const WorkflowPath = ".github/workflows/harness.yml"

// Requirement is one thing the runner has to supply before a check can run.
type Requirement struct {
	// Name is what a person types, what the job reports under, and what the
	// build tag spells with underscores. decisions/headless-and-unelevated.md
	// fixes these three names; the tag spelling below is what this package
	// fixes, because a Go build constraint cannot carry a hyphen.
	Name string

	// Reaches is what a check under this requirement is allowed to reach for.
	Reaches string

	// Costs is what asking for a run costs, in the words the disclosure prints.
	// It is here rather than in the workflow file because the sentence a person
	// reads next to a green gate is the point of the field.
	Costs string
}

// Tag is the build constraint a test file carries to belong to this
// requirement. It is the name with hyphens turned into underscores, and
// internal/reach spares a file carrying it.
func (r Requirement) Tag() string { return strings.ReplaceAll(r.Name, "-", "_") }

// Requirements is the harness, in the order the disclosure lists it.
//
// The three are decisions/headless-and-unelevated.md's, one requirement per
// name. A check needing two carries both tags, because a name covering a set of
// requirements stops answering the question it exists for.
func Requirements() []Requirement {
	return []Requirement{
		{
			Name:    "needs-network",
			Reaches: "a request off the runner: the published manifest at its address, a certificate or a redirect, the release API against the world rather than against a fixture",
			Costs:   "a run that goes red when somebody else's service is having an afternoon, and a rate limit shared with everything else using the same token",
		},
		{
			Name:    "needs-browser",
			Reaches: "a real render of the site: the numbers in the speed budget, focus behaviour, the accessibility rules judged against what a browser actually paints",
			Costs:   "a browser and its dependencies installed on the runner, and measurements that move with the runner's load rather than with the change",
		},
		{
			Name:    "needs-jellyfin",
			Reaches: "a running server: adding the repository address, listing the catalogue, installing a plugin and watching it load",
			Costs:   "a Jellyfin server to talk to, brought up for the run and torn down after it, and a failure that can be the server rather than the plugin",
		},
	}
}

// Lookup finds a requirement by name.
func Lookup(rs []Requirement, name string) (Requirement, bool) {
	for _, r := range rs {
		if r.Name == name {
			return r, true
		}
	}
	return Requirement{}, false
}

// Names lists the requirement names in order.
func Names(rs []Requirement) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Name)
	}
	return out
}

// TaggedFiles maps every harness tag a test file under root carries to the
// files carrying it, including a tag no requirement declares.
//
// It answers two questions with one walk. Which files a requirement would run,
// so that a requirement nothing carries can be told from one something does:
// `go test` over a tag no file carries compiles nothing, runs nothing and exits
// zero, which is the absent result rendered as a clean one. And which tags
// nobody declared, which is the worse case. A file tagged needs_netwrok is
// spared by gate-tests-reach-nothing, because that check reads only the prefix,
// and is run by no job here, because no job names it. It runs nowhere and
// nothing says so.
func TaggedFiles(root string) (map[string][]string, error) {
	found := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != root && (name == ".git" || name == "testdata") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, tag := range harnessTagsIn(content) {
			found[tag] = append(found[tag], filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for tag := range found {
		sort.Strings(found[tag])
	}
	return found, nil
}

func harnessTagsIn(content []byte) []string {
	var out []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "//") {
			return out
		}
		if !strings.HasPrefix(line, "//go:build ") {
			continue
		}
		for _, field := range strings.FieldsFunc(strings.TrimPrefix(line, "//go:build "), func(r rune) bool {
			return r == ' ' || r == '(' || r == ')' || r == '!' || r == '&' || r == '|' || r == ','
		}) {
			if strings.HasPrefix(field, reach.HarnessTagPrefix) {
				out = append(out, field)
			}
		}
	}
	return out
}

// Disclosure writes what the harness did on this run, which is nothing, and what
// asking would cost.
//
// It is printed beside the gate's own report because that is where a person
// reads the result, and a green gate with no mention of the harness is the
// reading the decision calls wrong. Nothing here is a verdict about the world:
// every line says the check did not run.
func Disclosure(w io.Writer, rs []Requirement, carriers map[string][]string) {
	fmt.Fprintf(w, "\nthe harness did not run here, and none of the %d requirements below was asked for.\n", len(rs))
	fmt.Fprintf(w, "no leg above depends on any of them, so this verdict is complete and not partial.\n")

	width := 0
	for _, r := range rs {
		if n := len(r.Name); n > width {
			width = n
		}
	}
	for _, r := range rs {
		files := carriers[r.Tag()]
		switch len(files) {
		case 0:
			fmt.Fprintf(w, "  %-*s  no check carries it yet; it reaches %s\n", width, r.Name, r.Reaches)
		default:
			fmt.Fprintf(w, "  %-*s  %d file(s) carry it: %s\n", width, r.Name, len(files), strings.Join(files, ", "))
		}
		fmt.Fprintf(w, "  %-*s  asking costs %s\n", width, "", r.Costs)
	}
	fmt.Fprintf(w, "ask for one with `go run . harness <requirement>`, or run the job of that name in %s.\n", WorkflowPath)
}

// Triggers reads the event names a workflow file declares under `on:`.
//
// It is a line reader over a two-space-indented block for the same reason
// internal/gate's is: a YAML parser would be the first dependency in a tree
// whose empty dependency set CONTRIBUTING.md spends a section on, for one file
// this repository writes itself. It refuses what it cannot read rather than
// guessing, because a trigger this reader missed is the harness running on every
// pull request with nothing saying so.
func Triggers(r io.Reader) ([]string, error) {
	var (
		out  []string
		inOn bool
		seen bool
	)
	scan := bufio.NewScanner(r)
	line := 0
	for scan.Scan() {
		line++
		text := strings.TrimRight(scan.Text(), "\r")
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if !strings.HasPrefix(text, " ") {
			// A key at column zero ends whatever block was open. The inline
			// spellings `on: push` and `on: [push, pull_request]` are read
			// here, because a reader that only understood the block form would
			// pass a workflow written either of the other two ways.
			if !strings.HasPrefix(trimmed, "on:") {
				inOn = false
				continue
			}
			if seen {
				return nil, fmt.Errorf("line %d: the workflow declares `on:` twice", line)
			}
			seen = true
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "on:"))
			if rest == "" {
				inOn = true
				continue
			}
			inOn = false
			for _, name := range strings.Split(strings.Trim(rest, "[]"), ",") {
				if name = strings.TrimSpace(name); name != "" {
					out = append(out, name)
				}
			}
			continue
		}
		if !inOn {
			continue
		}
		if indent(text) != 2 {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "- "):
			out = append(out, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		case strings.HasSuffix(trimmed, ":"):
			out = append(out, strings.TrimSuffix(trimmed, ":"))
		default:
			return nil, fmt.Errorf("line %d: %q is not a trigger this reader understands", line, trimmed)
		}
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	if !seen {
		return nil, fmt.Errorf("the workflow declares no `on:` block, so nothing says when it runs")
	}
	return out, nil
}

func indent(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

// Deliberate is the only trigger the harness may declare: a person asking for
// it. A schedule is excluded as well as the pull-request events, because a
// harness that runs on a timer produces a result nobody asked for and a red
// nobody is holding.
const Deliberate = "workflow_dispatch"

// AutomaticTriggers returns the declared triggers that are not deliberate.
func AutomaticTriggers(declared []string) []string {
	var out []string
	for _, t := range declared {
		if t != Deliberate {
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}
