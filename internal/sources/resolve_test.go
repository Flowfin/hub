package sources

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

// answers is a Lister made of a fixture. Every classification below is judged
// against it, so no test in this package reaches the network and
// gate-tests-reach-nothing has nothing to refuse here.
type answers struct {
	releases map[string][]Release
	landedOn map[string]string
	failures map[string]error

	// latest is what the second reading of a repository returns, for the tests
	// that plant a disagreement between the two. Where a path is absent from
	// both maps below, the second reading is derived from the list, so a
	// fixture that says nothing about it cannot accidentally contradict itself.
	latest       map[string]string
	latestFailed map[string]error
}

func (a answers) ListReleases(_ context.Context, account, repository string) ([]Release, string, error) {
	path := account + "/" + repository
	if err, ok := a.failures[path]; ok {
		return nil, "", err
	}
	if _, ok := a.releases[path]; !ok {
		return nil, "", ErrNotFound
	}
	landed := path
	if other, ok := a.landedOn[path]; ok {
		landed = other
	}
	return a.releases[path], landed, nil
}

func (a answers) LatestRelease(_ context.Context, account, repository string) (string, error) {
	path := account + "/" + repository
	if err, ok := a.latestFailed[path]; ok {
		return "", err
	}
	if tag, ok := a.latest[path]; ok {
		return tag, nil
	}
	if got := a.releases[path]; len(got) > 0 {
		return got[0].Tag, nil
	}
	return "", ErrNoRelease
}

func declare(t *testing.T, slug, repository string, enabled bool) Declaration {
	t.Helper()
	body := `{
    "account": "an-account",
    "repository": "` + repository + `",
    "slug": "` + slug + `",
    "stable_tags": "^v?[0-9]+\\.[0-9]+$",
    "enabled": ` + map[bool]string{true: "true", false: "false"}[enabled] + `,
    "note": "a fixture"
}
`
	got, err := Load(fstest.MapFS{slug + ".json": &fstest.MapFile{Data: []byte(body)}})
	if err != nil {
		t.Fatalf("building the fixture declaration: %v", err)
	}
	return got[0]
}

func TestAResolutionCarriesBothSidesOfTheSplit(t *testing.T) {
	// decisions/plugin-identity.md takes a plugin's identity from the newest
	// release carrying a descriptor whichever channel it is in, and
	// decisions/channel-model.md publishes only the finished side. A resolution
	// that dropped the test side could answer the second rule and not the first,
	// and on a repository whose descriptors are newer than its finished releases
	// it would report that the plugin has no identity at all.
	d := declare(t, "both", "plugin-both", true)
	lister := answers{releases: map[string][]Release{
		d.Path(): {
			{Tag: "1.4-beta.2", Prerelease: true},
			{Tag: "v1.3"},
			{Tag: "1.3-beta.1", Prerelease: true},
		},
	}}

	got := Resolve(context.Background(), lister, []Declaration{d})[0]
	if len(got.Releases) != 3 {
		t.Fatalf("the resolution carries %d of 3 releases: %+v", len(got.Releases), got.Releases)
	}
	if got.Releases[0].Tag != "1.4-beta.2" {
		t.Errorf("the releases are not in the order they were read: %+v", got.Releases)
	}
	if got.Finished != 1 || got.Test != 2 {
		t.Errorf("the counts are %d finished and %d test", got.Finished, got.Test)
	}

	// The side each one is on stays the declaration's pattern applied to the
	// tag, so a reader asks that rather than reading a second copy of the
	// answer. The prerelease flag beside it is not the same question.
	finished := 0
	for _, rel := range got.Releases {
		if got.Declaration.IsFinished(rel.Tag) {
			finished++
		}
	}
	if finished != got.Finished {
		t.Errorf("%d releases answer the pattern and the count says %d", finished, got.Finished)
	}
}

