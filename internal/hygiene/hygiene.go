// Package hygiene judges a pull request rather than the tree it carries.
//
// Every other leg of this gate reads files. This one reads the request: whether
// it names the issue that argued for it, whether it says anything at all, and
// how much a reader is being asked to hold at once.
// decisions/gate-parity.md is where adopting it is argued, adapted from the
// plugin board's own hygiene leg with everything about a solution file, a
// changelog format and a test project dropped, because none of those exists
// here.
//
// The reason it is worth having on this board specifically: what this
// repository publishes is an address that cannot be withdrawn and a manifest
// servers trust. The change that does damage here is small and looks routine,
// and the link between a change and the issue that argued for it is the only
// thing that lets a later reader reconstruct why.
//
// Two properties carry over from the leg it is adapted from and both are here
// on purpose.
//
// The tiers. A blocking rule refuses the merge, and it earns that by being
// decidable with near enough no false positives. An advisory rule annotates and
// changes no verdict, because a gate that reds on a judgement call is a gate
// everybody learns to override, and one override is all it takes for the
// blocking rules to stop being read either.
//
// The skip that announces itself. A run that judged nothing prints why it
// judged nothing, so it can never be mistaken for a run that judged everything
// and was content. There are three ways to reach that state and each has its
// own sentence.
package hygiene

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// Tier says what a rule does when it refuses.
type Tier int

const (
	// Blocking refuses the merge.
	Blocking Tier = iota
	// Advisory annotates and leaves the verdict alone.
	Advisory
)

func (t Tier) String() string {
	if t == Blocking {
		return "blocking"
	}
	return "advisory"
}

// LargeChange is the number of changed lines above which a reader is being
// asked to hold too much at once.
//
// 400 is the figure the corpus this repository's rules were derived from uses,
// and it is advisory here rather than blocking for the reason that corpus gives
// for it: a size is readable only once the work exists, which is the wrong end
// of the plan, and no reading of a diff separates a scope that was planned
// badly from one that had to be that size. The annotation is a prompt to
// re-plan the issue, not a verdict on the change.
const LargeChange = 400

// LongTitle is the number of characters above which a title stops fitting where
// it is read: a log line, a release note, a check-run list.
const LongTitle = 72

// Event is the part of a pull_request payload this package reads.
type Event struct {
	Action      string `json:"action"`
	PullRequest *struct {
		Number            int    `json:"number"`
		Title             string `json:"title"`
		Body              string `json:"body"`
		Draft             bool   `json:"draft"`
		AuthorAssociation string `json:"author_association"`
		Additions         int    `json:"additions"`
		Deletions         int    `json:"deletions"`
		ChangedFiles      int    `json:"changed_files"`
	} `json:"pull_request"`
}

// Inside lists the author associations GitHub gives somebody who belongs to
// this repository. An author outside them is skipped rather than judged.
//
// Not because their work matters less. The rules below are this repository's
// own conventions, an outside contributor has had no reason to read them, and a
// red check is the worst possible way to introduce somebody to a convention.
// The skip is printed for the same reason every skip here is printed.
var Inside = []string{"OWNER", "MEMBER", "COLLABORATOR"}

// Finding is one rule refusing.
type Finding struct {
	Rule   string
	Tier   Tier
	Detail string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s (%s): %s", f.Rule, f.Tier, f.Detail)
}

// Rule names, so that a reader of the output and a reader of the suite are
// looking at the same string.
const (
	RuleNamesAnIssue  = "the request names the issue that argued for it"
	RuleSaysSomething = "the request body says what the change is"
	RuleIsReadable    = "the change is small enough to be read in one sitting"
	RuleTitleFits     = "the title fits where it is read"
)

// issueReference is the shape of a reference to an issue on this tracker. The
// digits have to be there: a bare hash is a heading in Markdown and is the most
// common thing a looser pattern matches by accident.
var issueReference = regexp.MustCompile(`#\d+`)

// Skip is why a run judged nothing.
type Skip struct{ Why string }

// Read decodes an event payload.
func Read(r io.Reader) (Event, error) {
	var e Event
	if err := json.NewDecoder(r).Decode(&e); err != nil {
		return Event{}, fmt.Errorf("reading the event payload: %w", err)
	}
	return e, nil
}

// FromEnvironment finds the event a run was started by.
//
// getenv and open are supplied rather than taken from the process so that the
// suite can put a run in any of these states without changing the environment
// the rest of the tests share.
//
// The two absent cases are separated on purpose. A run outside a pull request
// is the normal state of this leg on a push and locally, and it is not a
// defect. A run that says it is a pull request and hands over no payload is a
// runner that has changed under this code, and reporting that as the normal
// case would hide it.
func FromEnvironment(getenv func(string) string, open func(string) (io.ReadCloser, error)) (Event, *Skip, error) {
	name := getenv("GITHUB_EVENT_NAME")
	if name != "pull_request" && name != "pull_request_target" {
		return Event{}, &Skip{Why: fmt.Sprintf(
			"this run is not a pull request (the event is %q), and every rule below is about one",
			eventName(name))}, nil
	}
	path := getenv("GITHUB_EVENT_PATH")
	if strings.TrimSpace(path) == "" {
		return Event{}, nil, fmt.Errorf("the event is %q and GITHUB_EVENT_PATH is empty, so the payload this leg judges is not where it is supposed to be", name)
	}
	f, err := open(path)
	if err != nil {
		return Event{}, nil, fmt.Errorf("opening the event payload: %w", err)
	}
	defer f.Close()
	e, err := Read(f)
	if err != nil {
		return Event{}, nil, err
	}
	return e, nil, nil
}

