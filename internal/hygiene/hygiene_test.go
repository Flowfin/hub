package hygiene

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func event(t *testing.T, name string) Event {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("opening the fixture: %v", err)
	}
	defer f.Close()
	e, err := Read(f)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return e
}

func judge(t *testing.T, name string) ([]Finding, *Skip) {
	t.Helper()
	return Judge(event(t, name))
}

func only(t *testing.T, name, rule string, tier Tier) Finding {
	t.Helper()
	found, skip := judge(t, name)
	if skip != nil {
		t.Fatalf("%s was skipped rather than judged: %s", name, skip.Why)
	}
	if len(found) != 1 {
		t.Fatalf("%s produced %d refusals, want 1: %v", name, len(found), found)
	}
	if found[0].Rule != rule {
		t.Fatalf("%s was refused under %q, want %q", name, found[0].Rule, rule)
	}
	if found[0].Tier != tier {
		t.Fatalf("%s was refused at the %s tier, want %s", name, found[0].Tier, tier)
	}
	return found[0]
}

func TestSparesARequestThatNamesItsIssueAndSaysWhatItIs(t *testing.T) {
	found, skip := judge(t, "clean.json")
	if skip != nil {
		t.Fatalf("a request that should have been judged was skipped: %s", skip.Why)
	}
	if len(found) != 0 {
		t.Fatalf("a request meeting every rule was refused: %v", found)
	}
}

func TestBlocksARequestThatNamesNoIssue(t *testing.T) {
	only(t, "names_no_issue.json", RuleNamesAnIssue, Blocking)
}

func TestBlocksARequestWithNothingInItsBody(t *testing.T) {
	// The body is whitespace rather than absent, which is the shape that gets
	// through a check comparing against the empty string.
	only(t, "empty_body.json", RuleSaysSomething, Blocking)
}

func TestASizeAndALongTitleAnnotateAndDoNotBlock(t *testing.T) {
	found, skip := judge(t, "large_and_long_titled.json")
	if skip != nil {
		t.Fatalf("skipped rather than judged: %s", skip.Why)
	}
	if len(found) != 2 {
		t.Fatalf("produced %d refusals, want 2: %v", len(found), found)
	}
	if blocked := Blocked(found); len(blocked) != 0 {
		t.Fatalf("an advisory rule blocked the merge: %v", blocked)
	}
}

func TestTheTiersAreNotTheSameThing(t *testing.T) {
	// Deleting the tier from either rule leaves both tests above passing and
	// this one failing, which is the point of it: the tiering is the property
	// #45 asks for and it is invisible to a test that only counts findings.
	if Blocking == Advisory {
		t.Fatal("the two tiers are the same value, so nothing distinguishes a refusal from an annotation")
	}
	blockers := Blocked([]Finding{
		{Rule: RuleNamesAnIssue, Tier: Blocking},
		{Rule: RuleIsReadable, Tier: Advisory},
	})
	if len(blockers) != 1 || blockers[0].Rule != RuleNamesAnIssue {
		t.Fatalf("Blocked returned %v, want the blocking rule alone", blockers)
	}
}

func TestSkipsABranchInAForkAndSaysWhereItLives(t *testing.T) {
	// The fixture breaks two blocking rules. A leg that judged it would red a
	// fork's pull request against conventions its author has had no reason to
	// read.
	found, skip := judge(t, "from_outside.json")
	if skip == nil {
		t.Fatalf("a branch in a fork was judged, and was refused under: %v", found)
	}
	if !strings.Contains(skip.Why, "a-contributor/hub") {
		t.Fatalf("the skip does not say where the branch lives: %s", skip.Why)
	}
}

func TestTheAssociationDecidesNothingInEitherDirection(t *testing.T) {
	// The defect this replaced. author_association is what GitHub could say
	// about the author when the event was written, not a fact about access:
	// one pull request came back CONTRIBUTOR in its event and MEMBER through
	// the API, and the leg skipped every rule for it. Both fixtures below
	// carry the association that would have given the wrong answer.
	if _, skip := judge(t, "clean.json"); skip != nil {
		t.Fatalf("a branch in this repository was skipped over its association: %s", skip.Why)
	}
	if e := event(t, "clean.json"); !e.Inside() {
		t.Fatal("a head and base naming one repository were not read as inside")
	}
	if e := event(t, "from_outside.json"); e.Inside() {
		t.Fatal("a branch in a fork was read as inside because its association said MEMBER")
	}
}

func TestAPayloadNamingNoHeadRepositoryIsSkippedRatherThanJudged(t *testing.T) {
	// Err towards the skip that announces itself. A payload this reader cannot
	// place is not a stranger to be refused, and it is not an insider either.
	_, skip := judge(t, "no_head_repository.json")
	if skip == nil {
		t.Fatal("a payload naming no head repository was judged as though it were from here")
	}
	if !strings.Contains(skip.Why, "names no repository") {
		t.Fatalf("the skip does not say what was missing: %s", skip.Why)
	}
}

