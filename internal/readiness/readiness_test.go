// Every condition is tripped here against a planted reading, and each test is
// named for the release it refuses rather than for the function it calls. A
// condition that has never refused anything has not been shown to work, and
// three of the four are conditions this tree is in today, so a suite that only
// watched them clear would be watching them clear for the wrong reason.
package readiness_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowfin.dev/hub/internal/freshness"
	"flowfin.dev/hub/internal/readiness"
	"flowfin.dev/hub/internal/sources"
)

// theDeclarations is the fixture vocabulary, loaded through the real loader so
// that a declaration in a test is the same shape as one in the tree. The
// compiled channel pattern is unexported and is what tells a finished release
// from a test one, so a declaration built as a literal here would answer that
// question by panicking.
func theDeclarations(t *testing.T) []sources.Declaration {
	t.Helper()
	declarations, err := sources.Load(os.DirFS(filepath.Join("testdata", "sources")))
	if err != nil {
		t.Fatalf("reading the fixture declarations: %v", err)
	}
	return declarations
}

func find(t *testing.T, declarations []sources.Declaration, slug string) sources.Declaration {
	t.Helper()
	for _, d := range declarations {
		if d.Slug == slug {
			return d
		}
	}
	t.Fatalf("the fixtures declare no %s", slug)
	return sources.Declaration{}
}

func TestATreeCarryingNoTermsRefusesARelease(t *testing.T) {
	// The file an operator, a packager and a distribution all read first. A
	// release without it is one nobody may redistribute, and they find that out
	// after they have it.
	c := readiness.Licence(t.TempDir())
	if c.Reading != readiness.Blocks {
		t.Fatalf("a tree with no %s read as %s: %s", readiness.LicenceFile, c.Reading, c.Detail)
	}
	if !strings.Contains(c.Detail, readiness.LicenceFile) {
		t.Errorf("the refusal does not name the file that is missing: %s", c.Detail)
	}
}

func TestATreeWhoseTermsAreBlankRefusesARelease(t *testing.T) {
	// The near miss worth spending the fixture on. A file that exists satisfies
	// every check that asks whether it exists, and an empty one grants nothing.
	root := t.TempDir()
	write(t, filepath.Join(root, readiness.LicenceFile), "\n   \n\t\n")

	if c := readiness.Licence(root); c.Reading != readiness.Blocks {
		t.Fatalf("a blank %s read as %s: %s", readiness.LicenceFile, c.Reading, c.Detail)
	}
}

func TestATreeCarryingTermsClearsThatCondition(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, readiness.LicenceFile), "The terms this repository is under.\n")

	if c := readiness.Licence(root); c.Reading != readiness.Clear {
		t.Fatalf("a tree carrying terms read as %s: %s", c.Reading, c.Detail)
	}
}

func TestAnAddressNobodyHasReadRefusesARelease(t *testing.T) {
	// The condition this tree is in today. An instruction to paste an address
	// that answers with nothing shows the operator an empty repository and no
	// error, which is indistinguishable from a mistyped address.
	c := readiness.InstallAddress(nil)
	if c.Reading != readiness.Blocks {
		t.Fatalf("no recorded address read as %s: %s", c.Reading, c.Detail)
	}
	if !strings.Contains(c.Detail, "paste") {
		t.Errorf("the refusal does not say what an operator would be told to do: %s", c.Detail)
	}
}

func TestARecordedAddressClearsThatCondition(t *testing.T) {
	c := readiness.InstallAddress([]string{"https://a.example/manifest.json"})
	if c.Reading != readiness.Clear {
		t.Fatalf("a recorded address read as %s: %s", c.Reading, c.Detail)
	}
	if !strings.Contains(c.Detail, "a.example") {
		t.Errorf("the report does not name the address it read: %s", c.Detail)
	}
}

func TestACatalogueMissingTheNewestReleaseRefusesARelease(t *testing.T) {
	// The failure that is silent on every server: the catalogue is served, it
	// parses, it lists the plugin, and the version an operator is waiting for is
	// not in it. Nothing reports anything.
	expected := []freshness.Expected{{Slug: "one-plugin", Path: "an-account/a-repository", Tag: "1.1.0-stable"}}

	c := readiness.Catalogue("https://a.example/manifest.json", published("1.0.0-stable"), nil, expected)
	if c.Reading != readiness.Blocks {
		t.Fatalf("a stale catalogue read as %s: %s", c.Reading, c.Detail)
	}
	if !strings.Contains(c.Detail, "1.1.0-stable") {
		t.Errorf("the refusal does not name the release that is missing: %s", c.Detail)
	}
}

