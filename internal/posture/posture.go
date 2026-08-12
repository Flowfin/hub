// Package posture applies decisions/failure-posture.md to one plugin's
// releases: which of them become version entries, which are skipped by name,
// and which stop the run.
//
// The asymmetry it exists for is that a published release is immutable. A
// release from two years ago that shipped no archive, or a descriptor nothing
// can parse, will be missing it forever, so stopping the run over it converts
// somebody else's old mistake into this project's standing outage. A defect in
// the release this run is trying to publish is the project's own mistake and is
// worth stopping for while somebody is awake to fix it.
//
// So the classification turns on one question, which release is the newest for
// its plugin and channel, and the answer has to be available before any
// descriptor is read. It is the publication time, because
// decisions/manifest-schema.md refuses the tag as a version string and there is
// nothing else in a release list to order by.
//
// A skip is never silent. A manifest that is short because releases were skipped
// and one that is short because there was nothing to add are the same file, and
// only the output tells them apart.
//
// Nothing here reaches the network. Bytes arrive through the Fetch the caller
// supplies, which is the same shape internal/pairing takes and for the same
// reason: every rule below is judged against a fixture in the gate.
package posture

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"flowfin.dev/hub/internal/identity"
	"flowfin.dev/hub/internal/pairing"
	"flowfin.dev/hub/internal/sources"
	"flowfin.dev/hub/internal/version"
	"flowfin.dev/hub/manifest"
)

// Note is one release that did not become a published entry, and why.
//
// It names the plugin and the release, because a run reporting that a version
// was skipped without saying which one sends whoever reads it to every release
// page in turn.
type Note struct {
	Plugin  string
	Release string

	// Condition is the short name of what was hit, which is the vocabulary the
	// layer that found it uses rather than a second one invented here.
	Condition string

	// Detail says what happened in words somebody can act on.
	Detail string
}

func (n Note) String() string {
	return fmt.Sprintf("%s %s: %s: %s", n.Plugin, n.Release, n.Condition, n.Detail)
}

// Trimmed is the condition a release cut by the per-target cap carries.
//
// Named as trimmed rather than as defective, because a run reporting a capped
// release the same way it reports a broken one teaches everybody to ignore both.
// decisions/version-cap.md is the rule and decisions/failure-posture.md is why
// it is named at all.
const Trimmed = "trimmed"

// Transport is the condition a read that did not happen carries. It is always a
// stop: a request that failed says nothing about the release it was for, and a
// run that treated it as a defective release would publish a short catalogue
// that looks exactly like a correct one.
const Transport = "read-failed"

// Plan is what one plugin's releases came to.
type Plan struct {
	Plugin   string
	Versions []manifest.Version
	Skips    []Note
	Stops    []Note
}

// Of classifies the releases of one plugin.
//
// The releases given are the publishable side of the channel split, which is
// decisions/channel-model.md's question and not this package's. Every one of
// them is attempted, rather than stopping at the first defect, so that somebody
// fixing three of them finds all three in one run.
func Of(plugin string, releases []sources.Release, fetch pairing.Fetch) Plan {
	plan := Plan{Plugin: plugin}

	ordered := make([]sources.Release, len(releases))
	copy(ordered, releases)
	sort.SliceStable(ordered, func(i, j int) bool { return newerFirst(ordered[i], ordered[j]) })

	for i, release := range ordered {
		// The first one is the newest for this plugin and channel, which is the
		// release this run exists to publish. A defect in it stops the run and
		// the same defect below it does not.
		newest := i == 0

		entry, note, err := read(plugin, release, fetch)
		switch {
		case err != nil:
			// A read that did not happen, rather than a release that cannot be
			// published. Fatal wherever it happened, and not only on the newest.
			plan.Stops = append(plan.Stops, Note{
				Plugin:    plugin,
				Release:   release.Tag,
				Condition: Transport,
				Detail:    err.Error(),
			})
		case note != nil && newest:
			plan.Stops = append(plan.Stops, *note)
		case note != nil:
			plan.Skips = append(plan.Skips, *note)
		default:
			plan.Versions = append(plan.Versions, entry)
		}
	}

	plan.Versions, plan.Skips = applyCap(plan.Plugin, plan.Versions, plan.Skips)
	return plan
}

