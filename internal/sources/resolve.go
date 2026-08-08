package sources

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Release is the part of a release this layer reads. Everything else about a
// release belongs to the layers that pair archives and build entries.
type Release struct {
	Tag        string
	Prerelease bool
}

// ErrNotFound is what a lister returns for a repository the credential cannot
// see, whichever of the two reasons applies.
var ErrNotFound = errors.New("not found")

// Lister answers what a declared repository has published.
//
// It returns the path the request actually landed on, because that is the only
// layer where the difference is still visible. GitHub follows a rename, so a
// declaration naming a moved path answers, returns releases, and looks in every
// way like a correct declaration; by the time a caller has a release list, the
// name that was asked for is gone.
//
// It is an interface so the gate can judge every classification below against a
// fixture. decisions/headless-and-unelevated.md refuses a gate test that reaches
// the network, and the real implementation is the only thing that does.
type Lister interface {
	ListReleases(ctx context.Context, account, repository string) (releases []Release, landedOn string, err error)
}

// State is what a run learned about one declaration.
type State int

const (
	// Resolved: the repository answered and has at least one finished release.
	Resolved State = iota
	// Disabled: the record says not on this run.
	Disabled
	// NoReleases: the repository answered and has published nothing. The
	// ordinary case here rather than an edge, and not an error.
	NoReleases
	// NoFinishedReleases: it has published, and every tag is on the other side
	// of the channel split. Also not an error, and a different sentence from
	// having published nothing.
	NoFinishedReleases
	// Unresolvable: a not-found. Fatal.
	Unresolvable
	// Redirected: it answered under a path other than the declared one. Fatal.
	Redirected
	// Unreadable: a transport or credential failure. Fatal.
	Unreadable
)

func (s State) String() string {
	switch s {
	case Resolved:
		return "resolved"
	case Disabled:
		return "disabled"
	case NoReleases:
		return "no releases"
	case NoFinishedReleases:
		return "no finished releases"
	case Unresolvable:
		return "does not resolve"
	case Redirected:
		return "resolves under another name"
	case Unreadable:
		return "could not be read"
	}
	return "unknown"
}

// Fatal reports whether this state stops the run.
//
// decisions/failure-posture.md draws the line and this is the whole of it at
// this layer: a declaration pointing at nothing, or a read that failed, makes
// every conclusion about what exists unfounded. A repository with nothing to
// publish is a true answer about the world and is reported rather than refused.
//
// Redirected is fatal and the decision file does not name it, so the reason is
// here. A declaration that only answers through a redirect is not a failure
// today and becomes one with no error at all: the day the redirect stops being
// served, or the day somebody creates a repository at the freed name, the run
// reads a different repository than the one that was declared. The repair is one
// field, and it is only available at this layer.
func (s State) Fatal() bool {
	return s == Unresolvable || s == Redirected || s == Unreadable
}

// Resolution is one declaration and what became of it.
type Resolution struct {
	Declaration Declaration
	State       State
	// Finished and Test are the two sides of the channel split, counted rather
	// than carried, because a pattern that quietly emptied a channel is
	// otherwise indistinguishable from a plugin that stopped releasing.
	Finished int
	Test     int
	// Detail says what happened in words a person can act on.
	Detail string
	// Releases are the finished ones, for the layers that build entries.
	Releases []Release
}

func (r Resolution) String() string {
	line := fmt.Sprintf("%-20s %-24s %s", r.Declaration.Slug, r.State, r.Detail)
	return strings.TrimRight(line, " ")
}

// Resolve reads every declaration through lister and classifies each one.
//
// It does not stop at the first fatal state. A run that abandoned the set at the
// first bad declaration would report one problem per run, and somebody fixing
// three of them would need three runs to find out.
func Resolve(ctx context.Context, lister Lister, declarations []Declaration) []Resolution {
	out := make([]Resolution, 0, len(declarations))
	for _, d := range declarations {
		out = append(out, resolveOne(ctx, lister, d))
	}
	return out
}