func TestACurrentCatalogueClearsThatCondition(t *testing.T) {
	expected := []freshness.Expected{{Slug: "one-plugin", Path: "an-account/a-repository", Tag: "1.1.0-stable"}}

	c := readiness.Catalogue("https://a.example/manifest.json", published("1.1.0-stable"), nil, expected)
	if c.Reading != readiness.Clear {
		t.Fatalf("a current catalogue read as %s: %s", c.Reading, c.Detail)
	}
}

func TestAnAddressNothingWasReadFromIsNotACurrentCatalogue(t *testing.T) {
	// Not evaluated rather than blocking, and refusing all the same. The repair
	// for a catalogue that is behind is a publication run; the repair for a
	// catalogue nobody read is to read it.
	c := readiness.Catalogue("", nil, nil, nil)
	if c.Reading != readiness.NotEvaluated {
		t.Fatalf("an unread address read as %s: %s", c.Reading, c.Detail)
	}
}

func TestAReadThatFailedIsNotACurrentCatalogue(t *testing.T) {
	// The case that would be most convenient to treat as a pass, because the
	// likeliest cause is a network blip on a night somebody wants to release.
	c := readiness.Catalogue("https://a.example/manifest.json", nil, errors.New("no route to host"), nil)
	if c.Reading != readiness.NotEvaluated {
		t.Fatalf("a failed read of the address read as %s: %s", c.Reading, c.Detail)
	}
	if !strings.Contains(c.Detail, "no route to host") {
		t.Errorf("the report does not say why the read failed: %s", c.Detail)
	}
}

func TestADeclaredSetThatResolvedNothingRefusesARelease(t *testing.T) {
	// A run that resolved nothing because a credential expired produces exactly
	// the file a correct run over an empty world would produce, so the empty
	// catalogue is the one state where total failure and a true answer are the
	// same bytes.
	declarations := theDeclarations(t)
	resolutions := []sources.Resolution{{
		Declaration: find(t, declarations, "one-plugin"),
		State:       sources.NoReleases,
		Detail:      "a-repository has published nothing",
	}}

	c := readiness.DeclaredSet(resolutions)
	if c.Reading != readiness.Blocks {
		t.Fatalf("a set that resolved nothing read as %s: %s", c.Reading, c.Detail)
	}
}

func TestADeclaredSetWithSomethingToPublishClearsThatCondition(t *testing.T) {
	declarations := theDeclarations(t)
	resolutions := []sources.Resolution{{
		Declaration: find(t, declarations, "one-plugin"),
		State:       sources.Resolved,
		Finished:    1,
		Releases:    []sources.Release{{Tag: "1.1.0-stable"}},
	}}

	c := readiness.DeclaredSet(resolutions)
	if c.Reading != readiness.Clear {
		t.Fatalf("a set with something to publish read as %s: %s", c.Reading, c.Detail)
	}
}

func TestASetNothingWasResolvedFromIsNotAnEmptySet(t *testing.T) {
	// The two are different sentences. A run that resolved nothing has read the
	// world and found it empty; a run holding no resolutions has not read it.
	c := readiness.DeclaredSet(nil)
	if c.Reading != readiness.NotEvaluated {
		t.Fatalf("no resolutions at all read as %s: %s", c.Reading, c.Detail)
	}
}

func TestExpectationsTakeTheNewestFinishedReleaseRatherThanTheNewestOne(t *testing.T) {
	// A test build is newer than the finished one often enough that taking the
	// newest release outright would expect the catalogue to hold something the
	// catalogue is not allowed to publish, and the release would then be refused
	// for the catalogue being correct.
	declarations := theDeclarations(t)
	resolutions := []sources.Resolution{{
		Declaration: find(t, declarations, "one-plugin"),
		State:       sources.Resolved,
		Releases: []sources.Release{
			{Tag: "1.2.0-beta.1"},
			{Tag: "1.1.0-stable"},
			{Tag: "1.0.0-stable"},
		},
	}}

	expected := readiness.Expectations(resolutions)
	if len(expected) != 1 {
		t.Fatalf("expected one expectation, got %d: %+v", len(expected), expected)
	}
	if expected[0].Tag != "1.1.0-stable" {
		t.Errorf("the expectation names %s rather than the newest finished release", expected[0].Tag)
	}
	if expected[0].Path != "an-account/a-repository" {
		t.Errorf("the expectation names %s rather than the declared repository", expected[0].Path)
	}
}

func TestADeclarationThatIsOffIsExpectedOfNothing(t *testing.T) {
	declarations := theDeclarations(t)
	resolutions := []sources.Resolution{{
		Declaration: find(t, declarations, "another-plugin"),
		State:       sources.Disabled,
		Releases:    []sources.Release{{Tag: "1.1.0-stable"}},
	}}

	if expected := readiness.Expectations(resolutions); len(expected) != 0 {
		t.Fatalf("a declaration nobody reads on this run was expected of the catalogue: %+v", expected)
	}
}

