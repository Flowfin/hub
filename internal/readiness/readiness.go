// Package readiness decides whether this repository is in a state a release may
// be cut from.
//
// A procedure written down is a reminder. The conditions below are the half a
// machine decides, and each of them is a state in which a release is actively
// harmful rather than merely early: an operator who follows the instruction gets
// something worse than nothing. No terms, so nobody may package what they were
// given. An address that answers with no catalogue, so the server shows an empty
// repository and reports no error. A published catalogue that is not current, so
// the plugins an operator already has stop moving and the symptom is a version
// standing still. And a run with nothing to publish, which produces exactly the
// file a correct run over an empty world would produce.
//
// decisions/release-procedure.md is the procedure and names this as the step in
// front of it.
//
// Nothing here reaches the network, which is what lets the suite trip every
// condition against a planted reading rather than against somebody else's
// service. The two conditions that need the world are handed what was read:
// taking the reading is the verb's, judging it is this package's.
package readiness

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"flowfin.dev/hub/internal/freshness"
	"flowfin.dev/hub/internal/sources"
)

// Reading is what a run found out about one condition.
type Reading int

const (
	// NotEvaluated is a condition nothing was learned about, and it is the zero
	// value on purpose. A caller that builds a condition and forgets to fill it
	// in is refused rather than cleared, because the failure this package exists
	// against is a release cut on a question nobody asked.
	NotEvaluated Reading = iota

	// Blocks is a condition that was evaluated and holds.
	Blocks

	// Clear is a condition that was evaluated and does not hold.
	Clear
)

func (r Reading) String() string {
	switch r {
	case Blocks:
		return "BLOCKS"
	case Clear:
		return "clear"
	}
	return "NOT EVALUATED"
}

// Condition is one state a release may not be cut in.
type Condition struct {
	// Name is what the report prints and what a person repairing it searches
	// for.
	Name string

	// Refuses says what a release cut through this condition does to an
	// operator, in the words the report prints.
	Refuses string

	// Reading is what this run learned.
	Reading Reading

	// Detail is what was actually read, so that a refusal can be acted on
	// without running anything a second time.
	Detail string
}

// LicenceFile is where this repository's terms are, relative to the root.
const LicenceFile = "LICENSE"

// Licence reads whether the tree carries terms at all.
//
// It reads the file and not what the file says. Which licence is carried, and
// whether the published pages fall under different terms from the generator, is
// argued where licences are argued; what makes a release harmful is shipping
// with none, because a consumer who may not package what they were given finds
// out after they have it.
func Licence(root string) Condition {
	c := Condition{
		Name:    "no-licence",
		Refuses: "a release nobody may redistribute, package or fork, found out after it is public",
	}
	body, err := os.ReadFile(filepath.Join(root, LicenceFile))
	switch {
	case errors.Is(err, os.ErrNotExist):
		c.Reading = Blocks
		c.Detail = fmt.Sprintf("%s is not in the tree", LicenceFile)
	case err != nil:
		c.Reading = NotEvaluated
		c.Detail = fmt.Sprintf("%s could not be read: %v", LicenceFile, err)
	case len(bytes.TrimSpace(body)) == 0:
		c.Reading = Blocks
		c.Detail = fmt.Sprintf("%s is in the tree and carries no terms", LicenceFile)
	default:
		c.Reading = Clear
		c.Detail = fmt.Sprintf("%s carries %d byte(s) of terms", LicenceFile, len(body))
	}
	return c
}

// InstallAddress reads whether any address has been recorded as answering with
// the catalogue it promises.
//
// The recorded list is internal/address.Answered and it is the same authority
// the merge gate refuses a printed address against, so a release cannot be cut
// on an address the gate would not let anybody print. That is the point of
// reading the list rather than making a request here: a request says the address
// answered a moment ago, and the list says somebody read it and wrote down what
// they read.
func InstallAddress(recorded []string) Condition {
	c := Condition{
		Name:    "address-does-not-answer",
		Refuses: "an instruction to paste an address that answers with nothing, which a server shows as an empty repository and no error",
	}
	if len(recorded) == 0 {
		c.Reading = Blocks
		c.Detail = "no install address is recorded as answering, so there is nothing an operator could be told to paste"
		return c
	}
	c.Reading = Clear
	c.Detail = fmt.Sprintf("%d address(es) recorded as answering: %s", len(recorded), strings.Join(recorded, ", "))
	return c
}

