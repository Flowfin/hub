package lang

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

func TestRefusesAPageWithNoHTMLElement(t *testing.T) {
	// The shape both served pages were written in before this landed. It is
	// valid HTML, the parser supplies the element, and the element it supplies
	// carries no attribute anybody wrote, so a reader guesses.
	f := only(t, "has_no_html_element.html", RuleDeclared)
	if !strings.Contains(f.Detail, "no html element") {
		t.Fatalf("the refusal does not say what is missing: %s", f)
	}
}

func TestRefusesAnElementWithNoLangAttribute(t *testing.T) {
	only(t, "declares_nothing.html", RuleDeclared)
}

func TestRefusesAPageLeftBehindInAnotherLanguage(t *testing.T) {
	// The half that a presence check alone would pass. A page still in the
	// language the site used to publish in announces itself correctly and is
	// still the wrong page.
	f := only(t, "declares_another_language.html", RuleTag)
	if !strings.Contains(f.Detail, `"de"`) || !strings.Contains(f.Detail, `"en"`) {
		t.Fatalf("the refusal does not name both languages: %s", f)
	}
}

func TestSparesAPageThatDeclaresTheLanguageTheSitePublishesIn(t *testing.T) {
	if found := page(t, "declares_english.html"); len(found) != 0 {
		t.Fatalf("a page declaring the site's language was refused: %v", found)
	}
}

func TestSparesAPageThatIsMoreSpecificAboutTheSameLanguage(t *testing.T) {
	// A region or a script after the language is somebody being precise, not a
	// second language. Refusing it would push whoever writes the next page
	// towards the least specific tag, which is the wrong direction.
	if found := page(t, "declares_a_region.html"); len(found) != 0 {
		t.Fatalf("en-GB was refused: %v", found)
	}
}

func TestASiteWithNoPageIsAnErrorRatherThanACleanOne(t *testing.T) {
	if _, err := CheckTree(os.DirFS(t.TempDir())); err == nil {
		t.Fatal("a run that read no page reported every page declaring its language")
	}
}

func TestTheServedPagesDeclareTheirLanguage(t *testing.T) {
	// The leg, against what this repository actually publishes.
	found, err := CheckTree(os.DirFS(filepath.Join("..", "..", Dir)))
	if err != nil {
		t.Fatalf("checking the site: %v", err)
	}
	if len(found) != 0 {
		var lines []string
		for _, f := range found {
			lines = append(lines, f.String())
		}
		t.Fatalf("the published site does not say what language it is in:\n%s", strings.Join(lines, "\n"))
	}
}
