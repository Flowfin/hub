package posture

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"flowfin.dev/hub/internal/pairing"
	"flowfin.dev/hub/internal/sources"
	"flowfin.dev/hub/manifest"
)

// Every fixture here is invented and the only host in one is the domain reserved
// for documentation. Nothing in this package's suite reaches the network, which
// is what decisions/headless-and-unelevated.md requires of anything the gate
// runs, and the bytes arrive through the Fetch a caller supplies for exactly
// that reason.

const digest = "0123456789abcdef0123456789abcdef"

// world is the asset bodies a run would have fetched, by address.
type world map[string][]byte

func (w world) fetch(a pairing.Asset) ([]byte, error) {
	body, ok := w[a.URL]
	if !ok {
		return nil, fmt.Errorf("%s answered nothing", a.Name)
	}
	return body, nil
}

// at is a publication time, days after an arbitrary epoch, so a fixture states
// which release is newer as a number rather than as an order in a slice.
func at(day int) time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, day)
}

// published builds a release that can be published: an archive, a sidecar whose
// contents name it, and a descriptor beside it.
func published(w world, tag string, day int, number, target string) sources.Release {
	archive := "a-plugin_" + number + ".zip"
	base := "https://example.com/" + tag + "/"

	w[base+archive] = []byte("not really an archive")
	w[base+archive+".md5sum"] = fmt.Appendf(nil, "%s  %s\n", digest, archive)
	w[base+archive+".meta.json"] = fmt.Appendf(nil,
		`{"version": %q, "targetAbi": %q, "changelog": "A line.", "timestamp": "2026-01-0%dT00:00:00Z"}`,
		number, target, day%9+1)

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
// every defect in these fixtures is planted: by taking away a file a release
// would have shipped, rather than by inventing a condition.
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

// TestAnUnusableHistoricalReleaseIsSkippedByNameAndTheRestArePublished is the
// first clause of the Done-when of #28.
func TestAnUnusableHistoricalReleaseIsSkippedByNameAndTheRestArePublished(t *testing.T) {
	w := world{}
	newest := published(w, "2.0.0-stable", 30, "2.0.0.0", "10.11.0.0")
	middle := published(w, "1.5.0-stable", 20, "1.5.0.0", "10.11.0.0")
	oldest := without(published(w, "1.0.0-stable", 10, "1.0.0.0", "10.11.0.0"), ".zip.md5sum")

	plan := Of("a-plugin", []sources.Release{oldest, newest, middle}, w.fetch)

	if len(plan.Stops) != 0 {
		t.Fatalf("a release nobody can repair stopped the run: %v", plan.Stops)
	}
	if len(plan.Versions) != 2 {
		t.Fatalf("the run published %d of the 2 releases it could: %+v", len(plan.Versions), plan.Versions)
	}
	if len(plan.Skips) != 1 {
		t.Fatalf("the run named %d skips: %v", len(plan.Skips), plan.Skips)
	}

	skip := plan.Skips[0]
	if skip.Release != "1.0.0-stable" {
		t.Errorf("the skip names release %q", skip.Release)
	}
	if skip.Condition != "no-usable-sidecar" {
		t.Errorf("the skip carries condition %q", skip.Condition)
	}

	report := Report([]Plan{plan})
	for _, phrase := range []string{"a-plugin", "1.0.0-stable", "no-usable-sidecar", "skipped"} {
		if !strings.Contains(report, phrase) {
			t.Errorf("the output does not carry %q:\n%s", phrase, report)
		}
	}
	if err := Judge([]Plan{plan}); err != nil {
		t.Errorf("a named skip stopped the run: %v", err)
	}
}

// TestADefectInTheNewestReleaseStopsTheRun is the second clause of the Done-when
// of #28. The same missing file as above, moved to the release this run exists
// to publish.
func TestADefectInTheNewestReleaseStopsTheRun(t *testing.T) {
	w := world{}
	newest := without(published(w, "2.0.0-stable", 30, "2.0.0.0", "10.11.0.0"), ".zip.md5sum")
	older := published(w, "1.5.0-stable", 20, "1.5.0.0", "10.11.0.0")

	plan := Of("a-plugin", []sources.Release{older, newest}, w.fetch)

	if len(plan.Stops) != 1 {
		t.Fatalf("a defect in the newest release produced %d stops: %v", len(plan.Stops), plan.Stops)
	}
	if plan.Stops[0].Release != "2.0.0-stable" {
		t.Errorf("the stop names release %q", plan.Stops[0].Release)
	}

	err := Judge([]Plan{plan})
	if err == nil {
		t.Fatal("a build published wrong today was published anyway")
	}
	for _, phrase := range []string{"a-plugin", "2.0.0-stable", "no-usable-sidecar"} {
		if !strings.Contains(err.Error(), phrase) {
			t.Errorf("the verdict does not say %q: %v", phrase, err)
		}
	}
}

// TestTheSameDefectIsAStopOrASkipByWhichReleaseItIsIn is the asymmetry itself,
// held in one test so that a change collapsing the two sides cannot pass by
// moving a fixture.
func TestTheSameDefectIsAStopOrASkipByWhichReleaseItIsIn(t *testing.T) {
	for _, defect := range []string{".zip.md5sum", ".zip.meta.json", ".zip"} {
		w := world{}
		newest := published(w, "2.0.0-stable", 30, "2.0.0.0", "10.11.0.0")
		older := published(w, "1.0.0-stable", 10, "1.0.0.0", "10.11.0.0")

		asSkip := Of("a-plugin", []sources.Release{newest, without(older, defect)}, w.fetch)
		if len(asSkip.Stops) != 0 || len(asSkip.Skips) != 1 {
			t.Errorf("%s in the older release: %d stops and %d skips", defect, len(asSkip.Stops), len(asSkip.Skips))
		}

		asStop := Of("a-plugin", []sources.Release{without(newest, defect), older}, w.fetch)
		if len(asStop.Stops) != 1 || len(asStop.Skips) != 0 {
			t.Errorf("%s in the newest release: %d stops and %d skips", defect, len(asStop.Stops), len(asStop.Skips))
		}
	}
}

func TestTheNewestIsThePublicationTimeRatherThanTheOrderTheApiAnswered(t *testing.T) {
	// The list comes back in whatever order the API gives, and the tag is not an
	// order this can borrow: decisions/manifest-schema.md refuses the tag as a
	// version string. A run that took the first element would stop over a
	// release from years ago and publish a broken build from today.
	w := world{}
	newest := without(published(w, "1.0.0-stable", 30, "3.0.0.0", "10.11.0.0"), ".zip.md5sum")
	older := published(w, "2.0.0-stable", 10, "2.0.0.0", "10.11.0.0")

	plan := Of("a-plugin", []sources.Release{older, newest}, w.fetch)
	if len(plan.Stops) != 1 || plan.Stops[0].Release != "1.0.0-stable" {
		t.Fatalf("the newest release was decided as something else: %v %v", plan.Stops, plan.Skips)
	}
}

func TestAReleaseWithNoPublicationTimeIsNotTakenForTheNewest(t *testing.T) {
	// A release the API reported no publication time for cannot be shown to be
	// the newest, and the cost of guessing wrong in that direction is a run that
	// stops the world over a release nobody can repair.
	w := world{}
	dated := published(w, "2.0.0-stable", 30, "2.0.0.0", "10.11.0.0")
	undated := without(published(w, "1.0.0-stable", 10, "1.0.0.0", "10.11.0.0"), ".zip.md5sum")
	undated.Published = time.Time{}

	plan := Of("a-plugin", []sources.Release{undated, dated}, w.fetch)
	if len(plan.Stops) != 0 {
		t.Fatalf("a release with no publication time stopped the run: %v", plan.Stops)
	}
	if len(plan.Skips) != 1 || len(plan.Versions) != 1 {
		t.Fatalf("%d skips and %d versions", len(plan.Skips), len(plan.Versions))
	}
}

func TestAReadThatDidNotHappenStopsTheRunWhereverItWas(t *testing.T) {
	// The case decisions/failure-posture.md says most needs to be fatal, because
	// its symptom is a short list and a short list looks exactly like success. It
	// is a stop in the oldest release as much as in the newest, which is the one
	// place the asymmetry above does not apply.
	w := world{}
	newest := published(w, "2.0.0-stable", 30, "2.0.0.0", "10.11.0.0")
	older := published(w, "1.0.0-stable", 10, "1.0.0.0", "10.11.0.0")
	for address := range w {
		if strings.HasSuffix(address, ".md5sum") && strings.Contains(address, "1.0.0-stable") {
			delete(w, address)
		}
	}

	plan := Of("a-plugin", []sources.Release{newest, older}, w.fetch)
	if len(plan.Stops) != 1 {
		t.Fatalf("a read that failed produced %d stops: %v %v", len(plan.Stops), plan.Stops, plan.Skips)
	}
	if plan.Stops[0].Condition != Transport {
		t.Errorf("a read that failed was reported as %q, which is a release that cannot be published", plan.Stops[0].Condition)
	}
	if plan.Stops[0].Release != "1.0.0-stable" {
		t.Errorf("the stop names release %q", plan.Stops[0].Release)
	}
}

func TestEveryDefectIsReportedRatherThanTheFirst(t *testing.T) {
	// Somebody fixing three of them should need one run to find all three, not
	// three runs.
	w := world{}
	newest := published(w, "4.0.0-stable", 40, "4.0.0.0", "10.11.0.0")
	plan := Of("a-plugin", []sources.Release{
		newest,
		without(published(w, "3.0.0-stable", 30, "3.0.0.0", "10.11.0.0"), ".zip.md5sum"),
		without(published(w, "2.0.0-stable", 20, "2.0.0.0", "10.11.0.0"), ".zip.meta.json"),
		without(published(w, "1.0.0-stable", 10, "1.0.0.0", "10.11.0.0"), ".zip"),
	}, w.fetch)

	if len(plan.Skips) != 3 {
		t.Fatalf("%d of 3 defects were named: %v", len(plan.Skips), plan.Skips)
	}
	conditions := map[string]bool{}
	for _, n := range plan.Skips {
		conditions[n.Condition] = true
	}
	if len(conditions) != 3 {
		t.Errorf("three different defects were reported as %d condition(s): %v", len(conditions), conditions)
	}
}

func TestAReleaseTrimmedByTheCapIsNamedAsTrimmedRatherThanAsDefective(t *testing.T) {
	// decisions/failure-posture.md: a run that reports a capped release the same
	// way it reports a broken one teaches everybody to ignore both.
	w := world{}
	var releases []sources.Release
	for i := 1; i <= manifest.Cap+2; i++ {
		releases = append(releases, published(w,
			fmt.Sprintf("1.%d.0-stable", i), i, fmt.Sprintf("1.%d.0.0", i), "10.11.0.0"))
	}

	plan := Of("a-plugin", releases, w.fetch)
	if len(plan.Versions) != manifest.Cap {
		t.Fatalf("the cap kept %d of %d", len(plan.Versions), manifest.Cap)
	}
	if len(plan.Stops) != 0 {
		t.Fatalf("the cap stopped the run: %v", plan.Stops)
	}
	if len(plan.Skips) != 2 {
		t.Fatalf("%d releases were named as trimmed: %v", len(plan.Skips), plan.Skips)
	}
	for _, n := range plan.Skips {
		if n.Condition != Trimmed {
			t.Errorf("a capped release carries condition %q", n.Condition)
		}
	}
}

func TestTheCapIsPerTargetSoASlowLineIsNotPushedOff(t *testing.T) {
	// decisions/version-cap.md. An overall cap lets a fast-publishing target line
	// push a slow one off the end, and from then on a server on the slower line
	// sees a plugin with no installable version at all.
	w := world{}
	var releases []sources.Release
	for i := 1; i <= manifest.Cap+1; i++ {
		releases = append(releases, published(w,
			fmt.Sprintf("2.%d.0-stable", i), 10+i, fmt.Sprintf("2.%d.0.0", i), "10.11.0.0"))
	}
	releases = append(releases, published(w, "1.0.0-stable", 1, "1.0.0.0", "10.10.0.0"))

	plan := Of("a-plugin", releases, w.fetch)

	targets := map[string]int{}
	for _, v := range plan.Versions {
		targets[v.TargetABI]++
	}
	if targets["10.10.0.0"] != 1 {
		t.Fatalf("the older target line kept %d entries: %+v", targets["10.10.0.0"], plan.Versions)
	}
	if targets["10.11.0.0"] != manifest.Cap {
		t.Errorf("the newer target line kept %d entries", targets["10.11.0.0"])
	}
}

func TestAPublishedVersionCarriesEveryFieldTheServerReads(t *testing.T) {
	w := world{}
	release := published(w, "2.0.0-stable", 30, "2.0.0.0", "10.11.0.0")

	plan := Of("a-plugin", []sources.Release{release}, w.fetch)
	if len(plan.Versions) != 1 {
		t.Fatalf("%d versions: %v %v", len(plan.Versions), plan.Skips, plan.Stops)
	}

	got := plan.Versions[0]
	want := manifest.Version{
		Version:   "2.0.0.0",
		Changelog: "A line.",
		TargetABI: "10.11.0.0",
		SourceURL: "https://example.com/2.0.0-stable/a-plugin_2.0.0.0.zip",
		Checksum:  digest,
		Timestamp: "2026-01-04T00:00:00Z",
	}
	if got != want {
		t.Fatalf("the entry is %+v, want %+v", got, want)
	}
}

func TestAPluginThatPublishedNothingStillGetsALine(t *testing.T) {
	// The whole reason the output exists: a manifest that is short because
	// releases were skipped and one that is short because there was nothing to
	// add are the same file.
	report := Report([]Plan{{Plugin: "a-plugin"}})
	if !strings.Contains(report, "a-plugin") {
		t.Fatalf("a plugin with nothing to publish is absent from the output:\n%s", report)
	}
	if !strings.Contains(report, "0 version(s)") {
		t.Fatalf("the output does not say it published nothing:\n%s", report)
	}
}

func TestAVerdictNamesEveryStopRatherThanTheFirst(t *testing.T) {
	plans := []Plan{
		{Plugin: "a-plugin", Stops: []Note{{Plugin: "a-plugin", Release: "2.0", Condition: "no-archive", Detail: "one"}}},
		{Plugin: "another-plugin", Stops: []Note{{Plugin: "another-plugin", Release: "3.0", Condition: "no-archive", Detail: "two"}}},
	}
	err := Judge(plans)
	if err == nil {
		t.Fatal("two stops passed")
	}
	for _, phrase := range []string{"a-plugin", "another-plugin", "2.0", "3.0"} {
		if !strings.Contains(err.Error(), phrase) {
			t.Errorf("the verdict does not name %q: %v", phrase, err)
		}
	}
}

func TestARefusalIsToldFromAReadThatDidNotHappen(t *testing.T) {
	// The distinction the whole package turns on, held directly rather than
	// through a fixture: an error that is not one of the three refusals is a run
	// that could not find out, and it is never reported as a release that cannot
	// be published.
	if _, ok := noteOf("a-plugin", "1.0", errors.New("the network went away")); ok {
		t.Fatal("a transport failure was read as a release that cannot be published")
	}
	if _, ok := noteOf("a-plugin", "1.0", nil); ok {
		t.Fatal("no error at all produced a note")
	}
	if _, ok := noteOf("a-plugin", "1.0", &pairing.Refusal{Reason: pairing.NoArchive, Detail: "none"}); !ok {
		t.Fatal("a pairing refusal was read as a transport failure")
	}
}
