package catalogue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"flowfin.dev/hub/internal/pairing"
	"flowfin.dev/hub/internal/publish"
	"flowfin.dev/hub/internal/sources"
	"flowfin.dev/hub/manifest"
)

// Every fixture here is invented, the only host in one is the domain reserved
// for documentation, and no test in this package makes a request. That is what
// decisions/headless-and-unelevated.md asks of anything the gate runs, and it is
// the reason the route takes its release lists and its asset bodies as
// interfaces rather than reaching for a client of its own.

const (
	digest = "0123456789abcdef0123456789abcdef"
	guid   = "0a1b2c3d-4e5f-6071-8293-a4b5c6d7e8f9"
)

// world is the asset bodies a run would have fetched, by address.
type world map[string][]byte

func (w world) fetch(a pairing.Asset) ([]byte, error) {
	body, ok := w[a.URL]
	if !ok {
		return nil, fmt.Errorf("%s answered nothing", a.Name)
	}
	return body, nil
}

// listing is what each declared repository has published, by declared path.
type listing map[string][]sources.Release

func (l listing) ListReleases(_ context.Context, account, repository string) ([]sources.Release, string, error) {
	path := account + "/" + repository
	releases, ok := l[path]
	if !ok {
		return nil, "", sources.ErrNotFound
	}
	return releases, path, nil
}

// declaring builds the declared set through the loader rather than by filling in
// the struct, because the pattern that decides which side of the channel split a
// tag falls on is compiled there and a hand-built declaration carries none.
func declaring(t *testing.T, slugs ...string) []sources.Declaration {
	t.Helper()
	files := fstest.MapFS{}
	for _, slug := range slugs {
		files[slug+".json"] = &fstest.MapFile{Data: fmt.Appendf(nil,
			`{"account": "an-account", "repository": "jellyfin-plugin-%s", "slug": %q, "stable_tags": "-stable$", "enabled": true}`,
			slug, slug)}
	}
	declarations, err := sources.Load(files)
	if err != nil {
		t.Fatalf("the fixture declarations do not load: %v", err)
	}
	return declarations
}

// at is a publication time, days after an arbitrary epoch, so a fixture says
// which release is newer as a number rather than as an order in a slice.
func at(day int) time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, day)
}

// published builds a release that can be published: an archive, a sidecar whose
// contents name it, and a descriptor beside it carrying both halves, the
// plugin's identity and the version entry's fields.
func published(w world, plugin, tag string, day int, number, name string) sources.Release {
	archive := plugin + "_" + number + ".zip"
	base := "https://example.com/" + plugin + "/" + tag + "/"

	w[base+archive] = []byte("not really an archive")
	w[base+archive+".md5sum"] = fmt.Appendf(nil, "%s  %s\n", digest, archive)
	w[base+archive+".meta.json"] = fmt.Appendf(nil,
		`{"guid": %q, "name": %q, "description": "What it does, at length.", "overview": "What it does.",
		  "owner": "an-account", "category": "General",
		  "version": %q, "targetAbi": "10.11.0.0", "changelog": "A line.", "timestamp": "2026-01-01T00:00:00Z"}`,
		guid, name, number)

	return sources.Release{
		Tag:       tag,
		Published: at(day),
		Assets: []pairing.Asset{
			{Name: archive, URL: base + archive, Size: 1024},
			{Name: archive + ".md5sum", URL: base + archive + ".md5sum", Size: 74},
			{Name: archive + ".meta.json", URL: base + archive + ".meta.json", Size: 512},
		},
	}
}

// without returns the same release with one of its assets removed, which is how
// every defect below is planted: by taking away a file a release would have
// shipped rather than by inventing a condition.
func without(release sources.Release, suffix string) sources.Release {
	kept := make([]pairing.Asset, 0, len(release.Assets))
	for _, a := range release.Assets {
		if strings.HasSuffix(a.Name, suffix) {
			continue
		}
		kept = append(kept, a)
	}
	release.Assets = kept
	return release
}

// routeInto is a route placing into a directory of its own, with the target's
// directory already made, which publish.Place requires and does not create.
func routeInto(t *testing.T, l listing, w world) (Route, string) {
	t.Helper()
	root := t.TempDir()
	target := publish.Target{Dir: "docs", Name: "manifest.json"}
	if err := os.MkdirAll(filepath.Join(root, target.Dir), 0o755); err != nil {
		t.Fatalf("making the directory the fixture publishes from: %v", err)
	}
	return Route{Root: root, Target: target, Lister: l, Fetch: w.fetch}, root
}

