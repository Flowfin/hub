// Package scan reads what a static analyser reported and decides the run from
// it.
//
// The analyser this board adopted is CodeQL, which decisions/gate-parity.md
// records as adapted from the target's two names for one analysis. What carries
// over from the target unchanged is that a finding blocks rather than annotates,
// and that is the whole reason this package exists: the analysis step uploads its
// results to the code scanning tab and exits zero whether it found anything or
// not, so a green job is not the statement "no findings" unless something reads
// the report.
//
// The reading is here rather than in a shell fragment in the workflow file for
// the reason decisions/means.md gives. Enforcement logic in a workflow is
// enforcement logic in a language this tree has no suite for, and the mistake it
// makes is silent: a jq expression that matches nothing and a report that
// contains nothing both print nothing.
//
// Every refusal below is judged against a fixture. Nothing here runs an analyser
// or reaches the network, so the rules are decided in the gate's own suite and
// the one thing that needs a runner stays in the workflow.
package scan

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Suffix is what an analysis writes its report as.
const Suffix = ".sarif"

// Reason is why a report was refused. An enumeration rather than a string
// because three of the four are the analysis having not happened, and only one
// of them is the analyser having found something.
type Reason int

const (
	// NoReport means the directory holds no report at all. An analysis that
	// wrote nothing is not an analysis that found nothing, and those are the
	// two outcomes this package exists between.
	NoReport Reason = iota

	// Unreadable means a report is not the JSON document the format is.
	Unreadable

	// NothingAnalysed means a report carries no run, or a run that names no
	// tool. A report of that shape is produced by an analysis that was
	// configured for a language the tree does not hold, and it is empty for the
	// same reason a clean one is.
	NothingAnalysed

	// Findings means the analyser reported something.
	Findings
)

func (r Reason) String() string {
	switch r {
	case NoReport:
		return "no-report"
	case Unreadable:
		return "unreadable-report"
	case NothingAnalysed:
		return "nothing-analysed"
	case Findings:
		return "findings"
	}
	return "unknown"
}

// Refusal is a report the run may not pass on.
type Refusal struct {
	Reason Reason
	Detail string
}

func (r *Refusal) Error() string { return fmt.Sprintf("%s: %s", r.Reason, r.Detail) }

// Finding is one thing the analyser reported, in the words the run prints.
type Finding struct {
	Report  string
	Tool    string
	Rule    string
	Level   string
	File    string
	Line    int
	Message string
}

func (f Finding) String() string {
	where := f.File
	if where == "" {
		where = "no location"
	} else if f.Line > 0 {
		where = fmt.Sprintf("%s:%d", where, f.Line)
	}
	level := f.Level
	if level == "" {
		// The format leaves the level to the rule's own metadata when the
		// result omits it. Printing an empty column would read as a severity of
		// none rather than as one this reader did not have.
		level = "unstated"
	}
	return fmt.Sprintf("%s: %s: %s [%s]: %s", where, f.Tool, f.Rule, level, f.Message)
}

// sarif is the part of the report this package reads. The format carries a great
// deal more and none of it changes the verdict.
type sarif struct {
	Runs []struct {
		Tool struct {
			Driver struct {
				Name string `json:"name"`
			} `json:"driver"`
		} `json:"tool"`
		Results []struct {
			RuleID  string `json:"ruleId"`
			Level   string `json:"level"`
			Message struct {
				Text string `json:"text"`
			} `json:"message"`
			Locations []struct {
				PhysicalLocation struct {
					ArtifactLocation struct {
						URI string `json:"uri"`
					} `json:"artifactLocation"`
					Region struct {
						StartLine int `json:"startLine"`
					} `json:"region"`
				} `json:"physicalLocation"`
			} `json:"locations"`
		} `json:"results"`
	} `json:"runs"`
}

// Read parses one report into the findings it carries and the tools that wrote
// it.
func Read(name string, body []byte) (found []Finding, tools []string, err error) {
	var doc sarif
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, nil, &Refusal{
			Reason: Unreadable,
			Detail: fmt.Sprintf("%s: %s", name, err),
		}
	}
	if len(doc.Runs) == 0 {
		return nil, nil, &Refusal{
			Reason: NothingAnalysed,
			Detail: fmt.Sprintf("%s carries no run, so nothing was analysed and the file says so rather than saying it found nothing", name),
		}
	}

	for i, run := range doc.Runs {
		tool := strings.TrimSpace(run.Tool.Driver.Name)
		if tool == "" {
			return nil, nil, &Refusal{
				Reason: NothingAnalysed,
				Detail: fmt.Sprintf("%s: run %d names no tool, and a report nothing signed is a report nothing stands behind", name, i+1),
			}
		}
		tools = append(tools, tool)

		for _, r := range run.Results {
			f := Finding{
				Report:  name,
				Tool:    tool,
				Rule:    r.RuleID,
				Level:   r.Level,
				Message: strings.TrimSpace(r.Message.Text),
			}
			if len(r.Locations) > 0 {
				f.File = r.Locations[0].PhysicalLocation.ArtifactLocation.URI
				f.Line = r.Locations[0].PhysicalLocation.Region.StartLine
			}
			found = append(found, f)
		}
	}
	return found, tools, nil
}

// Judge reads every report in the directory and refuses one that reported
// something.
//
// It returns the reports it read and the tools that wrote them, so a run says
// what it examined. A verdict with no such list cannot be told from a verdict
// over a directory that was never filled, which is the failure this whole
// package is one line away from.
func Judge(fsys fs.FS) (reports []string, tools []string, found []Finding, err error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, nil, nil, &Refusal{
			Reason: NoReport,
			Detail: fmt.Sprintf("the directory the analysis writes into could not be read: %s", err),
		}
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), Suffix) {
			continue
		}
		body, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, nil, nil, &Refusal{
				Reason: Unreadable,
				Detail: fmt.Sprintf("%s: %s", e.Name(), err),
			}
		}
		f, t, err := Read(e.Name(), body)
		if err != nil {
			return nil, nil, nil, err
		}
		reports = append(reports, e.Name())
		tools = append(tools, t...)
		found = append(found, f...)
	}

	if len(reports) == 0 {
		return nil, nil, nil, &Refusal{
			Reason: NoReport,
			Detail: fmt.Sprintf("no file ending in %s, and an analysis that wrote no report is not one that found nothing", Suffix),
		}
	}

	sort.Strings(reports)
	sort.Strings(tools)
	tools = unique(tools)
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].File != found[j].File {
			return found[i].File < found[j].File
		}
		return found[i].Line < found[j].Line
	})
	return reports, tools, found, nil
}

func unique(in []string) []string {
	out := in[:0:0]
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}

// Report writes what was examined and what was found, and returns the error the
// run exits on.
//
// The count of reports and the tool names are printed whether anything was found
// or not, for the reason the gate prints every leg: a run that examined one
// report of two must not read as a run that examined both and found nothing.
func Report(w io.Writer, dir string, reports, tools []string, found []Finding) error {
	fmt.Fprintf(w, "code scanning: %d report(s) in %s, written by %s.\n",
		len(reports), path.Clean(dir), strings.Join(tools, ", "))
	for _, r := range reports {
		fmt.Fprintf(w, "  read %s\n", r)
	}
	if len(found) == 0 {
		fmt.Fprintln(w, "  nothing was reported.")
		return nil
	}
	for _, f := range found {
		fmt.Fprintf(w, "  %s\n", f)
	}
	return &Refusal{
		Reason: Findings,
		Detail: fmt.Sprintf("%d finding(s) across %d report(s)", len(found), len(reports)),
	}
}
