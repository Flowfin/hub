// Package carry proposes the bytes a publication run built to the branch the
// site is served from.
//
// internal/publish places the catalogue in the checkout the run was started
// from, and that is where it stopped. The site is served from the docs
// directory of the default branch, so a run whose checkout goes with the runner
// changes nothing an operator can fetch, however green it was. That gap was
// measured rather than supposed: the publication run was green on every
// dispatch while the freshness watch reddened twice on the exact version the
// run had just built and discarded.
//
// What closes it is a proposal and not a write to the served branch.
// decisions/release-procedure.md keeps a branch, a pull request, the gate and a
// merge in front of the docs directory on purpose, and nothing here shortens
// that. The run opens the request; whether it is merged stays a person's step,
// and the ruleset in front of the default branch is untouched.
//
// Three properties are worth reading before the code.
//
// It decides against the served branch rather than against the checkout. The
// question is whether what the declarations come to differs from what an
// operator would fetch, and the checkout is the nearest thing to hand rather
// than the thing itself: a run dispatched on a stale checkout would otherwise
// propose a catalogue against a file it never read.
//
// It never rewrites the standing branch. Every write below either creates that
// branch or commits on top of it, so a history somebody may already have
// fetched is never replaced, and the request reads as the sequence of
// catalogues it carried instead of one commit changing underneath whoever is
// reading it. Converging on one open request is what the standing branch is
// for, and a force-push was only ever one way of getting there.
//
// Nothing here reaches the network. The repository arrives through the two
// interfaces below, which is what lets every decision in this file be judged
// against a fixture rather than against somebody else's service:
// decisions/headless-and-unelevated.md is why that matters. github.go is where
// the reaching is, behind those interfaces.
package carry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
)

// ArguedIn is the issue this mechanism was argued in.
//
// It is carried because the request this package opens is judged by the same
// gate as every other one, and internal/hygiene refuses a request whose title
// and body carry no issue reference at all. A request nothing connects to a
// reason is what that leg exists against, and one opened by a run is no more
// exempt from it than one opened by hand.
const ArguedIn = 154

// ErrNoBranch is what a read of a branch the repository does not hold answers
// with.
//
// It is a state rather than a failed read. The standing branch does not exist
// before the first carry, and does not exist again after a merge that deleted
// it, and both are ordinary; treating either as an error would make the first
// run of this mechanism, and the run after every tidy merge, a red one.
var ErrNoBranch = errors.New("the repository holds no such branch")

// Repository is what a carry reads. Client satisfies it, and so does a fixture,
// which is what lets the whole decision be taken without leaving the runner.
type Repository interface {
	// DefaultBranch is the branch the site is served from.
	//
	// It is read rather than assumed for the reason internal/sweep gives for
	// reading it: a mechanism carrying the name of the default branch goes
	// silent on the day it is renamed, and going silent is the failure this
	// package exists against.
	DefaultBranch(ctx context.Context) (string, error)

	// Head is the commit a branch points at, or ErrNoBranch.
	Head(ctx context.Context, branch string) (string, error)

	// File is the content of one path on one branch and the blob that branch
	// holds it under, or nil and an empty string where the branch carries no
	// such file.
	File(ctx context.Context, branch, path string) (content []byte, blob string, err error)

	// OpenPull is the number of the open request from head, or zero where
	// there is none.
	OpenPull(ctx context.Context, head string) (int, error)
}

// Proposer writes.
//
// It is a second interface rather than three more methods on Repository for the
// reason internal/sweep splits Raiser off: the half of a run that only reads
// must not be able to write by accident, so it is handed nothing to call rather
// than trusted not to call it.
type Proposer interface {
	// Branch creates a branch at an existing commit.
	Branch(ctx context.Context, name, at string) error

	// Commit places content at path on branch, replacing the blob named by
	// replacing, or creating the file where that is empty. It returns the
	// commit it made.
	Commit(ctx context.Context, branch, path, message string, content []byte, replacing string) (string, error)

	// Pull opens a request from head against base and returns its number.
	Pull(ctx context.Context, head, base, title, body string) (int, error)
}

// Change is one file a run wants carried.
type Change struct {
	// Path is where the file lives in the repository, slash separated.
	Path string

	// Branch is the standing branch the change is proposed on.
	Branch string

	// Bytes are what the run placed at Path in its own checkout.
	Bytes []byte
}

