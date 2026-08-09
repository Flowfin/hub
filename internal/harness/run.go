package harness

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Argv is the command that runs one requirement's checks.
//
// It is data rather than a closure for the same reason a gate leg's is: the
// report can print what would run without running it, and the workflow job and
// the shell cannot drift apart because neither of them holds the command.
func Argv(r Requirement) []string {
	return []string{"go", "test", "-tags", r.Tag(), "./...", "-count=1", "-v"}
}

// Report writes what the harness holds, without running anything.
func Report(w io.Writer, rs []Requirement, tagged map[string][]string) {
	fmt.Fprintf(w, "the harness, %d requirement(s). None of them runs in the gate.\n", len(rs))
	for _, r := range rs {
		files := tagged[r.Tag()]
		fmt.Fprintf(w, "\n%s\n", r.Name)
		fmt.Fprintf(w, "  tag       %s\n", r.Tag())
		fmt.Fprintf(w, "  reaches   %s\n", r.Reaches)
		fmt.Fprintf(w, "  costs     %s\n", r.Costs)
		fmt.Fprintf(w, "  runs      %s\n", strings.Join(Argv(r), " "))
		if len(files) == 0 {
			fmt.Fprintf(w, "  carried by nothing yet\n")
			continue
		}
		fmt.Fprintf(w, "  carried by %s\n", strings.Join(files, ", "))
	}
}

// Ask runs one requirement's checks in dir.
//
// A requirement nothing carries is refused rather than run. `go test` over such
// a tag exits zero having compiled nothing, and a green job that ran no check is
// the absent result read as a clean one, which is the failure the harness exists
// against.
func Ask(out io.Writer, dir string, r Requirement, tagged map[string][]string) error {
	files := tagged[r.Tag()]
	if len(files) == 0 {
		return fmt.Errorf("no test file carries %s, so there is nothing to run and a green here would say nothing", r.Tag())
	}

	fmt.Fprintf(out, "harness: %s: %s\n", r.Name, r.Reaches)
	fmt.Fprintf(out, "harness: %s: asking costs %s\n", r.Name, r.Costs)
	fmt.Fprintf(out, "harness: %s: %d file(s): %s\n", r.Name, len(files), strings.Join(files, ", "))
	fmt.Fprintf(out, "harness: %s: %s\n", r.Name, strings.Join(Argv(r), " "))

	argv := Argv(r)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s: %w", r.Name, strings.Join(argv, " "), err)
	}
	return nil
}
