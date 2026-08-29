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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"flowfin.dev/hub/internal/address"
	"flowfin.dev/hub/internal/carry"
	"flowfin.dev/hub/internal/catalogue"
	"flowfin.dev/hub/internal/gate"
	"flowfin.dev/hub/internal/harness"
	"flowfin.dev/hub/internal/publish"
	"flowfin.dev/hub/internal/readiness"
	"flowfin.dev/hub/internal/releases"
	"flowfin.dev/hub/internal/scan"
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
	case "publish":
		return runPublish(args[1:], out)
	case "release":
		return runRelease(args[1:], out)
	case "freshness":
		return runFreshness(args[1:], out)
	case "scan":
		return runScan(args[1:], out)
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

// runPublish generates the catalogue and places it at the address an operator
// pasted into a server.
//
// It reaches the network, so it is a verb somebody or a schedule runs rather
// than a leg of the gate, for the reason runSources gives above. What it adds
// beside that verb is the rest of the route: internal/catalogue is the order the
// parts run in, and every refusal in it is judged against a fixture there.
//
// The location is publish.Stable and is named here rather than configured
// anywhere else. Moving what the address is served from is an edit to that one
// value, which is the separation #31 asks for: the address is permanent and the
// storage behind it is not.
//
// The word after the verb decides whether what was placed is proposed anywhere.
// Placing writes into the checkout the run was started from, which on a runner
// goes with the runner, so a run without the word changes nothing an operator
// can fetch and says so. Reporting and acting are two words for the reason the
// sweep gives for the same split: finding out what this does must not be done
// by doing it.
func runPublish(args []string, out io.Writer) error {
	carrying := false
	switch {
	case len(args) == 1 && args[0] == "carry":
		carrying = true
	case len(args) > 0:
		return fmt.Errorf("the only word after publish is `carry`, and %q was given", strings.Join(args, " "))
	}

	declarations, err := sources.Load(os.DirFS(sources.Dir))
	if err != nil {
		return err
	}

	// The root is the working directory rather than a flag, so a run publishes
	// into the checkout it was started from and cannot be pointed at a served
	// directory belonging to something else by a typo.
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	client := releases.New()
	client.Token = os.Getenv("GITHUB_TOKEN")

	ctx := context.Background()
	route := catalogue.Route{
		Root:   wd,
		Target: publish.Stable,
		Lister: client,
		// Memo, because the descriptor of a release is read once to build its
		// version entry and again to choose which release names the plugin, and
		// the rate limit those two spend is shared with everything else using
		// the token.
		Fetch: catalogue.Memo(client.Fetching(ctx)),
	}
	if err := route.Publish(ctx, out, declarations); err != nil {
		return err
	}
	if !carrying {
		fmt.Fprintf(out, "this run placed %s in its own checkout and nowhere else; `go run . publish carry` is the one that proposes it.\n", publish.Stable.Name)
		return nil
	}
	return carryPlaced(ctx, out, wd)
}

// carryPlaced proposes what the run just placed to the branch the site is
// served from.
//
// The bytes are read back off the placed file rather than kept from the
// producing writer, and that is the one reading in this verb that is worth
// arguing for: what is proposed has to be what the target now holds, so the
// target is what it is read from. internal/publish is where the bytes on their
// way in are compared against what was there before, and that comparison is a
// different question from this one.
//
// Everything else here is read from the run's environment rather than written
// into this tree. no-hardcoded-names refuses the repository name in source, and
// the branch a run is on is a fact about the run.
func carryPlaced(ctx context.Context, out io.Writer, root string) error {
	repository := os.Getenv("GITHUB_REPOSITORY")
	if repository == "" {
		return fmt.Errorf("GITHUB_REPOSITORY names the repository to carry into, and it is not set")
	}
	on := os.Getenv("GITHUB_REF_NAME")
	if on == "" {
		return fmt.Errorf("GITHUB_REF_NAME names the branch this run is on, and it is not set")
	}
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("carrying the catalogue needs GITHUB_TOKEN, and it is not set")
	}

	placed, err := os.ReadFile(publish.Stable.Path(root))
	if err != nil {
		return fmt.Errorf("reading back the placed %s: %w", publish.Stable.Name, err)
	}

	client := carry.New()
	client.Repository = repository
	client.Token = token

	change := carry.Change{
		Path:   path.Join(publish.Stable.Dir, publish.Stable.Name),
		Branch: publish.Stable.Branch(),
		Bytes:  placed,
	}
	return carry.Carry(ctx, out, on, change, client, client)
}

