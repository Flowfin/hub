package publish

import (
	"bufio"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

// WorkflowPath is the file that runs the publication, relative to the
// repository root.
const WorkflowPath = ".github/workflows/publish.yml"

// Group is the name of the queue a run writing to this target belongs in.
//
// It names the destination and nothing else, and that is the property rather
// than the string. Two runs that would write one file have to queue behind one
// another however each of them was started, so a group carrying the workflow,
// the ref or the run id puts a scheduled run and a hand-started one in
// different queues and lets them hold the same file at once. Deriving it from
// the target keeps the workflow's queue and the place it writes to moving
// together instead of one of them being left behind.
func (t Target) Group() string { return "publish-" + path.Join(t.Dir, t.Name) }

// Concurrency is a workflow's top-level concurrency block.
type Concurrency struct {
	// Declared is false where the file carries no such block at all, which is
	// a different state from a block naming an empty group and reads
	// differently in a refusal.
	Declared bool

	// Group is the queue the runs of that workflow are placed in.
	Group string

	// Cancels is cancel-in-progress. Absent means false on the server and is
	// read the same way here.
	Cancels bool
}

// Serialised says why runs writing to t are not serialised by c, and is nil
// where they are.
//
// The group is compared against the one the target derives rather than
// inspected for shapes that would be wrong. That is the stronger of the two:
// every expression a workflow can put in a group - the run id, the ref, the
// event - fails the comparison for the same reason a misspelling does, so the
// list of them does not have to be written down here or kept current.
func (c Concurrency) Serialised(t Target) error {
	switch {
	case !c.Declared:
		return fmt.Errorf("the workflow declares no concurrency block, so two runs can hold %s at once", t.Name)
	case c.Group != t.Group():
		return fmt.Errorf("the group is %q and a run writing to %s belongs in %q", c.Group, t.Name, t.Group())
	case c.Cancels:
		return fmt.Errorf("cancel-in-progress is true, so a newer run cancels the one holding the write to %s", t.Name)
	}
	return nil
}

// ConcurrencyIn reads the top-level concurrency block of a workflow file.
//
// It is a line reader over a two-space-indented block for the same reason
// internal/gate's and internal/harness's readers are: a YAML parser would be
// the first dependency in a tree whose empty dependency set CONTRIBUTING.md
// spends a section on, for a file this repository writes itself. It refuses a
// setting it does not understand rather than stepping over it, because a
// setting this reader skipped is a run cancelled mid-write with the suite still
// green.
func ConcurrencyIn(r io.Reader) (Concurrency, error) {
	var (
		out    Concurrency
		inside bool
	)

	scan := bufio.NewScanner(r)
	line := 0
	for scan.Scan() {
		text := strings.TrimRight(scan.Text(), "\r")
		line++
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// A key at column zero ends whatever block was open and may open this
		// one. A concurrency setting on a single job is indented under jobs and
		// is a different thing: it serialises that job and not the workflow.
		if !strings.HasPrefix(text, " ") && !strings.HasPrefix(text, "\t") {
			if !strings.HasPrefix(trimmed, "concurrency:") {
				inside = false
				continue
			}
			if out.Declared {
				return Concurrency{}, fmt.Errorf("line %d: the workflow declares `concurrency:` twice", line)
			}
			out.Declared = true

			// `concurrency: a-name` is the short form. It is legal, it names a
			// group and cancels nothing, and a reader that only understood the
			// block form would report a file written that way as having no
			// block at all.
			if rest := strings.TrimSpace(stripComment(strings.TrimPrefix(trimmed, "concurrency:"))); rest != "" {
				out.Group = unquote(rest)
				continue
			}
			inside = true
			continue
		}
		if !inside {
			continue
		}
		if indent(text) != 2 {
			return Concurrency{}, fmt.Errorf("line %d: %q is indented past the settings of the concurrency block", line, trimmed)
		}

		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			return Concurrency{}, fmt.Errorf("line %d: %q is not a key", line, trimmed)
		}
		value = strings.TrimSpace(stripComment(value))
		switch strings.TrimSpace(key) {
		case "group":
			out.Group = unquote(value)
		case "cancel-in-progress":
			cancels, err := strconv.ParseBool(unquote(value))
			if err != nil {
				return Concurrency{}, fmt.Errorf("line %d: cancel-in-progress is %q, which is neither true nor false", line, value)
			}
			out.Cancels = cancels
		default:
			return Concurrency{}, fmt.Errorf("line %d: %q is not a concurrency setting this reader understands", line, strings.TrimSpace(key))
		}
	}
	if err := scan.Err(); err != nil {
		return Concurrency{}, err
	}
	return out, nil
}

func indent(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

// stripComment removes a trailing comment from a value.
//
// It cuts at the first hash rather than parsing quoting, which is the bound
// worth stating: a hash inside a quoted group name would be cut too. No group
// name here carries one, and the alternative is a quoting parser for a value
// that is one word.
func stripComment(s string) string {
	if cut := strings.IndexByte(s, '#'); cut >= 0 {
		return s[:cut]
	}
	return s
}

func unquote(s string) string {
	for _, q := range []string{`"`, `'`} {
		if len(s) >= 2 && strings.HasPrefix(s, q) && strings.HasSuffix(s, q) {
			return s[1 : len(s)-1]
		}
	}
	return s
}