func placedBytes(t *testing.T, r Route) []byte {
	t.Helper()
	body, err := os.ReadFile(r.Target.Path(r.Root))
	if err != nil {
		t.Fatalf("reading what the address answers with: %v", err)
	}
	return body
}

// TestARunPlacesTheGeneratedBytesAtTheConfiguredLocation is the first clause of
// the Done-when of #31.
func TestARunPlacesTheGeneratedBytesAtTheConfiguredLocation(t *testing.T) {
	w := world{}
	l := listing{"an-account/jellyfin-plugin-a-plugin": []sources.Release{
		published(w, "a-plugin", "1.0.0-stable", 10, "1.0.0.0", "A Plugin"),
	}}
	route, _ := routeInto(t, l, w)

	var out strings.Builder
	if err := route.Publish(context.Background(), &out, declaring(t, "a-plugin")); err != nil {
		t.Fatalf("a run over one publishable release failed: %v\n%s", err, out.String())
	}

	var placed []manifest.Plugin
	if err := json.Unmarshal(placedBytes(t, route), &placed); err != nil {
		t.Fatalf("the placed file is not the manifest: %v", err)
	}
	if len(placed) != 1 || len(placed[0].Versions) != 1 {
		t.Fatalf("the catalogue carries %d plugin(s): %+v", len(placed), placed)
	}
	if placed[0].Name != "A Plugin" || placed[0].GUID != guid {
		t.Errorf("the entry does not carry the descriptor's identity: %+v", placed[0])
	}
	if placed[0].Versions[0].Version != "1.0.0.0" || placed[0].Versions[0].Checksum != digest {
		t.Errorf("the version entry does not carry the release's build: %+v", placed[0].Versions[0])
	}

	// The bytes are the encoder's rather than something this route shapes on the
	// way past, which is what makes a regenerated file comparable to a published
	// one.
	var want strings.Builder
	if err := manifest.Encode(&want, placed); err != nil {
		t.Fatalf("encoding what was placed: %v", err)
	}
	if string(placedBytes(t, route)) != want.String() {
		t.Errorf("the placed bytes are not manifest.Encode's:\n%s", placedBytes(t, route))
	}

	for _, phrase := range []string{"a-plugin", "manifest.json", "wrote new bytes", "1 plugin(s)"} {
		if !strings.Contains(out.String(), phrase) {
			t.Errorf("the run output does not carry %q:\n%s", phrase, out.String())
		}
	}
}

// TestARunThatStopsLeavesThePublishedFileByteIdentical is the second clause of
// the Done-when of #31, at the level of the run rather than of the write.
//
// internal/publish holds the half about a write that fails in the middle. This
// is the half about a run that fails before the write: a defect in the release
// this run exists to publish is fatal, and a fatal run has to leave the address
// answering with the last file that was good, not with a shorter one.
func TestARunThatStopsLeavesThePublishedFileByteIdentical(t *testing.T) {
	w := world{}
	l := listing{"an-account/jellyfin-plugin-a-plugin": []sources.Release{
		without(published(w, "a-plugin", "2.0.0-stable", 20, "2.0.0.0", "A Plugin"), ".zip.md5sum"),
		published(w, "a-plugin", "1.0.0-stable", 10, "1.0.0.0", "A Plugin"),
	}}
	route, _ := routeInto(t, l, w)

	before := []byte("[\n    {\"guid\": \"the file from the last run that finished\"}\n]\n")
	if err := os.WriteFile(route.Target.Path(route.Root), before, 0o644); err != nil {
		t.Fatalf("planting the previously published file: %v", err)
	}

	var out strings.Builder
	err := route.Publish(context.Background(), &out, declaring(t, "a-plugin"))
	if err == nil {
		t.Fatal("a defect in the release this run publishes exited zero")
	}
	if !strings.Contains(err.Error(), "2.0.0-stable") {
		t.Errorf("the refusal does not name the release that stopped it: %v", err)
	}
	if got := placedBytes(t, route); string(got) != string(before) {
		t.Fatalf("a stopping run rewrote the published file:\n%s", got)
	}
}

