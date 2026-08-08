// Command hub is this repository's single entry point.
//
// It exists so that the gate is one procedure rather than two. A contributor
// runs the whole thing before pushing:
//
//	go run . gate
//
// and each job in .github/workflows/gate.yml runs exactly one leg of the same
// thing, under the leg's own name:
//
//	go run . gate format
//
// so a red check says which leg refused, and nothing in the workflow file
// decides anything the command would not decide locally.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"flowfin.dev/hub/internal/gate"
	"flowfin.dev/hub/internal/releases"
	"flowfin.dev/hub/internal/sources"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		usage(out)
		return fmt.Errorf("no verb given")
	}

	switch args[0] {
	case "gate":
		return runGate(args[1:], out)
	case "sources":
		return runSources(out)
	default:
		usage(out)
		return fmt.Errorf("unknown verb %q", args[0])
	}
}

// runSources reads the declared set and says what each declaration resolved to.
//
// It reaches the network, which is why it is a verb somebody runs rather than a
// leg of the gate: decisions/headless-and-unelevated.md keeps a merge from
// depending on somebody else's service being up. Every classification it prints
// is judged against a fixture in internal/sources, so what this verb adds is the
// answer about the world rather than the logic that reads it.
func runSources(out io.Writer) error {
	declarations, err := sources.Load(os.DirFS(sources.Dir))
	if err != nil {
		return err
	}

	client := releases.New()
	// The token is read from the environment rather than declared anywhere. A
	// run without one still works against public repositories and meets the rate
	// limit sooner, which is a read that failed rather than an empty catalogue.
	client.Token = os.Getenv("GITHUB_TOKEN")

	resolutions := sources.Resolve(context.Background(), client, declarations)
	fmt.Fprint(out, sources.Report(resolutions))
	return sources.Judge(resolutions)
}

func runGate(asked []string, out io.Writer) error {
	legs := gate.Legs()
	if unknown := gate.Unknown(legs, asked); len(unknown) > 0 {
		return fmt.Errorf("no such leg: %s (the legs are %s)",
			strings.Join(unknown, ", "), strings.Join(gate.Names(legs), ", "))
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	_, err = gate.Run(out, legs, asked, gate.Shell(out, wd))
	return err
}

func usage(out io.Writer) {
	fmt.Fprintf(out, "usage: go run . gate [leg...]\n       go run . sources\n\nthe legs, in order: %s\n",
		strings.Join(gate.Names(gate.Legs()), ", "))
}