func eventName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "not named at all, which is what a run outside a workflow looks like"
	}
	return name
}

// Judge returns the findings, or the reason nothing was judged.
//
// Both are returned rather than one being signalled by an empty other, because
// "no rule refused anything" and "no rule was applied" are the two answers this
// leg most needs to keep apart.
func Judge(e Event) ([]Finding, *Skip) {
	pr := e.PullRequest
	if pr == nil {
		return nil, &Skip{Why: "the event carries no pull request, so there is nothing here to judge"}
	}
	if pr.Draft {
		return nil, &Skip{Why: fmt.Sprintf("pull request %d is a draft, and a draft is somebody thinking out loud rather than asking for a merge", pr.Number)}
	}
	if !inside(pr.AuthorAssociation) {
		return nil, &Skip{Why: fmt.Sprintf(
			"pull request %d is from an author outside this repository (%s), and these are this repository's own conventions rather than rules an outside contributor has had a reason to read",
			pr.Number, association(pr.AuthorAssociation))}
	}

	var found []Finding
	body := strings.TrimSpace(pr.Body)

	if !issueReference.MatchString(pr.Title) && !issueReference.MatchString(body) {
		found = append(found, Finding{
			Rule: RuleNamesAnIssue, Tier: Blocking,
			Detail: "neither the title nor the body carries a #number, so nothing connects this change to the issue that argued for it and a later reader has no way back to the reasoning",
		})
	}
	if body == "" {
		found = append(found, Finding{
			Rule: RuleSaysSomething, Tier: Blocking,
			Detail: "the body is empty, so the only description of this change is its title and whatever the diff says for itself",
		})
	}
	if n := pr.Additions + pr.Deletions; n > LargeChange {
		found = append(found, Finding{
			Rule: RuleIsReadable, Tier: Advisory,
			Detail: fmt.Sprintf("%d lines changed across %d file(s), over %d. Where that is one topic that could not be smaller, it is fine and this line is noise; where it is not, the repair is to re-plan the issue rather than to carve up the diff",
				n, pr.ChangedFiles, LargeChange),
		})
	}
	if n := len([]rune(pr.Title)); n > LongTitle {
		found = append(found, Finding{
			Rule: RuleTitleFits, Tier: Advisory,
			Detail: fmt.Sprintf("the title is %d characters, over %d, so it is cut off in the places it is read back", n, LongTitle),
		})
	}

	sort.SliceStable(found, func(i, j int) bool { return found[i].Tier < found[j].Tier })
	return found, nil
}

func inside(association string) bool {
	for _, a := range Inside {
		if strings.EqualFold(association, a) {
			return true
		}
	}
	return false
}

// association renders an empty association as words rather than as nothing, so
// the skip line never reads as though a field were missing from this sentence.
func association(a string) string {
	if strings.TrimSpace(a) == "" {
		return "the payload names no association"
	}
	return a
}

// Blocked reports whether any finding refuses the merge.
func Blocked(found []Finding) []Finding {
	var out []Finding
	for _, f := range found {
		if f.Tier == Blocking {
			out = append(out, f)
		}
	}
	return out
}

// Report writes what this run did, and it writes something on every path.
//
// annotate turns an advisory finding into a workflow annotation where the run
// is inside one. It is a parameter rather than a check of the environment so
// that the suite reads what would be written without setting a variable the
// rest of the process shares.
func Report(w io.Writer, found []Finding, skip *Skip, annotate bool) {
	if skip != nil {
		fmt.Fprintf(w, "pr-hygiene judged nothing, and this is why: %s\n", skip.Why)
		fmt.Fprintf(w, "no rule below was applied, so this run is not evidence that any of them holds.\n")
		for _, r := range []string{RuleNamesAnIssue, RuleSaysSomething, RuleIsReadable, RuleTitleFits} {
			fmt.Fprintf(w, "  not applied: %s\n", r)
		}
		return
	}

	blocked := Blocked(found)
	fmt.Fprintf(w, "pr-hygiene applied 4 rules and refused under %d of them, %d blocking.\n",
		len(found), len(blocked))
	for _, f := range found {
		if f.Tier == Advisory && annotate {
			fmt.Fprintf(w, "::warning title=%s::%s\n", f.Rule, f.Detail)
			continue
		}
		fmt.Fprintf(w, "  %s\n", f)
	}
	if len(blocked) == 0 {
		fmt.Fprintf(w, "no blocking rule refused, so this leg passes. An advisory line above, if there is one, changes no verdict.\n")
	}
}