// TestNothingIsPlacedForADeclarationThatDoesNotResolve is the same property one
// layer earlier: a repository that answers with a not-found is a declaration
// pointing at nothing, and a run that continued would shrink the catalogue by a
// plugin without saying so.
func TestNothingIsPlacedForADeclarationThatDoesNotResolve(t *testing.T) {
	w := world{}
	l := listing{"an-account/jellyfin-plugin-a-plugin": []sources.Release{
		published(w, "a-plugin", "1.0.0-stable", 10, "1.0.0.0", "A Plugin"),
	}}
	route, _ := routeInto(t, l, w)

	var out strings.Builder
	err := route.Publish(context.Background(), &out, declaring(t, "a-plugin", "another-plugin"))
	if err == nil {
		t.Fatal("a declaration pointing at nothing exited zero")
	}
	if !strings.Contains(err.Error(), "another-plugin") {
		t.Errorf("the refusal does not name the declaration: %v", err)
	}
	if _, statErr := os.Stat(route.Target.Path(route.Root)); statErr == nil {
		t.Fatal("a run that did not resolve its set placed a file")
	}
}

// TestAnUnusableHistoricalReleaseIsNamedAndTheRestIsPublished is the first
// clause of the Done-when of #28, which is what that issue is waiting on this
// route for: the classification is judged in internal/posture, and this is the
// run that prints it and publishes the rest.
func TestAnUnusableHistoricalReleaseIsNamedAndTheRestIsPublished(t *testing.T) {
	w := world{}
	l := listing{"an-account/jellyfin-plugin-a-plugin": []sources.Release{
		published(w, "a-plugin", "2.0.0-stable", 20, "2.0.0.0", "A Plugin"),
		without(published(w, "a-plugin", "1.0.0-stable", 10, "1.0.0.0", "A Plugin"), ".zip.md5sum"),
	}}
	route, _ := routeInto(t, l, w)

	var out strings.Builder
	if err := route.Publish(context.Background(), &out, declaring(t, "a-plugin")); err != nil {
		t.Fatalf("a release nobody can repair stopped the run: %v\n%s", err, out.String())
	}

	var placed []manifest.Plugin
	if err := json.Unmarshal(placedBytes(t, route), &placed); err != nil {
		t.Fatalf("the placed file is not the manifest: %v", err)
	}
	if len(placed) != 1 || len(placed[0].Versions) != 1 {
		t.Fatalf("the catalogue does not carry the one release that could be published: %+v", placed)
	}
	for _, phrase := range []string{"skipped", "1.0.0-stable", "no-usable-sidecar"} {
		if !strings.Contains(out.String(), phrase) {
			t.Errorf("the run output does not name the skip with %q:\n%s", phrase, out.String())
		}
	}
}