// Carry proposes c against the branch the site is served from, and says what it
// did.
//
// on is the branch this run is running on. A run somewhere else is reported and
// carries nothing: what it compared its bytes against is that branch's file
// rather than the served one, so a request opened from it would carry a
// difference nobody asked about. That is a sentence in the output rather than a
// refusal, because dispatching the run on a branch to see what it builds is a
// legitimate thing to do, and reddening it would teach whoever did it to stop
// reading the verdict.
func Carry(ctx context.Context, out io.Writer, on string, c Change, repo Repository, prop Proposer) error {
	base, err := repo.DefaultBranch(ctx)
	if err != nil {
		return err
	}
	if on != base {
		fmt.Fprintf(out, "\nnot carried: this run is on %s and %s is what the site is served from, so what it compared %s against is not the published file.\n",
			on, base, c.Path)
		return nil
	}

	served, _, err := repo.File(ctx, base, c.Path)
	if err != nil {
		return fmt.Errorf("reading what %s carries at %s: %w", base, c.Path, err)
	}
	if bytes.Equal(served, c.Bytes) {
		fmt.Fprintf(out, "\nnothing to carry: %s already carries these bytes at %s, and no pull request was opened.\n", base, c.Path)
		return nil
	}

	if err := standing(ctx, c.Branch, base, repo, prop); err != nil {
		return err
	}

	// Read from the standing branch rather than from the served one. The blob a
	// commit replaces has to be the one that branch holds, and taking the
	// served branch's blob would fail every carry after the first, on a branch
	// that has moved since.
	current, blob, err := repo.File(ctx, c.Branch, c.Path)
	if err != nil {
		return fmt.Errorf("reading what %s carries at %s: %w", c.Branch, c.Path, err)
	}
	if bytes.Equal(current, c.Bytes) {
		// The standing branch already proposes exactly these bytes, which is
		// every run between a catalogue changing and its request being merged.
		// Committing them again would add a commit that changes nothing to a
		// request somebody is in the middle of reading.
		fmt.Fprintf(out, "\n%s already carries these bytes at %s, so nothing was committed.\n", c.Branch, c.Path)
	} else {
		commit, err := prop.Commit(ctx, c.Branch, c.Path, Message(c.Path), c.Bytes, blob)
		if err != nil {
			return fmt.Errorf("committing %s to %s: %w", c.Path, c.Branch, err)
		}
		fmt.Fprintf(out, "\n%s: %d byte(s) committed to %s as %s.\n", c.Path, len(c.Bytes), c.Branch, commit)
	}

	number, err := repo.OpenPull(ctx, c.Branch)
	if err != nil {
		return fmt.Errorf("reading the open request from %s: %w", c.Branch, err)
	}
	if number != 0 {
		fmt.Fprintf(out, "pull request #%d is the one already open from %s, and it carries the difference.\n", number, c.Branch)
		return nil
	}

	number, err = prop.Pull(ctx, c.Branch, base, Title, Body(c.Path, base))
	if err != nil {
		return fmt.Errorf("opening a request from %s against %s: %w", c.Branch, base, err)
	}
	fmt.Fprintf(out, "pull request #%d is open against %s, carrying %s.\n", number, base, c.Path)
	return nil
}

// standing makes sure the branch the change is proposed on exists, creating it
// at the served branch's head where it does not.
//
// A branch that exists is left exactly where it is. Moving it to the served
// head would be a rewrite of a history a reviewer may already have fetched,
// which this package does not do at all, and the difference the request carries
// against the served branch is the same either way.
func standing(ctx context.Context, branch, base string, repo Repository, prop Proposer) error {
	_, err := repo.Head(ctx, branch)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrNoBranch) {
		return fmt.Errorf("reading %s: %w", branch, err)
	}

	at, err := repo.Head(ctx, base)
	if err != nil {
		return fmt.Errorf("reading %s: %w", base, err)
	}
	if err := prop.Branch(ctx, branch, at); err != nil {
		return fmt.Errorf("creating %s at %s: %w", branch, at, err)
	}
	return nil
}

// Title is what the opened request is called.
//
// It says what the request carries rather than what produced it, and it is one
// string rather than one per run, because a title that moved would make the
// same standing request read as a different one every night.
const Title = "Carry the catalogue the publication run built"

// Message is the commit message the carried bytes land under.
func Message(path string) string {
	return fmt.Sprintf("Carry the generated %s to the branch the site is served from", path)
}

// Body is what the opened request says.
//
// Three things, and each of them is there because a reader of the request has
// no other way to get it: what the change is and why the run that made it does
// not merge it, the issue the mechanism was argued in, which internal/hygiene
// refuses a request without, and the one trap.
//
// The trap is a property of the platform rather than of this repository. A
// request opened with a run's own token creates no workflow run, so the jobs the
// ruleset requires never start, and the request sits carrying no verdict at all
// rather than a red one. Whoever merges it has to start them, and the last
// paragraph is where they are told so.
func Body(path, base string) string {
	return fmt.Sprintf(`The publication run built %[1]s from the declarations, and what %[2]s carries at that path is different, so this request carries the difference. It closes nothing. The run proposes and merges nothing itself, and the merge stays a person's step in front of the served directory, which is what decisions/release-procedure.md asks for.

The mechanism that opened this was argued in #%[3]d.

The checks on this request have not started, and they will not start on their own. A request opened with a run's own token creates no workflow run, so the jobs the ruleset requires are absent rather than red. Closing this request and reopening it starts them. Read the verdict before merging.`, path, base, ArguedIn)
}