// TestTheThreeCasesInTheIssueAreDistinguished is the Done-when of #24: a
// declaration file with one resolvable repository, one that does not exist and
// one with no releases, and the run telling all three apart in its output.
func TestTheThreeCasesInTheIssueAreDistinguished(t *testing.T) {
	resolvable := declare(t, "resolvable", "plugin-resolvable", true)
	absent := declare(t, "absent", "plugin-absent", true)
	empty := declare(t, "empty", "plugin-empty", true)

	lister := answers{releases: map[string][]Release{
		resolvable.Path(): {{Tag: "v1.2"}, {Tag: "1.3-beta.1", Prerelease: true}},
		empty.Path():      {},
	}}

	got := Resolve(context.Background(), lister, []Declaration{resolvable, absent, empty})
	want := map[string]State{"resolvable": Resolved, "absent": Unresolvable, "empty": NoReleases}
	for _, r := range got {
		if w := want[r.Declaration.Slug]; r.State != w {
			t.Errorf("%s: state %v, want %v (%s)", r.Declaration.Slug, r.State, w, r.Detail)
		}
	}

	report := Report(got)
	for _, phrase := range []string{
		"absent               does not resolve",
		"empty                no releases",
		"resolvable           resolved",
		"1 of 3 declared plugin(s) resolved",
	} {
		if !strings.Contains(report, phrase) {
			t.Errorf("the report does not say %q:\n%s", phrase, report)
		}
	}

	// The three are told apart in the verdict as well as in the words: one of
	// them stops the run and the other two do not.
	if err := Judge(got); err == nil {
		t.Fatal("a declaration pointing at nothing did not stop the run")
	} else if !strings.Contains(err.Error(), "absent") {
		t.Fatalf("the verdict does not name the declaration that failed: %v", err)
	}
}

func TestANotFoundNamesBothReasonsAndClaimsNeither(t *testing.T) {
	// A repository that was never created and one the credential cannot see both
	// answer with a not-found, and nothing in the response separates them.
	absent := declare(t, "absent", "plugin-absent", true)
	got := Resolve(context.Background(), answers{}, []Declaration{absent})
	detail := got[0].Detail
	if !strings.Contains(detail, "does not exist") || !strings.Contains(detail, "cannot see it") {
		t.Fatalf("the message picks one of the two reasons: %q", detail)
	}
}

func TestAReadThatFailedIsFatalAndIsNotAnEmptyRepository(t *testing.T) {
	// The case that most needs to be fatal: its symptom is a short list, which
	// looks exactly like success.
	broken := declare(t, "broken", "plugin-broken", true)
	lister := answers{failures: map[string]error{broken.Path(): errors.New("429 Too Many Requests")}}

	got := Resolve(context.Background(), lister, []Declaration{broken})
	if got[0].State != Unreadable {
		t.Fatalf("a failed read was classified as %v", got[0].State)
	}
	if !got[0].State.Fatal() {
		t.Fatal("a failed read does not stop the run")
	}
}

func TestAnEmptyListASecondReadingContradictsStopsTheRun(t *testing.T) {
	// The failure this is against arrives as a success. A listing route that
	// answers with no rows for a repository that has releases produces the same
	// bytes as a repository that has published nothing, and the run prints the
	// second reading as a sentence about the plugin.
	quiet := declare(t, "quiet", "plugin-quiet", true)
	lister := answers{
		releases: map[string][]Release{quiet.Path(): {}},
		latest:   map[string]string{quiet.Path(): "v4.2.1"},
	}

	got := Resolve(context.Background(), lister, []Declaration{quiet})
	if got[0].State != Contradicted {
		t.Fatalf("an empty list a second reading disagrees with was classified as %v (%s)", got[0].State, got[0].Detail)
	}
	if !got[0].State.Fatal() {
		t.Fatal("an empty list a second reading disagrees with does not stop the run")
	}
	if !strings.Contains(got[0].Detail, "v4.2.1") {
		t.Fatalf("the message does not name the release the second reading returned: %q", got[0].Detail)
	}
	if err := Judge(got); err == nil {
		t.Fatal("the run was not stopped")
	}
	if report := Report(got); !strings.Contains(report, "1 empty list contradicted") {
		t.Fatalf("the report does not count the state:\n%s", report)
	}
}

