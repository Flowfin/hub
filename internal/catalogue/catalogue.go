// Package catalogue is the route from the declared source set to the bytes at
// the install address.
//
// Every part of the run is somewhere else and is judged by its own suite:
// internal/sources resolves a declaration to a repository, internal/posture
// decides which of a plugin's releases become version entries and which stop the
// run, internal/identity says who the plugin is, manifest.Encode fixes the byte
// format, and internal/publish places the bytes. What this package adds is the
// order they run in.
//
// The order is the point rather than the glue. Nothing is placed until every
// declaration has resolved, every plugin has been classified and every entry has
// been built, so a run that stops has written nothing and the address goes on
// answering with the file from the last run that finished. The alternative shape,
// placing each plugin as it is built, publishes a catalogue that is missing
// whatever came after the failure and reports the failure separately, which a
// server reads as a plugin that no longer exists.
//
// decisions/manifest-is-generated.md leaves where the generator runs to #31, and
// this package is that answer together with the verb in main.go that calls it.
//
// Nothing here reaches the network. The release lists arrive through a
// sources.Lister and the release files through a pairing.Fetch, which is what
// lets the gate judge the order above against fixtures rather than against
// somebody else's service: decisions/headless-and-unelevated.md is why that
// matters.
package catalogue

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"flowfin.dev/hub/internal/identity"
	"flowfin.dev/hub/internal/pairing"
	"flowfin.dev/hub/internal/posture"
	"flowfin.dev/hub/internal/publish"
	"flowfin.dev/hub/internal/sources"
	"flowfin.dev/hub/manifest"
)

// Route is what one publication run needs from outside itself.
//
// The four fields are the whole of what a run depends on that a fixture cannot
// supply, which is the shape that keeps this package testable: a test hands it a
// release list, some asset bodies and a directory of its own, and the code under
// it is the code that runs against the real ones.
type Route struct {
	// Root is the directory Target is resolved against, which for a real run is
	// the repository root the run was started from.
	Root string

	// Target is where the bytes are placed. publish.Stable is the published one
	// and is not read here directly, because a package that reached for it would
	// be a second place the address is decided.
	Target publish.Target

	// Lister answers what a declared repository has published.
	Lister sources.Lister

	// Fetch reads one asset of a release.
	Fetch pairing.Fetch
}