func TestSkipsADraft(t *testing.T) {
	_, skip := judge(t, "draft.json")
	if skip == nil {
		t.Fatal("a draft was judged as though it were asking for a merge")
	}
}

func TestSkipsAnEventCarryingNoPullRequest(t *testing.T) {
	_, skip := judge(t, "no_pull_request.json")
	if skip == nil {
		t.Fatal("an event with no pull request in it was judged")
	}
}

func TestEverySkipIsPrintedAndSaysWhichRulesWereNotApplied(t *testing.T) {
	// The half that makes a skip readable as a skip. A run that judged nothing
	// and printed nothing looks exactly like a run that judged everything and
	// was content, and this leg reaches that state on most of its runs.
	for _, name := range []string{"from_outside.json", "draft.json", "no_pull_request.json", "no_head_repository.json"} {
		found, skip := judge(t, name)
		var out strings.Builder
		Report(&out, found, skip, false)
		text := out.String()
		if !strings.Contains(text, "judged nothing") || !strings.Contains(text, skip.Why) {
			t.Fatalf("%s: the report does not carry the reason: %s", name, text)
		}
		for _, rule := range []string{RuleNamesAnIssue, RuleSaysSomething, RuleIsReadable, RuleTitleFits} {
			if !strings.Contains(text, "not applied: "+rule) {
				t.Fatalf("%s: the report does not say that %q went unapplied: %s", name, rule, text)
			}
		}
	}
}

func TestAnAdvisoryFindingIsWrittenAsAnAnnotationInsideAWorkflow(t *testing.T) {
	found, skip := judge(t, "large_and_long_titled.json")
	if skip != nil {
		t.Fatalf("skipped: %s", skip.Why)
	}
	var out strings.Builder
	Report(&out, found, skip, true)
	if n := strings.Count(out.String(), "::warning title="); n != 2 {
		t.Fatalf("wrote %d annotations for 2 advisory findings: %s", n, out.String())
	}
}

func TestARunOutsideAPullRequestIsSkippedAndNotAnError(t *testing.T) {
	// The state this leg is in on every push and on every local run.
	env := map[string]string{"GITHUB_EVENT_NAME": "push"}
	_, skip, err := FromEnvironment(func(k string) string { return env[k] }, nil)
	if err != nil {
		t.Fatalf("a push was an error rather than a skip: %v", err)
	}
	if skip == nil || !strings.Contains(skip.Why, "push") {
		t.Fatalf("the skip does not say what the event was: %+v", skip)
	}
}

func TestAPullRequestWithNoPayloadIsAnErrorRatherThanASkip(t *testing.T) {
	// A runner that changed under this code. Reporting it as the ordinary
	// absence above would turn a broken leg into a quiet green.
	env := map[string]string{"GITHUB_EVENT_NAME": "pull_request"}
	if _, _, err := FromEnvironment(func(k string) string { return env[k] }, nil); err == nil {
		t.Fatal("a pull request with no payload path was read as nothing to judge")
	}
}

func TestReadsThePayloadTheRunnerPointsAt(t *testing.T) {
	env := map[string]string{
		"GITHUB_EVENT_NAME": "pull_request",
		"GITHUB_EVENT_PATH": filepath.Join("testdata", "clean.json"),
	}
	e, skip, err := FromEnvironment(
		func(k string) string { return env[k] },
		func(p string) (io.ReadCloser, error) { return os.Open(p) })
	if err != nil || skip != nil {
		t.Fatalf("reading the payload: err=%v skip=%+v", err, skip)
	}
	if e.PullRequest == nil || e.PullRequest.Number != 41 {
		t.Fatalf("read the wrong payload: %+v", e.PullRequest)
	}
}

func TestThisPullRequestIsHygienic(t *testing.T) {
	// The leg. On a push and on a local run it judges nothing and says so; the
	// leg passes `-v` so that the saying is visible rather than swallowed.
	e, skip, err := FromEnvironment(os.Getenv, func(p string) (io.ReadCloser, error) { return os.Open(p) })
	if err != nil {
		t.Fatalf("finding the event: %v", err)
	}

	var found []Finding
	if skip == nil {
		found, skip = Judge(e)
	}

	var out strings.Builder
	Report(&out, found, skip, os.Getenv("GITHUB_ACTIONS") == "true")
	t.Log("\n" + out.String())

	if blocked := Blocked(found); len(blocked) > 0 {
		var lines []string
		for _, f := range blocked {
			lines = append(lines, f.String())
		}
		t.Fatalf("this pull request is refused by %d blocking rule(s):\n%s",
			len(blocked), strings.Join(lines, "\n"))
	}
}