func TestAConditionNobodyFilledInRefuses(t *testing.T) {
	// The zero value of a reading is NotEvaluated, and this is what that is for.
	// A condition added to the list and never given an answer would otherwise
	// pass, which is a release cut on a question nobody asked.
	err := readiness.Judge([]readiness.Condition{{Name: "a-condition-somebody-added", Refuses: "nothing yet"}})
	if err == nil {
		t.Fatal("a condition nothing was read about cleared the release")
	}
	if !strings.Contains(err.Error(), "not evaluated") {
		t.Errorf("the refusal does not say the condition was unread: %v", err)
	}
}

func TestNoConditionsAtAllRefuses(t *testing.T) {
	if err := readiness.Judge(nil); err == nil {
		t.Fatal("a run that read nothing about the tree cleared the release")
	}
}

func TestABlockingConditionAndAnUnreadOneAreRefusedInDifferentWords(t *testing.T) {
	err := readiness.Judge([]readiness.Condition{
		{Name: "one-that-holds", Reading: readiness.Blocks, Detail: "it holds"},
		{Name: "one-nobody-read", Reading: readiness.NotEvaluated, Detail: "nothing was read"},
		{Name: "one-that-is-fine", Reading: readiness.Clear, Detail: "clear"},
	})
	if err == nil {
		t.Fatal("a blocking condition cleared the release")
	}
	text := err.Error()
	if !strings.Contains(text, "one-that-holds") || !strings.Contains(text, "one-nobody-read") {
		t.Errorf("the refusal does not name both conditions: %v", err)
	}
	if strings.Contains(text, "one-that-is-fine") {
		t.Errorf("the refusal names a condition that is clear: %v", err)
	}
}

func TestEveryConditionClearIsARunThatRefusesNothing(t *testing.T) {
	err := readiness.Judge([]readiness.Condition{
		{Name: "one", Reading: readiness.Clear},
		{Name: "two", Reading: readiness.Clear},
	})
	if err != nil {
		t.Fatalf("a tree in which every condition is clear was refused: %v", err)
	}
}

func TestATreeWithNoRecordedAddressStillAsksAboutTheCatalogue(t *testing.T) {
	// The near miss this branch exists for. A loop over an empty list runs zero
	// times, so a set assembled without it would carry three conditions instead
	// of four and clear the fourth by never having asked it.
	read := func(string) ([]byte, error) {
		t.Error("something was read although no address is recorded")
		return nil, nil
	}

	conditions := readiness.Conditions(t.TempDir(), nil, nil, read)
	if len(conditions) != 4 {
		t.Fatalf("expected four conditions, got %d: %+v", len(conditions), conditions)
	}
	if err := readiness.Judge(conditions); err == nil {
		t.Fatal("a tree with no terms, no address and nothing resolved cleared a release")
	}
}

func TestEveryRecordedAddressIsReadOnItsOwn(t *testing.T) {
	// One condition per address. A single condition covering both would have to
	// choose which of them to report, and the one it dropped would be the one
	// that had stopped answering.
	var asked []string
	read := func(addr string) ([]byte, error) {
		asked = append(asked, addr)
		return published("1.1.0-stable"), nil
	}

	recorded := []string{"https://a.example/manifest.json", "https://b.example/manifest.json"}
	conditions := readiness.Conditions(t.TempDir(), recorded, nil, read)

	if len(asked) != len(recorded) {
		t.Fatalf("read %d address(es) of %d: %v", len(asked), len(recorded), asked)
	}
	if len(conditions) != 3+len(recorded) {
		t.Fatalf("expected one condition per address beside the three, got %d", len(conditions))
	}
}

func TestTheReportNamesEveryConditionAndWhatHappenedToIt(t *testing.T) {
	// A run that evaluated three conditions of four must not read as a run that
	// evaluated four and found nothing.
	var out strings.Builder
	readiness.Report(&out, []readiness.Condition{
		{Name: "one-that-holds", Refuses: "the thing it refuses", Reading: readiness.Blocks, Detail: "it holds"},
		{Name: "one-that-is-fine", Refuses: "something else", Reading: readiness.Clear, Detail: "clear"},
		{Name: "one-nobody-read", Refuses: "a third thing", Reading: readiness.NotEvaluated, Detail: "nothing was read"},
	})

	text := out.String()
	for _, want := range []string{
		"one-that-holds", "one-that-is-fine", "one-nobody-read",
		"the thing it refuses", "BLOCKS", "clear", "NOT EVALUATED",
		"3 condition(s): 1 clear, 1 blocking, 1 not evaluated.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the report does not carry %q:\n%s", want, text)
		}
	}
}