// read turns one release into an entry, a note saying why it cannot be
// published, or an error saying the read did not happen. Exactly one of the
// three is returned.
func read(plugin string, release sources.Release, fetch pairing.Fetch) (manifest.Version, *Note, error) {
	pair, err := pairing.Resolve(release.Tag, release.Assets, fetch)
	if note, ok := noteOf(plugin, release.Tag, err); ok {
		return manifest.Version{}, note, nil
	} else if err != nil {
		return manifest.Version{}, nil, err
	}

	descriptor, err := identity.SelectDescriptor(plugin, release.Tag, release.Assets)
	if note, ok := noteOf(plugin, release.Tag, err); ok {
		return manifest.Version{}, note, nil
	} else if err != nil {
		return manifest.Version{}, nil, err
	}

	body, err := fetch(descriptor)
	if err != nil {
		return manifest.Version{}, nil, fmt.Errorf("reading %s: %w", descriptor.Name, err)
	}

	entry, err := version.Read(plugin, release.Tag, body, pair)
	if note, ok := noteOf(plugin, release.Tag, err); ok {
		return manifest.Version{}, note, nil
	} else if err != nil {
		return manifest.Version{}, nil, err
	}

	return entry, nil, nil
}

// noteOf recognises the three refusals the layers below return, and reports
// whether the error was one of them.
//
// A refusal is a release that cannot be published and an error is a run that
// could not find out. Collapsing the two is how a credential failure becomes a
// catalogue that quietly lost a plugin, so they are told apart by type here and
// nowhere else.
func noteOf(plugin, release string, err error) (*Note, bool) {
	if err == nil {
		return nil, false
	}

	var paired *pairing.Refusal
	if errors.As(err, &paired) {
		return &Note{Plugin: plugin, Release: release, Condition: paired.Reason.String(), Detail: paired.Detail}, true
	}
	var identified *identity.Refusal
	if errors.As(err, &identified) {
		return &Note{Plugin: plugin, Release: release, Condition: identified.Reason.String(), Detail: identified.Detail}, true
	}
	var versioned *version.Refusal
	if errors.As(err, &versioned) {
		return &Note{Plugin: plugin, Release: release, Condition: versioned.Reason.String(), Detail: versioned.Detail}, true
	}
	return nil, false
}

// applyCap applies the per-target cap and names what it dropped.
//
// The cap is applied here rather than left to the pass that builds the file,
// because this is the layer that owes a sentence about every release the run
// did not publish, and a release trimmed by the cap is one of those.
func applyCap(plugin string, versions []manifest.Version, skips []Note) ([]manifest.Version, []Note) {
	kept := manifest.CapPerTarget(versions, manifest.Cap)

	remaining := map[manifest.Version]int{}
	for _, v := range kept {
		remaining[v]++
	}
	for _, v := range versions {
		if remaining[v] > 0 {
			remaining[v]--
			continue
		}
		skips = append(skips, Note{
			Plugin:    plugin,
			Release:   v.Version,
			Condition: Trimmed,
			Detail: fmt.Sprintf("target %s already carries the %d newest versions this run publishes",
				v.TargetABI, manifest.Cap),
		})
	}
	return kept, skips
}

// newerFirst orders releases by publication time, newest first, and breaks a tie
// by tag descending so the order is total.
//
// A release the API reported no publication time for sorts last rather than
// first. It cannot be shown to be the newest, and the cost of guessing wrong in
// that direction is a run that stops the world over a release from years ago.
func newerFirst(a, b sources.Release) bool {
	switch {
	case a.Published.IsZero() && !b.Published.IsZero():
		return false
	case !a.Published.IsZero() && b.Published.IsZero():
		return true
	case !a.Published.Equal(b.Published):
		return a.Published.After(b.Published)
	}
	return a.Tag > b.Tag
}

// Report writes what became of every release, in the order the plans were made.
//
// A plugin that published nothing still gets a line. The whole reason this
// output exists is that a manifest which is short because releases were skipped
// and one that is short because there was nothing to add are the same file.
func Report(plans []Plan) string {
	var b strings.Builder
	for _, p := range plans {
		fmt.Fprintf(&b, "  %-20s %d version(s), %d skipped, %d stopping the run\n",
			p.Plugin, len(p.Versions), len(p.Skips), len(p.Stops))
		for _, n := range p.Skips {
			fmt.Fprintf(&b, "    skipped %s\n", strings.TrimPrefix(n.String(), p.Plugin+" "))
		}
		for _, n := range p.Stops {
			fmt.Fprintf(&b, "    stops   %s\n", strings.TrimPrefix(n.String(), p.Plugin+" "))
		}
	}
	return b.String()
}

// Judge turns the plans into the run's verdict.
//
// Only a stop is fatal. A skip has already been named in the report and the run
// publishes the rest, which is the asymmetry this package opens with.
func Judge(plans []Plan) error {
	var stops []string
	for _, p := range plans {
		for _, n := range p.Stops {
			stops = append(stops, n.String())
		}
	}
	if len(stops) == 0 {
		return nil
	}
	return fmt.Errorf("%d release(s) this run is trying to publish cannot be:\n  %s",
		len(stops), strings.Join(stops, "\n  "))
}