func TestAnEmptyListASecondReadingAgreesWithIsStillAnEmptyRepository(t *testing.T) {
	// The ordinary case here rather than an edge, and the guard above may not
	// take it with it. Ten of the twelve declared repositories are in this state
	// and a run that refused it would publish nothing on any day.
	quiet := declare(t, "quiet", "plugin-quiet", true)
	other := declare(t, "other", "plugin-other", true)
	lister := answers{releases: map[string][]Release{
		quiet.Path(): {},
		other.Path(): {{Tag: "v1.0"}},
	}}

	got := Resolve(context.Background(), lister, []Declaration{quiet, other})
	if got[0].State != NoReleases {
		t.Fatalf("a repository both readings call empty was classified as %v (%s)", got[0].State, got[0].Detail)
	}
	if got[0].State.Fatal() {
		t.Fatal("a repository that has published nothing stops the run")
	}
	if err := Judge(got); err != nil {
		t.Fatalf("a set carrying one empty repository did not resolve: %v", err)
	}
}

func TestACorroborationThatFailedIsFatalRatherThanAnEmptyRepository(t *testing.T) {
	// A reading that did not happen corroborates nothing. Keeping the first
	// answer here would put "has published nothing" into the report on the
	// strength of a request that failed, which is the claim from a read that did
	// not happen this run refuses everywhere else.
	quiet := declare(t, "quiet", "plugin-quiet", true)
	lister := answers{
		releases:     map[string][]Release{quiet.Path(): {}},
		latestFailed: map[string]error{quiet.Path(): errors.New("504 Gateway Timeout")},
	}

	got := Resolve(context.Background(), lister, []Declaration{quiet})
	if got[0].State != Unreadable {
		t.Fatalf("a corroboration that failed was classified as %v (%s)", got[0].State, got[0].Detail)
	}
	if !strings.Contains(got[0].Detail, "504 Gateway Timeout") {
		t.Fatalf("the message does not carry what failed: %q", got[0].Detail)
	}
	if err := Judge(got); err == nil {
		t.Fatal("a corroboration that failed did not stop the run")
	}
}

func TestARenamedRepositoryIsNotReadAsACorrectDeclaration(t *testing.T) {
	// The case that is not a failure today and becomes one with no error at all.
	// The request answers, returns releases, and is for a different repository
	// than the one declared.
	moved := declare(t, "moved", "plugin-moved", true)
	lister := answers{
		releases: map[string][]Release{moved.Path(): {{Tag: "v1.0"}}},
		landedOn: map[string]string{moved.Path(): "another-account/plugin-moved"},
	}

	got := Resolve(context.Background(), lister, []Declaration{moved})
	if got[0].State != Redirected {
		t.Fatalf("a declaration answered under another name was classified as %v", got[0].State)
	}
	if !strings.Contains(got[0].Detail, "another-account/plugin-moved") {
		t.Fatalf("the message does not say which path answered: %q", got[0].Detail)
	}
	if err := Judge(got); err == nil {
		t.Fatal("a declaration answered under another name did not stop the run")
	}
}

func TestAPluginWithOnlyTestBuildsIsCountedRatherThanRefused(t *testing.T) {
	// Different from having published nothing, and the output says which.
	betas := declare(t, "betas", "plugin-betas", true)
	lister := answers{releases: map[string][]Release{
		betas.Path(): {{Tag: "1.1-beta.1"}, {Tag: "1.1-beta.2"}},
	}}

	got := Resolve(context.Background(), lister, []Declaration{betas})
	if got[0].State != NoFinishedReleases {
		t.Fatalf("state %v", got[0].State)
	}
	if got[0].State.Fatal() {
		t.Fatal("a plugin that has only published test builds stopped the run")
	}
	if got[0].Test != 2 {
		t.Fatalf("counted %d test builds, want 2", got[0].Test)
	}
}

func TestADisabledRecordIsNotRead(t *testing.T) {
	off := declare(t, "off", "plugin-off", false)
	// The lister would answer not-found for it, which would be fatal if it were
	// asked. A disabled record is not asked.
	got := Resolve(context.Background(), answers{}, []Declaration{off})
	if got[0].State != Disabled {
		t.Fatalf("state %v", got[0].State)
	}
	if err := Judge(got); err == nil || strings.Contains(err.Error(), "not-found") {
		t.Fatalf("a disabled record was read: %v", err)
	}
}

