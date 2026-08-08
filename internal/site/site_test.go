package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func page(t *testing.T, name string) []Finding {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	return CheckPage(name, src)
}

func only(t *testing.T, name, rule string) Finding {
	t.Helper()
	found := page(t, name)
	if len(found) != 1 {
		t.Fatalf("%s produced %d refusals, want 1: %v", name, len(found), found)
	}
	if found[0].Rule != rule {
		t.Fatalf("%s was refused under %q, want %q", name, found[0].Rule, rule)
	}
	return found[0]
}

func TestRefusesAWebfontFromSomewhereElse(t *testing.T) {
	// The way it actually arrives. Nobody adds a tracker; somebody adds a font.
	f := only(t, "webfont.html", RuleOutsideLoad)
	if !strings.Contains(f.Detail, "fonts.example-not-reserved.test") {
		t.Fatalf("the refusal does not name the host: %s", f)
	}
}

func TestRefusesASchemeRelativeLoad(t *testing.T) {
	// Looks like a path at a glance and is a request to somebody else.
	only(t, "scheme_relative.html", RuleOutsideLoad)
}

func TestRefusesAStylesheetPullingAFontThroughUrl(t *testing.T) {
	// The reference that is not an attribute, so a scan reading only attributes
	// would pass this page.
	only(t, "css_url.html", RuleOutsideLoad)
}

func TestRefusesAPageWithNoPolicy(t *testing.T) {
	f := only(t, "no_policy.html", RuleNoPolicy)
	if !strings.Contains(f.Detail, "carries no") {
		t.Fatalf("the refusal does not say what is missing: %s", f)
	}
}

func TestRefusesAPolicyThatIsNotRestrictiveByDefault(t *testing.T) {
	// A policy naming hosts, with no default, passes on the day somebody adds a
	// host nobody listed, which is the only day it would have mattered.
	found := page(t, "blocklist_policy.html")
	if len(found) < 2 {
		t.Fatalf("a blocklist policy produced %d refusals: %v", len(found), found)
	}
	var namesHost, noDefault bool
	for _, f := range found {
		if f.Rule != RuleNoPolicy {
			t.Fatalf("unexpected rule: %s", f)
		}
		namesHost = namesHost || strings.Contains(f.Detail, "names a host")
		noDefault = noDefault || strings.Contains(f.Detail, "no default-src")
	}
	if !namesHost || !noDefault {
		t.Fatalf("both halves were not refused: %v", found)
	}
}

func TestSparesAPageThatLoadsOnlyItsOwnThings(t *testing.T) {
	// The direction that matters as much as the other one. This fixture holds a
	// relative stylesheet, an inline image and an anchor to another site. A check
	// refusing any of those would be one somebody turns off.
	if found := page(t, "clean.html"); len(found) != 0 {
		t.Fatalf("an ordinary page was refused: %v", found)
	}
}

func TestAnAnchorIsNotALoad(t *testing.T) {
	// Written out on its own because it is the judgement in this check most
	// likely to be read as an oversight. Nothing is sent anywhere until the
	// visitor clicks, and the posture is about what happens before they have done
	// anything at all.
	const anchor = `<a href="https://somewhere.test/page">go</a>`
	src := []byte("<meta http-equiv=\"Content-Security-Policy\" content=\"default-src 'none'\">\n" + anchor)
	if found := CheckPage("anchor.html", src); len(found) != 0 {
		t.Fatalf("an anchor was refused as a load: %v", found)
	}

	// The same address in a loading position is refused, so the distinction is
	// the position rather than the host.
	src = []byte("<meta http-equiv=\"Content-Security-Policy\" content=\"default-src 'none'\">\n" +
		`<link rel="stylesheet" href="https://somewhere.test/page">`)
	if found := CheckPage("link.html", src); len(found) != 1 {
		t.Fatalf("the same address in a link element produced %d refusals: %v", len(found), found)
	}
}

func TestADataURIIsNotAnOutsideHost(t *testing.T) {
	for _, address := range []string{"data:image/gif;base64,AAAA", "mailto:someone@somewhere.test", "#top", "local.css", "/manifest.json"} {
		if host, outside := hasHost(address); outside {
			t.Errorf("%q was read as a load from %q", address, host)
		}
	}
	for _, address := range []string{"https://somewhere.test/x", "//somewhere.test/x", "HTTP://Somewhere.Test/x"} {
		if _, outside := hasHost(address); !outside {
			t.Errorf("%q was read as local", address)
		}
	}
}

func TestTheServedPagesFetchNothingFromAnybodyElse(t *testing.T) {
	// The leg, against what this repository actually publishes.
	found, err := CheckTree("../..")
	if err != nil {
		t.Fatalf("checking the site: %v", err)
	}
	if len(found) != 0 {
		var lines []string
		for _, f := range found {
			lines = append(lines, f.String())
		}
		t.Fatalf("the published site would fetch from somewhere else:\n%s", strings.Join(lines, "\n"))
	}
}

func TestAnEmptySiteIsAnErrorRatherThanACleanOne(t *testing.T) {
	if _, err := CheckTree(t.TempDir()); err == nil {
		t.Fatal("a run that read no page reported a clean site")
	}
}