// TestAPluginWhoseNewestDescriptorCannotBeReadStopsTheRun holds the sentence
// identityOf is written for. The catalogue carries a second plugin that is
// perfectly publishable, so the failure the test refuses is the live one: a run
// that publishes the rest, drops this plugin and exits zero. A server shows a
// plugin that was dropped and a plugin that never existed the same way, which is
// not at all.
func TestAPluginWhoseNewestDescriptorCannotBeReadStopsTheRun(t *testing.T) {
	w := world{}
	l := listing{
		"an-account/jellyfin-plugin-a-plugin": []sources.Release{
			published(w, "a-plugin", "2.0.0-stable", 20, "2.0.0.0", "A Plugin"),
			published(w, "a-plugin", "1.0.0-stable", 10, "1.0.0.0", "A Plugin"),
		},
		"an-account/jellyfin-plugin-another-plugin": []sources.Release{
			published(w, "another-plugin", "1.0.0-stable", 10, "1.0.0.0", "Another Plugin"),
		},
	}
	// The identity fields go out of the newest descriptor and the version fields
	// stay, so its release is publishable and the plugin has no name.
	w["https://example.com/a-plugin/2.0.0-stable/a-plugin_2.0.0.0.zip.meta.json"] = []byte(
		`{"version": "2.0.0.0", "targetAbi": "10.11.0.0", "changelog": "A line.", "timestamp": "2026-01-01T00:00:00Z"}`)

	route, _ := routeInto(t, l, w)

	var out strings.Builder
	err := route.Publish(context.Background(), &out, declaring(t, "a-plugin", "another-plugin"))
	if err == nil {
		t.Fatalf("a plugin nothing can name was dropped and the run exited zero:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "a-plugin") {
		t.Errorf("the refusal does not name the plugin: %v", err)
	}
	if _, statErr := os.Stat(route.Target.Path(route.Root)); statErr == nil {
		t.Fatalf("a run that could not name a plugin placed a file:\n%s", placedBytes(t, route))
	}
}

// TestTheIdentityComesFromTheNewestDescriptorWhicheverChannelItIsIn is
// decisions/plugin-identity.md at the level of the run: the version list is per
// channel and the identity is not.
func TestTheIdentityComesFromTheNewestDescriptorWhicheverChannelItIsIn(t *testing.T) {
	w := world{}
	l := listing{"an-account/jellyfin-plugin-a-plugin": []sources.Release{
		published(w, "a-plugin", "3.0.0-beta.1", 30, "3.0.0.1", "The Renamed Plugin"),
		published(w, "a-plugin", "1.0.0-stable", 10, "1.0.0.0", "The Old Name"),
	}}
	route, _ := routeInto(t, l, w)

	var out strings.Builder
	if err := route.Publish(context.Background(), &out, declaring(t, "a-plugin")); err != nil {
		t.Fatalf("a run with a newer test build failed: %v\n%s", err, out.String())
	}

	var placed []manifest.Plugin
	if err := json.Unmarshal(placedBytes(t, route), &placed); err != nil {
		t.Fatalf("the placed file is not the manifest: %v", err)
	}
	if len(placed) != 1 {
		t.Fatalf("the catalogue carries %d plugin(s): %+v", len(placed), placed)
	}
	if placed[0].Name != "The Renamed Plugin" {
		t.Errorf("the entry carries the name %q, which is not the newest descriptor's", placed[0].Name)
	}
	if len(placed[0].Versions) != 1 || placed[0].Versions[0].Version != "1.0.0.0" {
		t.Errorf("the test build was published as a version: %+v", placed[0].Versions)
	}
}

// TestTheLocationIsChangedWithoutTouchingWhatProducesTheBytes is the third
// clause of the Done-when of #31. The same run against two targets produces the
// same bytes in two places, and nothing but the target moved.
func TestTheLocationIsChangedWithoutTouchingWhatProducesTheBytes(t *testing.T) {
	w := world{}
	l := listing{"an-account/jellyfin-plugin-a-plugin": []sources.Release{
		published(w, "a-plugin", "1.0.0-stable", 10, "1.0.0.0", "A Plugin"),
	}}
	route, root := routeInto(t, l, w)

	var out strings.Builder
	if err := route.Publish(context.Background(), &out, declaring(t, "a-plugin")); err != nil {
		t.Fatalf("the run against the first target failed: %v\n%s", err, out.String())
	}
	first := placedBytes(t, route)

	moved := route
	moved.Target = publish.Target{Dir: "somewhere-else", Name: "catalogue.json"}
	if err := os.MkdirAll(filepath.Join(root, moved.Target.Dir), 0o755); err != nil {
		t.Fatalf("making the second directory: %v", err)
	}
	if err := moved.Publish(context.Background(), &out, declaring(t, "a-plugin")); err != nil {
		t.Fatalf("the run against the moved target failed: %v\n%s", err, out.String())
	}

	if second := placedBytes(t, moved); string(second) != string(first) {
		t.Fatalf("moving the target changed the bytes:\n%s\n%s", first, second)
	}
	if !strings.Contains(out.String(), "catalogue.json") {
		t.Errorf("the run does not say where it placed the file:\n%s", out.String())
	}
}

// TestAnEmptyCatalogueIsRefusedRatherThanPlaced is the guard the package comment
// says no run reaches today. It is judged here, where the state can be made.
func TestAnEmptyCatalogueIsRefusedRatherThanPlaced(t *testing.T) {
	if err := Judge(nil); err == nil {
		t.Fatal("a catalogue with nothing in it was accepted")
	}
	if err := Judge([]manifest.Plugin{{GUID: guid}}); err != nil {
		t.Fatalf("a catalogue with an entry in it was refused: %v", err)
	}
}

// TestAnAssetIsReadOnceHoweverManyLayersAskForIt covers both halves of Memo: the
// body is remembered and so is the failure, because two layers reading one asset
// and reaching two conclusions about it is worse than either conclusion.
func TestAnAssetIsReadOnceHoweverManyLayersAskForIt(t *testing.T) {
	reads := map[string]int{}
	fetch := Memo(func(a pairing.Asset) ([]byte, error) {
		reads[a.Name]++
		if a.Name == "refuses" {
			return nil, fmt.Errorf("%s answered nothing", a.Name)
		}
		return []byte("a body"), nil
	})

	for range 3 {
		if _, err := fetch(pairing.Asset{Name: "answers", URL: "https://example.com/answers"}); err != nil {
			t.Fatalf("reading an asset that answers: %v", err)
		}
		if _, err := fetch(pairing.Asset{Name: "refuses", URL: "https://example.com/refuses"}); err == nil {
			t.Fatal("an asset that answered nothing was read")
		}
	}
	if reads["answers"] != 1 || reads["refuses"] != 1 {
		t.Fatalf("the assets were read %d and %d times", reads["answers"], reads["refuses"])
	}
}
