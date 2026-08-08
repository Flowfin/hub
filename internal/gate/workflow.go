package gate

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// WorkflowPath is the gate's workflow file, relative to the repository root.
const WorkflowPath = ".github/workflows/gate.yml"

// Job is one job a workflow file declares: the key under jobs:, and the name it
// reports under.
type Job struct {
	ID   string
	Name string
}

// JobsIn reads the jobs out of a workflow file.
//
// It is a line reader over a two-space-indented block rather than a YAML
// parser, and that is a deliberate limit rather than an oversight. A parser
// would be the first dependency in a tree whose empty dependency set is a
// property CONTRIBUTING.md spends a section on, for one file this repository
// writes itself. The cost is that the reader understands the shape of that one
// file and nothing wider, so it refuses what it cannot read instead of guessing:
// a job with no explicit name is an error, because such a job reports under its
// key and the mapping this reader exists to check becomes invisible.
func JobsIn(r io.Reader) ([]Job, error) {
	var (
		jobs    []Job
		inJobs  bool
		current = -1
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

		// A key at column zero ends the jobs block and starts another.
		if !strings.HasPrefix(text, " ") {
			inJobs = trimmed == "jobs:"
			continue
		}
		if !inJobs {
			continue
		}

		switch {
		case indent(text) == 2 && strings.HasSuffix(trimmed, ":"):
			id := strings.TrimSuffix(trimmed, ":")
			jobs = append(jobs, Job{ID: id})
			current = len(jobs) - 1
		case indent(text) == 4 && strings.HasPrefix(trimmed, "name:"):
			if current < 0 {
				return nil, fmt.Errorf("line %d: a name outside any job", line)
			}
			if jobs[current].Name != "" {
				return nil, fmt.Errorf("line %d: job %q names itself twice", line, jobs[current].ID)
			}
			jobs[current].Name = unquote(strings.TrimSpace(strings.TrimPrefix(trimmed, "name:")))
		}
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}

	for _, j := range jobs {
		if j.Name == "" {
			return nil, fmt.Errorf("job %q declares no name, so its check run would be called %q", j.ID, j.ID)
		}
	}
	return jobs, nil
}

func indent(s string) int {
	return len(s) - len(strings.TrimLeft(s, " "))
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}
