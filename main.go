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
	"flowfin.dev/hub/internal/harness"
	"flowfin.dev/hub/internal/releases"
	"flowfin.dev/hub/internal/sources"
	"flowfin.dev/hub/internal/sweep"
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
	case "harness":
		return runHarness(args[1:], out)
	case "sources":
		return runSources(out)
	case "sweep":
		return runSweep(args[1:], out)
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

// runSweep says which scheduled runs ended in something other than success, and
// with the word `raise` opens a tracking issue for each one nothing is holding.
//
// It reaches the network and the tracker, so it is a verb somebody or a
// schedule runs rather than a leg: decisions/headless-and-unelevated.md keeps a
// merge from depending on a service being up, and no merge should depend on the
// tracker at all. Reporting and raising are two words for the same reason the
// harness has two: finding out what this does must not be done by doing it.
func runSweep(args []string, out io.Writer) error {
	raising := false
	switch {
	case len(args) == 1 && args[0] == "raise":
		raising = true
	case len(args) > 0:
		return fmt.Errorf("the only word after sweep is `raise`, and %q was given", strings.Join(args, " "))
	}

	repository := os.Getenv("GITHUB_REPOSITORY")
	if repository == "" {
		// The name is not written into this tree. no-hardcoded-names refuses an
		// account name in source, and a sweep pointed at a second repository
		// should be a variable rather than an edit.
		return fmt.Errorf("GITHUB_REPOSITORY names the repository to sweep, and it is not set")
	}

	client := sweep.New()
	client.Repository = repository
	client.Token = os.Getenv("GITHUB_TOKEN")

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	var raiser sweep.Raiser
	if raising {
		if client.Token == "" {
			return fmt.Errorf("raising an issue needs GITHUB_TOKEN, and it is not set")
		}
		raiser = client
	}
	return sweep.Sweep(context.Background(), out, os.DirFS(wd), client, raiser)
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

	_, gateErr := gate.Run(out, legs, asked, gate.Shell(out, wd))

	// The disclosure the gate's verdict is incomplete without. It is printed
	// whether the gate passed or failed, and after the report rather than
	// before it, because the reading it exists against is a green gate taken as
	// a green everything. decisions/headless-and-unelevated.md is where that is
	// argued.
	if err := discloseTheHarness(out, wd); err != nil {
		return err
	}
	return gateErr
}

func discloseTheHarness(out io.Writer, wd string) error {
	tagged, err := harness.TaggedFiles(wd)
	if err != nil {
		return fmt.Errorf("reading the harness tags: %w", err)
	}
	harness.Disclosure(out, harness.Requirements(), tagged)
	return nil
}

// runHarness reports what the harness holds, or runs one requirement.
//
// It is a verb somebody types rather than a leg, and that is the whole point:
// every requirement needs something a clean runner does not have, so a merge
// that waited on one would be a merge waiting on somebody else's service, a
// browser install or a running server.
func runHarness(args []string, out io.Writer) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	tagged, err := harness.TaggedFiles(wd)
	if err != nil {
		return fmt.Errorf("reading the harness tags: %w", err)
	}

	requirements := harness.Requirements()
	if len(args) == 0 {
		harness.Report(out, requirements, tagged)
		return nil
	}
	if len(args) > 1 {
		return fmt.Errorf("one requirement at a time, and %d were given: %s",
			len(args), strings.Join(args, ", "))
	}

	r, ok := harness.Lookup(requirements, args[0])
	if !ok {
		return fmt.Errorf("no such requirement: %s (the requirements are %s)",
			args[0], strings.Join(harness.Names(requirements), ", "))
	}
	return harness.Ask(out, wd, r, tagged)
}

func usage(out io.Writer) {
	fmt.Fprintf(out, "usage: go run . gate [leg...]\n       go run . harness [requirement]\n       go run . sources\n       go run . sweep [raise]\n\nthe legs, in order: %s\nthe harness requirements, which are never legs: %s\n",
		strings.Join(gate.Names(gate.Legs()), ", "),
		strings.Join(harness.Names(harness.Requirements()), ", "))
}