func TestARunThatResolvesNothingIsFatal(t *testing.T) {
	// The longest section of decisions/failure-posture.md. A run whose
	// credentials expired produces exactly the file a correct run over an empty
	// set would produce, so zero is refused rather than published.
	empty := declare(t, "empty", "plugin-empty", true)
	lister := answers{releases: map[string][]Release{empty.Path(): {}}}

	got := Resolve(context.Background(), lister, []Declaration{empty})
	if got[0].State.Fatal() {
		t.Fatal("an empty repository was treated as an error in its own right")
	}
	err := Judge(got)
	if err == nil {
		t.Fatal("a run that resolved nothing was allowed to publish")
	}
	if !strings.Contains(err.Error(), "empty catalogue is a decision") {
		t.Fatalf("the verdict does not say what the alternative is: %v", err)
	}
}

func TestEveryDeclarationAppearsInTheReport(t *testing.T) {
	// A manifest that is short because releases were skipped and one that is
	// short because there was nothing to add are the same file. Only the output
	// tells them apart, so nothing may be missing from it.
	var declarations []Declaration
	for _, slug := range []string{"zeta", "alpha", "mid"} {
		declarations = append(declarations, declare(t, slug, "plugin-"+slug, true))
	}
	lister := answers{releases: map[string][]Release{
		"an-account/plugin-zeta":  {{Tag: "v1.0"}},
		"an-account/plugin-alpha": {},
		"an-account/plugin-mid":   {},
	}}

	report := Report(Resolve(context.Background(), lister, declarations))
	for _, slug := range []string{"alpha", "mid", "zeta"} {
		if !strings.Contains(report, slug) {
			t.Errorf("%s is missing from the report:\n%s", slug, report)
		}
	}
	// Sorted, so two runs over one set produce one order.
	if a, m := strings.Index(report, "alpha"), strings.Index(report, "mid"); a > m {
		t.Errorf("the report is not in slug order:\n%s", report)
	}
}

func TestADisabledDeclarationSaysWhyInEveryReport(t *testing.T) {
	// The count line says one declaration is off and says nothing about why, so
	// a catalogue that is short deliberately reads exactly like one that is
	// short because somebody forgot. The note is the only place the reason is
	// written, and a note nothing prints is a note nobody reads.
	off := declare(t, "off", "plugin-off", false)
	off.Note = "waiting for a finished release that carries a descriptor"
	on := declare(t, "on", "plugin-on", true)
	lister := answers{releases: map[string][]Release{
		"an-account/plugin-on": {{Tag: "v1.0"}},
	}}

	report := Report(Resolve(context.Background(), lister, []Declaration{off, on}))
	if !strings.Contains(report, off.Note) {
		t.Errorf("the report does not say why the disabled declaration is off:\n%s", report)
	}
	// The reason has to sit under the counts rather than anywhere in the text,
	// because a slug appearing in the per-declaration line above would satisfy a
	// looser assertion while the reason went unprinted.
	if !strings.Contains(report, "why each disabled declaration is off:") {
		t.Errorf("the report has no section naming the disabled declarations:\n%s", report)
	}
}

func TestAReportOverNothingDisabledDoesNotOfferTheSection(t *testing.T) {
	// The other direction of the same line. A heading over an empty list trains
	// a reader to skip it, and then it is not read on the run where it carries
	// something.
	on := declare(t, "on", "plugin-on", true)
	lister := answers{releases: map[string][]Release{
		"an-account/plugin-on": {{Tag: "v1.0"}},
	}}
	report := Report(Resolve(context.Background(), lister, []Declaration{on}))
	if strings.Contains(report, "why each disabled declaration is off:") {
		t.Errorf("a run with nothing disabled printed the section anyway:\n%s", report)
	}
}

func TestADisabledDeclarationWithNoNoteSaysThatRatherThanNothing(t *testing.T) {
	// A record that gives no reason is the case the section exists for, so it
	// prints the absence instead of an empty column that reads as tidy.
	off := declare(t, "off", "plugin-off", false)
	off.Note = ""
	on := declare(t, "on", "plugin-on", true)
	lister := answers{releases: map[string][]Release{
		"an-account/plugin-on": {{Tag: "v1.0"}},
	}}
	report := Report(Resolve(context.Background(), lister, []Declaration{off, on}))
	if !strings.Contains(report, "the record gives no note") {
		t.Errorf("a disabled declaration with no note printed nothing about it:\n%s", report)
	}
}