// Publish runs the route and says what it did.
//
// Each refusal below is somebody else's rule applied in order: the declared set
// has to resolve, every plugin's newest release has to be publishable, and the
// catalogue has to have something in it. Only then are bytes produced, and they
// are produced into publish.Place, which is what makes the write atomic from the
// point of view of a server polling the address.
func (r Route) Publish(ctx context.Context, out io.Writer, declarations []sources.Declaration) error {
	resolutions := sources.Resolve(ctx, r.Lister, declarations)
	fmt.Fprint(out, sources.Report(resolutions))
	if err := sources.Judge(resolutions); err != nil {
		return err
	}

	plugins, plans := Build(resolutions, r.Fetch)
	fmt.Fprintf(out, "\nwhat each resolved plugin's releases came to:\n%s", posture.Report(plans))
	if err := posture.Judge(plans); err != nil {
		return err
	}
	if err := Judge(plugins); err != nil {
		return err
	}

	placed, err := publish.Place(r.Root, r.Target, func(w io.Writer) error {
		return manifest.Encode(w, plugins)
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\n%s: %s, carrying %d plugin(s) and %d version(s).\n",
		r.Target.Name, placed, len(plugins), countVersions(plugins))
	return nil
}

// Build turns the resolutions into the entries of one manifest, and the plan
// behind each of them.
//
// A plan is returned for every declaration that resolved, including the ones
// that contribute nothing, because the report is what tells a short catalogue
// from a correct one and it can only say what it was given. A declaration that
// did not resolve has already been reported by sources.Report and is not
// repeated here under a second vocabulary.
//
// An entry is built only for a plugin whose plan stops nothing and carries at
// least one version. The two exclusions are different sentences.
// decisions/failure-posture.md makes a stop fatal, so a caller that placed an
// entry from a stopping plan would be publishing bytes the run is about to
// refuse; and an entry with no versions is a plugin a server offers and cannot
// install, which the same file calls worse than one that is not offered.
func Build(resolutions []sources.Resolution, fetch pairing.Fetch) ([]manifest.Plugin, []posture.Plan) {
	var (
		plugins []manifest.Plugin
		plans   []posture.Plan
	)

	for _, r := range resolutions {
		if r.State != sources.Resolved {
			continue
		}
		slug := r.Declaration.Slug

		// The publishable side of the split, which is the declaration's own
		// pattern rather than a second reading of the tags.
		// decisions/channel-model.md publishes these and nothing else.
		finished := make([]sources.Release, 0, r.Finished)
		for _, release := range r.Releases {
			if r.Declaration.IsFinished(release.Tag) {
				finished = append(finished, release)
			}
		}

		plan := posture.Of(slug, finished, fetch)

		// Identity is read from both sides of the split, which is
		// decisions/plugin-identity.md: the same plugin appearing under two
		// names is a guid a server cannot resolve, so the answer does not depend
		// on which channel is being generated.
		fields, note := identityOf(slug, r.Releases, fetch)
		if note != nil {
			plan.Stops = append(plan.Stops, *note)
		}

		plans = append(plans, plan)
		// The note is asked about again rather than left to the stop it just
		// became. Reading it off the stop count couples "this plugin has no
		// name" to a decision one line above about how loud that is, and the
		// entry built from a Fields nothing filled in carries an empty guid,
		// which a server keys an installed plugin by.
		if note != nil || len(plan.Stops) > 0 || len(plan.Versions) == 0 {
			continue
		}
		plugins = append(plugins, fields.Entry(plan.Versions))
	}

	return plugins, plans
}

// identityOf reads the plugin's identity from its releases, or says why it could
// not be, and a plugin whose identity cannot be read stops the run.
//
// That it stops rather than being skipped is worth the sentence, because the
// asymmetry everywhere else in this run goes the other way. identity.Of picks
// the newest release that carries a descriptor at all, so a refusal from it is a
// defect in the newest descriptor rather than in an old one, and
// decisions/failure-posture.md puts a defect in what this run is publishing on
// the fatal side. The alternative is worse in both directions:
// decisions/plugin-identity.md refuses the fallback to an older release, because
// that publishes a name the plugin has stopped using, and dropping the plugin
// instead shrinks the catalogue by one silently, which is the shape that file
// exists against.
func identityOf(plugin string, releases []sources.Release, fetch pairing.Fetch) (identity.Fields, *posture.Note) {
	var bearings []identity.Bearing
	for _, release := range releases {
		descriptor, err := identity.SelectDescriptor(plugin, release.Tag, release.Assets)
		if err != nil {
			// A release that carries no descriptor beside its archive is not a
			// bearing on the question. Whether that release can be published is
			// the release level's question and posture.Of has already answered
			// it for the finished ones.
			continue
		}
		body, err := fetch(descriptor)
		if err != nil {
			return identity.Fields{}, &posture.Note{
				Plugin:    plugin,
				Release:   release.Tag,
				Condition: posture.Transport,
				Detail:    fmt.Sprintf("reading %s: %v", descriptor.Name, err),
			}
		}
		bearings = append(bearings, identity.Bearing{Release: release.Tag, Body: body})
	}

	fields, err := identity.Of(plugin, bearings)
	if err == nil {
		return fields, nil
	}

	note := posture.Note{Plugin: plugin, Release: "no release", Condition: "identity", Detail: err.Error()}
	var refusal *identity.Refusal
	if errors.As(err, &refusal) {
		note.Condition, note.Detail = refusal.Reason.String(), refusal.Detail
		if refusal.Release != "" {
			note.Release = refusal.Release
		}
	}
	return identity.Fields{}, &note
}

// Judge refuses a catalogue with no entries in it.
//
// decisions/failure-posture.md spends its longest section on this one: a run
// that resolved nothing because a credential expired produces exactly the file a
// correct run over an empty set would produce, so placing it replaces a working
// catalogue with an empty one and reports success.
//
// sources.Judge already refuses a run in which no declaration resolved, and this
// asks the question one layer later, about entries rather than declarations. No
// state the layers below produce today reaches it with a nil error: a resolved
// declaration has a newest finished release, and that release either becomes a
// version or stops the run. It is here because that is a property of today's
// classification rather than of this route, and the failure it names is one
// nothing else would catch.
func Judge(plugins []manifest.Plugin) error {
	if len(plugins) == 0 {
		return errors.New("no declared plugin produced an entry, and an empty catalogue is not placed: " +
			"it would replace the file the address answers with today, and a server reading it shows the operator " +
			"nothing and reports no error")
	}
	return nil
}

// countVersions is what the run says it published, so that a catalogue which
// lost every version of a plugin but one is not read off a plugin count that did
// not move.
func countVersions(plugins []manifest.Plugin) int {
	n := 0
	for _, p := range plugins {
		n += len(p.Versions)
	}
	return n
}

// Memo reads each asset once however many layers ask for it.
//
// The release files are read twice by construction: internal/posture reads a
// descriptor to build the version entry, and the identity rule reads the
// descriptor of every release on both sides of the split to choose between them.
// Both are the same bytes at the same address, and the rate limit they spend is
// shared with everything else using the token.
//
// It caches the failure as well as the body, so a run does not ask a service
// that has started refusing for the same file again, and so that two layers
// reading one asset cannot reach two different conclusions about it.
func Memo(fetch pairing.Fetch) pairing.Fetch {
	type answer struct {
		body []byte
		err  error
	}
	var (
		mu   sync.Mutex
		seen = map[string]answer{}
	)
	return func(a pairing.Asset) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		if got, ok := seen[a.URL]; ok {
			return got.body, got.err
		}
		body, err := fetch(a)
		seen[a.URL] = answer{body: body, err: err}
		return body, err
	}
}
