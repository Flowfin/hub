package sweep

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
)

// Label is what a raised issue carries so that it lands where the other
// automation issues on this board land.
const Label = "ci"

// Source is what the sweep reads the world through. Client satisfies it, and
// so does a fixture, which is what lets the whole sweep be decided without
// leaving the runner.
type Source interface {
	DefaultBranch(ctx context.Context) (string, error)
	Runs(ctx context.Context, workflow string) ([]Run, error)
	OpenIssues(ctx context.Context) ([]Issue, error)
}

// Raiser writes. It is a second interface rather than a method on Source
// because the reporting run must not be able to write by accident: it is handed
// a nil Raiser, and there is then nothing there to call.
type Raiser interface {
	Raise(ctx context.Context, f Failure, label string) (int, error)
}

// Sweep derives the watched set, reads the recent runs, and reports.
//
// With a nil Raiser it writes nothing and says what it would have raised. That
// is the shape the harness uses for the same reason: a verb that reports and a
// verb that acts are different words, so a person finding out what this does
// cannot find out by doing it.
func Sweep(ctx context.Context, out io.Writer, fsys fs.FS, src Source, raiser Raiser) error {
	watched, err := Scheduled(fsys)
	if err != nil {
		return err
	}
	unwatched, err := unscheduled(fsys, watched)
	if err != nil {
		return err
	}

	branch, err := src.DefaultBranch(ctx)
	if err != nil {
		return err
	}

	var runs []Run
	var never []string
	for _, w := range watched {
		got, err := src.Runs(ctx, w)
		if errors.Is(err, ErrNoHistory) {
			never = append(never, w)
			continue
		}
		if err != nil {
			return fmt.Errorf("reading the runs of %s: %w", w, err)
		}
		runs = append(runs, got...)
	}

	open, err := src.OpenIssues(ctx)
	if err != nil {
		return err
	}

	fresh, held := Unreported(Select(watched, runs, branch), open)

	raised := map[string]int{}
	var raiseErr error
	if raiser != nil {
		for _, f := range fresh {
			number, err := raiser.Raise(ctx, f, Label)
			if err != nil {
				// The remaining failures are still raised. One tracker error
				// must not swallow the report of every other failing workflow,
				// which is the same asymmetry decisions/failure-posture.md
				// draws for a release nobody can repair.
				if raiseErr == nil {
					raiseErr = err
				}
				continue
			}
			raised[f.Key()] = number
		}
	}

	fmt.Fprint(out, Report(watched, unwatched, never, fresh, held, raised))
	if raiser == nil && len(fresh) > 0 {
		fmt.Fprintln(out, "this run wrote nothing; `go run . sweep raise` is the one that does.")
	}
	return raiseErr
}

// unscheduled is every workflow file the derivation read and left out, so the
// report can say what is outside the watch rather than only what is inside it.
func unscheduled(fsys fs.FS, watched []string) ([]string, error) {
	entries, err := fs.ReadDir(fsys, WorkflowDir)
	if err != nil {
		return nil, err
	}
	inSet := map[string]bool{}
	for _, w := range watched {
		inSet[w] = true
	}

	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if ext := path.Ext(e.Name()); ext != ".yml" && ext != ".yaml" {
			continue
		}
		full := WorkflowDir + "/" + e.Name()
		if !inSet[full] {
			out = append(out, full)
		}
	}
	return out, nil
}
