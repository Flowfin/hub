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
	"fmt"
	"io"
	"os"
	"strings"

	"flowfin.dev/hub/internal/gate"
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
	default:
		usage(out)
		return fmt.Errorf("unknown verb %q", args[0])
	}
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
	fmt.Fprintf(out, "usage: go run . gate [leg...]\n\nthe legs, in order: %s\n",
		strings.Join(gate.Names(gate.Legs()), ", "))
}