// runRelease says whether this repository is in a state a release may be cut
// from, and exits non-zero while it is not.
//
// It is the step in front of the procedure in decisions/release-procedure.md
// rather than a leg of the merge gate, and that is not a softening. Two of the
// four conditions are questions about the world - what the published address
// answers with, and what the release lists hold - so a leg deciding them would
// be a merge waiting on somebody else's service, which
// decisions/headless-and-unelevated.md refuses. The conditions themselves are
// judged in internal/readiness against planted readings, so what this verb adds
// is the readings and not the rules.
//
// What it does not do is tag, build or publish anything. A verb that refused a
// release and then performed one on the next line would be two things, and the
// half worth having is the one that can say no.
func runRelease(args []string, out io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("release takes no further words, and %q was given", strings.Join(args, " "))
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	declarations, err := sources.Load(os.DirFS(sources.Dir))
	if err != nil {
		return err
	}

	client := releases.New()
	client.Token = os.Getenv("GITHUB_TOKEN")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	resolutions := sources.Resolve(ctx, client, declarations)
	fmt.Fprint(out, sources.Report(resolutions))
	fmt.Fprintln(out)

	conditions := readiness.Conditions(wd, address.Answered, resolutions,
		func(addr string) ([]byte, error) { return fetch(ctx, addr) })

	readiness.Report(out, conditions)
	return readiness.Judge(conditions)
}

// runFreshness says whether the published catalogue still carries the newest
// finished release of every declared plugin, and exits non-zero while it does
// not.
//
// It is the one condition of decisions/release-procedure.md that starts holding
// on its own. The other three are states of this tree and hold still until
// somebody changes them; this one arrives the moment a plugin releases, with
// nothing here having moved, and the symptom is a version standing still on
// somebody else's server. That is why it is asked on a schedule rather than only
// when a person types the verb above: deciding that a publication run's verdict
// calls for a merge was the step that stayed a person's, and for two days in
// August nobody took it.
//
// Nothing new is judged here. readiness.CatalogueConditions is the release
// verb's own assembly of that condition, so a watch that reports and a release
// that refuses cannot come to different answers about one catalogue, and every
// rule the two share is tripped against a planted reading in internal/readiness.
//
// What it refuses is wider than a stale catalogue, and deliberately so. An
// address that could not be read and a release list that would not resolve each
// leave the condition unevaluated, which refuses in its own words rather than
// passing: the likeliest real failure of a check that leaves the runner is a
// blip, and reading a blip as evidence of freshness is worse than a false alarm.
// The resolution report is printed above the verdict, so the run says which of
// the two happened rather than leaving a reader to guess.
func runFreshness(args []string, out io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("freshness takes no further words, and %q was given", strings.Join(args, " "))
	}

	declarations, err := sources.Load(os.DirFS(sources.Dir))
	if err != nil {
		return err
	}

	client := releases.New()
	client.Token = os.Getenv("GITHUB_TOKEN")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	resolutions := sources.Resolve(ctx, client, declarations)
	fmt.Fprint(out, sources.Report(resolutions))
	fmt.Fprintln(out)

	conditions := readiness.CatalogueConditions(address.Answered, resolutions,
		func(addr string) ([]byte, error) { return fetch(ctx, addr) })

	// The report and the refusal below are the release verb's own words, because
	// they are the release verb's own condition. This line is what says which of
	// the four is being asked here, so a reader of an unattended run is not left
	// to infer it from the one condition that appears.
	fmt.Fprintf(out, "the freshness watch asks one of the states a release is refused for, on its own: %d address(es) recorded as answering.\n\n",
		len(address.Answered))

	readiness.Report(out, conditions)
	return readiness.Judge(conditions)
}

// fetch reads what an address answers with, the way a Jellyfin server would.
//
// A status other than 200 is an error rather than a body, because a server
// handed an error page shows an empty repository and reports nothing, and a
// check that judged the error page's bytes would be judging the wrong file.
func fetch(ctx context.Context, addr string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("it answered %s, so nothing installable is published there", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return body, nil
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

// runScan decides a code scanning run from the report it wrote.
//
// It is a verb rather than a leg of the gate, and that is the one place this
// board departs from the shape decisions/gate-parity.md gives every other
// adopted leg. A leg's contract is that a contributor runs the same command
// before pushing, and the analyser is not in the toolchain decisions/means.md
// fixes, so `go run . gate` could not honour that for it. What is here instead
// is the half that decides: the analysis writes a report on the runner and this
// reads it, so a finding blocks rather than annotating, which is the property
// the target's leg carries and the one worth keeping.
func runScan(args []string, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("scan takes the one directory the analysis wrote its report into, and %d word(s) were given", len(args))
	}

	reports, tools, found, err := scan.Judge(os.DirFS(args[0]))
	if err != nil {
		return err
	}
	return scan.Report(out, args[0], reports, tools, found)
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
	fmt.Fprintf(out, "usage: go run . gate [leg...]\n       go run . harness [requirement]\n       go run . sources\n       go run . publish [carry]\n       go run . release\n       go run . freshness\n       go run . sweep [raise]\n\nthe legs, in order: %s\nthe harness requirements, which are never legs: %s\n",
		strings.Join(gate.Names(gate.Legs()), ", "),
		strings.Join(harness.Names(harness.Requirements()), ", "))
}
