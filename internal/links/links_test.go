package links

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefusesALinkToAPageThatIsNotThere(t *testing.T) {
	// The bite. The fixture is a page linking to design-system.html in a site
	// that does not carry one, which is the site's real link with the file
	// renamed out from under it.
	found, err := Check(os.DirFS(filepath.Join("testdata", "dangling")))
	if err != nil {
		t.Fatalf("checking the fixture: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("the planted dangling link produced %d refusals, want 1: %v", len(found), found)
	}
	if found[0].Rule != RuleDangling {
		t.Fatalf("refused under %q, want %q", found[0].Rule, RuleDangling)
	}
	if !strings.Contains(found[0].Detail, "design-system.html") {
		t.Fatalf("the refusal does not name what is missing: %s", found[0])
	}
	if found[0].Line != 4 {
		t.Fatalf("the refusal points at line %d, want the line the link is on", found[0].Line)
	}
}

func TestSparesASiteWhoseClaimsAreAllTrue(t *testing.T) {
	// The other direction, and the one that decides whether anybody keeps the
	// leg switched on. The fixture holds a relative link, a link written from
	// the site root, a link carrying a query and a fragment, a bare fragment, a
	// mailbox and an address somewhere else.
	found, err := Check(os.DirFS(filepath.Join("testdata", "whole")))
	if err != nil {
		t.Fatalf("checking the fixture: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("a site whose links resolve was refused: %v", found)
	}
}

func TestASiteWithNoPageIsAnErrorRatherThanACleanOne(t *testing.T) {
	// Nothing found and nothing read produce the same empty slice, and only one
	// of them is a site with no broken links.
	if _, err := Check(os.DirFS(t.TempDir())); err == nil {
		t.Fatal("a run that read no page reported a site with no dangling links")
	}
}

func TestAddressesThisCheckDoesNotDecide(t *testing.T) {
	for _, address := range []string{
		"https://somewhere.test/x",
		"http://somewhere.test/x",
		"//somewhere.test/x",
		"mailto:someone@somewhere.test",
		"tel:+4900000000",
		"data:image/gif;base64,AAAA",
		"#top",
		"",
	} {
		if target, inside := Inside("index.html", address); inside {
			t.Errorf("%q was read as a file in the site, resolving to %q", address, target)
		}
	}
}

func TestWhereAnAddressResolvesTo(t *testing.T) {
	for _, c := range []struct{ from, address, want string }{
		{"index.html", "design-system.html", "design-system.html"},
		{"index.html", "guide/install.html", "guide/install.html"},
		{"guide/install.html", "../index.html", "index.html"},
		{"guide/install.html", "/index.html", "index.html"},
		{"index.html", "guide/install.html?a=1#b", "guide/install.html"},
		{"index.html", "./style.css", "style.css"},
	} {
		got, inside := Inside(c.from, c.address)
		if !inside || got != c.want {
			t.Errorf("%q from %q resolved to %q (inside=%v), want %q", c.address, c.from, got, inside, c.want)
		}
	}
}

func TestALeadingSlashIsTheSiteRootAndNotTheRepositoryRoot(t *testing.T) {
	// Written out on its own because getting it the other way round is silent.
	// The site is served at the root of its own host, so /manifest.json is a
	// file beside the pages. Resolving it against the repository root would
	// point this check at a path nothing tracks, and every root-relative link
	// would be refused for a reason that is not about the link.
	got, inside := Inside("index.html", "/manifest.json")
	if !inside || got != "manifest.json" {
		t.Fatalf("/manifest.json resolved to %q (inside=%v)", got, inside)
	}
}

func TestAnAnchorIsRead(t *testing.T) {
	// The one deliberate difference from internal/site's reader, so that a
	// later reader does not repair it into agreement. That check asks what the
	// page fetches before the visitor does anything, and an anchor fetches
	// nothing. This one asks whether the page's claims are true.
	found := ReferencesIn("index.html", []byte(`<a href="gone.html">go</a>`))
	if len(found) != 1 || found[0].Address != "gone.html" {
		t.Fatalf("the anchor was not read: %v", found)
	}
}

func TestTheServedSitesLinksResolve(t *testing.T) {
	// The leg, against what this repository actually publishes.
	found, err := Check(os.DirFS(filepath.Join("..", "..", Dir)))
	if err != nil {
		t.Fatalf("checking the site: %v", err)
	}
	if len(found) != 0 {
		var lines []string
		for _, f := range found {
			lines = append(lines, f.String())
		}
		t.Fatalf("the published site links to something it does not carry:\n%s", strings.Join(lines, "\n"))
	}
}