// Catalogue judges the bytes the address answered with against what the release
// lists say should be in them.
//
// The reading is handed in rather than taken here, and both ways of not having
// one are the same answer: an address nobody has recorded and a read that failed
// are each a condition nothing is known about, which refuses. Reading a network
// failure as evidence that the catalogue is current is the shape of claim this
// tree refuses everywhere.
func Catalogue(address string, body []byte, readErr error, expected []freshness.Expected) Condition {
	c := Condition{
		Name:    "catalogue-not-current",
		Refuses: "a published catalogue missing the newest release of a plugin, which every server reads as that version not existing",
	}
	switch {
	case address == "":
		c.Reading = NotEvaluated
		c.Detail = "no address is recorded as answering, so there was nothing to read"
	case readErr != nil:
		c.Reading = NotEvaluated
		c.Detail = fmt.Sprintf("%s could not be read: %v", address, readErr)
	default:
		if err := freshness.Judge(body, expected); err != nil {
			c.Reading = Blocks
			c.Detail = fmt.Sprintf("%s: %v", address, err)
			return c
		}
		c.Reading = Clear
		c.Detail = fmt.Sprintf("%s lists the newest finished release of all %d declared plugin(s) that have one", address, len(expected))
	}
	return c
}

// DeclaredSet judges what this run made of the declared source set.
//
// The refusal it carries is sources.Judge's rather than a second copy of it, so
// a release is refused by the same rule that stops a publication run. A run that
// resolved nothing because a credential expired produces exactly the file a
// correct run over an empty world would produce, and a release cut from it
// replaces a working catalogue with an empty one.
func DeclaredSet(resolutions []sources.Resolution) Condition {
	c := Condition{
		Name:    "nothing-to-publish",
		Refuses: "a catalogue with no entries in it, which is what a total failure and a true answer both look like",
	}
	if len(resolutions) == 0 {
		c.Reading = NotEvaluated
		c.Detail = "nothing was resolved on this run, so what the catalogue would hold is unknown"
		return c
	}
	if err := sources.Judge(resolutions); err != nil {
		c.Reading = Blocks
		c.Detail = err.Error()
		return c
	}
	resolved := 0
	for _, r := range resolutions {
		if r.State == sources.Resolved {
			resolved++
		}
	}
	c.Reading = Clear
	c.Detail = fmt.Sprintf("%d of %d declaration(s) resolved with something to publish", resolved, len(resolutions))
	return c
}

// Expectations is what the published catalogue has to hold, read off the release
// lists rather than off anything this repository wrote.
//
// The newest finished release, which is the first one on that side of the
// channel split rather than the first one in the list. A resolution carries both
// sides, and the newest release overall is a test build often enough that taking
// it would expect the catalogue to hold something the catalogue does not publish.
//
// A declaration that resolved nothing contributes no expectation. It is not an
// omission: a plugin with no finished release has nothing the catalogue could be
// missing, and asserting otherwise would refuse a release for a repository that
// is behaving correctly.
func Expectations(resolutions []sources.Resolution) []freshness.Expected {
	var out []freshness.Expected
	for _, r := range resolutions {
		if !r.Declaration.On() {
			continue
		}
		for _, rel := range r.Releases {
			if !r.Declaration.IsFinished(rel.Tag) {
				continue
			}
			out = append(out, freshness.Expected{
				Slug: r.Declaration.Slug,
				Path: r.Declaration.Path(),
				Tag:  rel.Tag,
			})
			break
		}
	}
	return out
}

// Reader reads what one address answers with, the way a Jellyfin server would.
//
// It is a function the caller supplies rather than a request made here, which
// is what keeps this package judgeable against planted readings: the suite hands
// it a reader that answers from a literal, and the verb hands it one that leaves
// the runner.
type Reader func(address string) ([]byte, error)