func resolveOne(ctx context.Context, lister Lister, d Declaration) Resolution {
	r := Resolution{Declaration: d}

	if !d.On() {
		r.State, r.Detail = Disabled, "not read on this run"
		return r
	}

	releases, landedOn, err := lister.ListReleases(ctx, d.Account, d.Repository)
	switch {
	case errors.Is(err, ErrNotFound):
		r.State = Unresolvable
		// The two reasons are not separable. A repository that was never created
		// and one that exists but is invisible to the credential both answer with
		// a not-found, and nothing in the response tells them apart, so the
		// message names both rather than claiming to have chosen.
		r.Detail = fmt.Sprintf("%s answered not-found: it does not exist, or the credential cannot see it", d.Path())
		return r
	case err != nil:
		r.State = Unreadable
		r.Detail = fmt.Sprintf("%s could not be read: %v", d.Path(), err)
		return r
	}

	if landedOn != "" && !strings.EqualFold(landedOn, d.Path()) {
		r.State = Redirected
		r.Detail = fmt.Sprintf("declared %s, answered as %s; declare the path that answered", d.Path(), landedOn)
		return r
	}

	if len(releases) == 0 {
		r.State, r.Detail = NoReleases, fmt.Sprintf("%s has published nothing", d.Path())
		return r
	}

	for _, rel := range releases {
		if d.IsFinished(rel.Tag) {
			r.Finished++
			r.Releases = append(r.Releases, rel)
			continue
		}
		r.Test++
	}

	if r.Finished == 0 {
		r.State = NoFinishedReleases
		r.Detail = fmt.Sprintf("%d release(s), none matching stable_tags", r.Test)
		return r
	}

	r.State = Resolved
	r.Detail = fmt.Sprintf("%d finished, %d test", r.Finished, r.Test)
	return r
}

// Report writes one line per declaration, in slug order, and ends with the
// counts.
//
// Every declaration appears, including the ones that contributed nothing.
// decisions/failure-posture.md: a manifest that is short because releases were
// skipped and a manifest that is short because there was nothing to add are the
// same file, and only the run output tells them apart. How the release-level
// skips are shaped is #28; this is the repository level.
func Report(resolutions []Resolution) string {
	sorted := append([]Resolution(nil), resolutions...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Declaration.Slug < sorted[j].Declaration.Slug
	})

	var b strings.Builder
	counts := map[State]int{}
	resolved := 0
	for _, r := range sorted {
		fmt.Fprintf(&b, "  %s\n", r)
		counts[r.State]++
		if r.State == Resolved {
			resolved++
		}
	}

	fmt.Fprintf(&b, "\n%d of %d declared plugin(s) resolved with something to publish.\n",
		resolved, len(sorted))
	for _, s := range []State{Disabled, NoReleases, NoFinishedReleases, Unresolvable, Redirected, Unreadable} {
		if n := counts[s]; n > 0 {
			fmt.Fprintf(&b, "  %d %s\n", n, s)
		}
	}
	return b.String()
}

// Judge turns the resolutions into the run's verdict.
//
// Two things are fatal here and both come from decisions/failure-posture.md. Any
// declaration in a fatal state, and a run that resolved zero plugins whatever the
// reason. The second is the one the decision spends its longest section on: a run
// that resolved nothing because a credential expired produces exactly the file a
// correct run over an empty set would produce, so publishing it would replace a
// working catalogue with an empty one and report success.
func Judge(resolutions []Resolution) error {
	var fatal []string
	resolved := 0
	for _, r := range resolutions {
		if r.State.Fatal() {
			fatal = append(fatal, fmt.Sprintf("%s: %s", r.Declaration.Slug, r.Detail))
		}
		if r.State == Resolved {
			resolved++
		}
	}

	if len(fatal) > 0 {
		return fmt.Errorf("the declared source set did not resolve:\n  %s", strings.Join(fatal, "\n  "))
	}
	if resolved == 0 {
		return errors.New("no declared plugin resolved with anything to publish; " +
			"publishing an empty catalogue would replace a working one and report success, " +
			"and an empty catalogue is a decision somebody makes by disabling every declaration deliberately")
	}
	return nil
}