// published is a manifest body carrying one plugin whose single version entry
// comes from the given tag, in the shape a Jellyfin server reads.
func published(tag string) []byte {
	return []byte(fmt.Sprintf(`[{"guid":"11111111-2222-3333-4444-555555555555","name":"One Plugin","versions":[{"version":"1.1.0.0","targetAbi":"10.10.0.0","sourceUrl":"https://a.example/an-account/a-repository/releases/download/%s/one-plugin.zip"}]}]`, tag))
}

// resolvedTo is one declaration resolved to the releases named, newest first,
// so that Expectations takes the first finished one among them.
func resolvedTo(t *testing.T, slug string, tags ...string) []sources.Resolution {
	t.Helper()
	releases := make([]sources.Release, 0, len(tags))
	for _, tag := range tags {
		releases = append(releases, sources.Release{Tag: tag})
	}
	return []sources.Resolution{{
		Declaration: find(t, theDeclarations(t), slug),
		State:       sources.Resolved,
		Releases:    releases,
	}}
}

func write(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

func TestTheWatchAsksTheCatalogueConditionAndNoOther(t *testing.T) {
	// What the freshness watch runs on a schedule. It is the same condition the
	// release verb refuses on, assembled by the same function, so the two cannot
	// answer differently about one catalogue. Asking a subset is the point: a
	// watch that also refused for a missing licence would go red every night
	// over a state nothing about the world can change.
	read := func(string) ([]byte, error) { return published("1.0.0-stable"), nil }
	conditions := readiness.CatalogueConditions([]string{"https://a.example/manifest.json"},
		resolvedTo(t, "one-plugin", "1.1.0-stable", "1.0.0-stable"), read)

	if len(conditions) != 1 {
		t.Fatalf("the watch asked %d condition(s), want the catalogue and no other: %+v", len(conditions), conditions)
	}
	if conditions[0].Name != "catalogue-not-current" {
		t.Fatalf("the watch asks %s rather than the catalogue condition", conditions[0].Name)
	}
}

func TestACatalogueMadeStaleInAFixtureIsWhatTheWatchRefuses(t *testing.T) {
	// The two days in August, planted: the address answers, the body parses,
	// the plugin is listed, and the release everybody is waiting for is not in
	// it. A watch that asked only whether the address answered would be green
	// here, which is the state this whole route exists against.
	read := func(string) ([]byte, error) { return published("1.0.0-stable"), nil }
	conditions := readiness.CatalogueConditions([]string{"https://a.example/manifest.json"},
		resolvedTo(t, "one-plugin", "1.1.0-stable", "1.0.0-stable"), read)

	err := readiness.Judge(conditions)
	if err == nil {
		t.Fatal("a catalogue missing the newest finished release passed the watch, so every server would go on seeing an old build with nothing said")
	}
	if !strings.Contains(err.Error(), "1.1.0-stable") {
		t.Errorf("the refusal does not name the release that is missing: %v", err)
	}

	// The same fixture with the release present, so the refusal above is one
	// this reading could have avoided rather than one it always makes.
	current := func(string) ([]byte, error) { return published("1.1.0-stable"), nil }
	if err := readiness.Judge(readiness.CatalogueConditions([]string{"https://a.example/manifest.json"},
		resolvedTo(t, "one-plugin", "1.1.0-stable", "1.0.0-stable"), current)); err != nil {
		t.Errorf("a current catalogue was refused by the watch: %v", err)
	}
}

func TestAWatchThatKnowsOfNoAddressRefusesRatherThanPassing(t *testing.T) {
	// The branch the assembly exists for, read from the watch's end. A loop over
	// an empty list runs zero times, and a set with no conditions in it is what
	// Judge calls a tree nothing has been read about.
	read := func(string) ([]byte, error) {
		t.Error("something was read although no address is recorded")
		return nil, nil
	}

	conditions := readiness.CatalogueConditions(nil, nil, read)
	if len(conditions) != 1 {
		t.Fatalf("a watch with no address assembled %d condition(s), so it would report a catalogue it never looked for", len(conditions))
	}
	if err := readiness.Judge(conditions); err == nil {
		t.Fatal("a watch that knows of no address to read passed")
	}
}

func TestTheWatchReadsEveryRecordedAddress(t *testing.T) {
	// One condition per address, the same way the release verb gets them. An
	// address that stopped answering says nothing about its neighbour, and a
	// watch reading only the first would be silent about the second forever.
	var asked []string
	read := func(addr string) ([]byte, error) {
		asked = append(asked, addr)
		return published("1.1.0-stable"), nil
	}

	recorded := []string{"https://a.example/manifest.json", "https://b.example/manifest.json"}
	conditions := readiness.CatalogueConditions(recorded, nil, read)

	if strings.Join(asked, ",") != strings.Join(recorded, ",") {
		t.Fatalf("the watch read %v of %v", asked, recorded)
	}
	if len(conditions) != len(recorded) {
		t.Fatalf("the watch assembled %d condition(s) for %d address(es)", len(conditions), len(recorded))
	}
}