// Conditions assembles the whole set from what this run can read.
//
// The assembly is here rather than in the verb because the empty-list branch is
// the one that decides what a tree with no recorded address is told, and a
// release judged against three conditions instead of four would pass for having
// asked one question fewer. That branch now lives in CatalogueConditions below,
// which is the half a second caller asks for, and this function is what puts the
// four together.
//
// One condition per recorded address rather than one for the list. An address
// that has stopped answering is a fact about that address and says nothing about
// its neighbour, and a single condition covering both would have to choose which
// of them to report.
func Conditions(root string, recorded []string, resolutions []sources.Resolution, read Reader) []Condition {
	return append([]Condition{
		Licence(root),
		InstallAddress(recorded),
		DeclaredSet(resolutions),
	}, CatalogueConditions(recorded, resolutions, read)...)
}

// CatalogueConditions is the catalogue half of that set on its own.
//
// It is separated out because the freshness watch asks this half and no other.
// Three of the four conditions above are states of this tree that hold still
// until somebody changes them; this one starts holding on its own, the moment a
// plugin releases and before anybody has touched anything here, which is why it
// is the one asked on a schedule. A watch carrying its own assembly of the same
// condition would answer differently from the release verb on the day either of
// them moved, and the two disagreeing about one catalogue is worse than either
// being wrong alone.
//
// The empty-list branch is why this is a function rather than a loop written
// twice. A loop over an empty list runs zero times and produces no condition at
// all, so a run with no recorded address would report nothing rather than
// refusing, and a watch that read nowhere would be green.
func CatalogueConditions(recorded []string, resolutions []sources.Resolution, read Reader) []Condition {
	expected := Expectations(resolutions)
	if len(recorded) == 0 {
		return []Condition{Catalogue("", nil, nil, expected)}
	}

	out := make([]Condition, 0, len(recorded))
	for _, addr := range recorded {
		body, err := read(addr)
		out = append(out, Catalogue(addr, body, err, expected))
	}
	return out
}

// Report writes one line per condition and then the counts.
//
// Every condition appears with what happened to it, including the ones that
// cleared, because a run that evaluated three of four must not read as a run
// that evaluated four and found nothing. That is the same sentence the merge
// gate's own report is built on.
func Report(w io.Writer, cs []Condition) {
	width := 0
	for _, c := range cs {
		if n := len(c.Name); n > width {
			width = n
		}
	}

	fmt.Fprintf(w, "what a release is refused for, and what this run read:\n")
	counts := map[Reading]int{}
	for _, c := range cs {
		counts[c.Reading]++
		fmt.Fprintf(w, "  %-*s  %-13s %s\n", width, c.Name, c.Reading, c.Detail)
		fmt.Fprintf(w, "  %-*s  %-13s it refuses %s\n", width, "", "", c.Refuses)
	}
	fmt.Fprintf(w, "\n%d condition(s): %d clear, %d blocking, %d not evaluated.\n",
		len(cs), counts[Clear], counts[Blocks], counts[NotEvaluated])
}

// Judge refuses unless every condition was evaluated and every one of them is
// clear.
//
// The two ways of not being clear are named in different words and both refuse.
// A condition that holds is a fact about the tree; a condition nothing is known
// about is a fact about this run, and the repair is different in each case. What
// they have in common is that neither is a release anybody may cut, because
// "checked and fine" and "not checked" are the distinction this whole package
// exists to keep.
func Judge(cs []Condition) error {
	if len(cs) == 0 {
		return errors.New("no condition was evaluated, so nothing about this tree has been read; a release is refused")
	}

	var blocking, unknown []string
	for _, c := range cs {
		switch c.Reading {
		case Blocks:
			blocking = append(blocking, fmt.Sprintf("%s: %s", c.Name, c.Detail))
		case NotEvaluated:
			unknown = append(unknown, fmt.Sprintf("%s: %s", c.Name, c.Detail))
		}
	}

	var b strings.Builder
	if len(blocking) > 0 {
		fmt.Fprintf(&b, "a release is refused, %d condition(s) hold:\n  %s\n",
			len(blocking), strings.Join(blocking, "\n  "))
	}
	if len(unknown) > 0 {
		fmt.Fprintf(&b, "a release is refused, %d condition(s) were not evaluated and an unread condition is not a clear one:\n  %s\n",
			len(unknown), strings.Join(unknown, "\n  "))
	}
	if b.Len() == 0 {
		return nil
	}
	return errors.New(strings.TrimRight(b.String(), "\n"))
}
