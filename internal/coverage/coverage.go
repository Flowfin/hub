// Package coverage reads what `go test -cover` printed and refuses a package
// that is below the floor.
//
// The floor is deliberately below every value the tree holds today, and that is
// the design rather than an oversight. A coverage number rises when somebody
// writes a test that calls a function and asserts nothing, so a floor set near
// the current value turns into a target and buys tests written for the number.
// What this refuses instead is the case a percentage is actually good at: a
// package that arrives with no test at all, or with one test over a corner of
// it, which is invisible in a green suite because a package nobody tests fails
// nothing.
//
// It reads the printed summary rather than a coverage profile. The summary is
// what the same command already prints, so the leg is the suite plus one flag
// and there is no profile file to write, keep or clean up. The bound that comes
// with that is stated where it is decided, in Judge: this sees per-package
// percentages and never which statements were missed, so it cannot say anything
// about which branch of a refusal was reached. decisions/gate-parity.md is where
// that bound is argued against the alternative.
package coverage

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Floor is the percentage of statements a package must execute.
//
// Measured 2026-08-09 on the tree this landed against, the lowest package was
// the entry point at 48.1% and every other package was above 73%:
//
//	go test ./... -cover -count=1
//
// The number below sits under all of them on purpose, so that it refuses a
// package nobody tested and never a package somebody is still writing tests for.
// Raising it is a decision somebody takes and records, not a ratchet that
// follows the tree upward.
const Floor = 40.0

// Package is one line of the summary.
type Package struct {
	// Path is the import path the summary named.
	Path string

	// Percent is the statements it executed. A package with no test file is
	// zero here, and Tested says which of the two it was.
	Percent float64

	// Tested is false for a package the toolchain reported as having no test
	// files, which prints no percentage at all.
	Tested bool
}

// Read parses the summary `go test -cover` writes.
//
// Three line shapes carry a verdict and each is read. A tested package prints
// `ok`, its path, a duration and a coverage clause. A package with no test file
// prints `?` and a bracketed note instead. A failing package prints FAIL, and it
// is not this check's to report: the test leg already refused it, and a second
// leg saying so would send a reader to the wrong place.
func Read(stdout string) []Package {
	var out []Package
	for _, line := range strings.Split(strings.ReplaceAll(stdout, "\r\n", "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "?":
			out = append(out, Package{Path: fields[1]})
		case "ok":
			percent, found := percentIn(line)
			if !found {
				// `ok` with no coverage clause is a run that was not asked for
				// coverage. Reporting it as zero would refuse a package for the
				// caller's mistake, so it is left out and Judge refuses the
				// empty result instead.
				continue
			}
			out = append(out, Package{Path: fields[1], Percent: percent, Tested: true})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// clause is what the toolchain prints in front of the number.
const clause = "coverage: "

func percentIn(line string) (float64, bool) {
	i := strings.Index(line, clause)
	if i < 0 {
		return 0, false
	}
	rest := line[i+len(clause):]
	j := strings.Index(rest, "%")
	if j < 0 {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(rest[:j]), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// Judge refuses every package under the floor, and refuses a run that read no
// package at all.
//
// The second half is the one worth having. A summary this reader did not
// understand produces an empty list, which is indistinguishable from a tree
// where every package passed, and the difference is a leg that has quietly
// stopped checking anything.
func Judge(stdout string, floor float64) error {
	packages := Read(stdout)
	if len(packages) == 0 {
		return fmt.Errorf("no package coverage was read from the run, so this leg judged nothing; the summary it expects is what `go test ./... -cover` prints")
	}

	var below []string
	for _, p := range packages {
		if !p.Tested {
			below = append(below, fmt.Sprintf("%s carries no test file", p.Path))
			continue
		}
		if p.Percent < floor {
			below = append(below, fmt.Sprintf("%s ran %.1f%% of its statements", p.Path, p.Percent))
		}
	}
	if len(below) == 0 {
		return nil
	}
	return fmt.Errorf("%d of %d package(s) are under the %.0f%% floor: %s",
		len(below), len(packages), floor, strings.Join(below, "; "))
}
