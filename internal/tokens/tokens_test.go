package tokens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tree reads a file out of the repository this package sits in.
func tree(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(body)
}

// published reads both sides out of the tree.
func published(t *testing.T) (fileSide, pageSide []Declaration, skipped []string) {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", filepath.FromSlash(File)))
	if err != nil {
		t.Fatalf("opening the token file: %v", err)
	}
	defer f.Close()

	fileSide, skipped, err = FromFile(f)
	if err != nil {
		t.Fatalf("reading the token file: %v", err)
	}
	pageSide, err = FromPage(tree(t, Page), Kinds(fileSide))
	if err != nil {
		t.Fatalf("reading the page: %v", err)
	}
	return fileSide, pageSide, skipped
}

// The leg, against what this repository actually publishes.
func TestTheServedPageMatchesTheTokenFile(t *testing.T) {
	fileSide, pageSide, skipped := published(t)

	found, examinedFile, examinedPage, err := Check(fileSide, pageSide)
	if err != nil {
		t.Fatalf("comparing: %v", err)
	}
	if len(found) != 0 {
		var lines []string
		for _, f := range found {
			lines = append(lines, f.String())
		}
		t.Fatalf("the page and the token file disagree:\n%s", strings.Join(lines, "\n"))
	}

	// Stated rather than left to be counted, so a token file that quietly lost
	// a section, or a page that quietly lost its stylesheet, is a failure here
	// rather than a comparison of nothing against nothing reported as a match.
	if examinedFile < 30 || examinedPage < 30 {
		t.Fatalf("compared %d value(s) in the file and %d in the page, and both sides carry more than that",
			examinedFile, examinedPage)
	}
	if len(skipped) == 0 {
		t.Error("no section was reported as carrying no custom property, and the file has some")
	}

	// Printed rather than only asserted on, because half of what this leg
	// delivers is the count it compared and the sentence naming the served page
	// it does not read. The leg passes -v for that reason: go test throws a
	// passing test's log away without it, and this test passes on every run.
	var report strings.Builder
	Report(&report, found, examinedFile, examinedPage, skipped)
	if !strings.Contains(report.String(), Unheld) {
		t.Errorf("the report does not name the served page this check is not the subject of:\n%s", report.String())
	}
	t.Log("\n" + report.String())
}

// The direction that catches a page growing an untracked colour.
func TestAValueInThePageThatIsNotInTheFileIsRefused(t *testing.T) {
	fileSide, _, _ := published(t)
	page := strings.Replace(tree(t, Page), "--sel:#5B9CFF", "--sel:#5B9CFE", 1)
	if page == tree(t, Page) {
		t.Fatal("the planted value changed nothing, so this proves nothing")
	}

	pageSide, err := FromPage(page, Kinds(fileSide))
	if err != nil {
		t.Fatalf("reading the planted page: %v", err)
	}
	found, _, _, err := Check(fileSide, pageSide)
	if err != nil {
		t.Fatalf("comparing: %v", err)
	}

	if !refused(found, "--sel", RuleTracked) {
		t.Errorf("a one-digit change to an accent was not refused:\n%s", lines(found))
	}
}

// A custom property the token file names nothing for. The page grows one of
// these every time somebody needs a colour and does not want to edit two files.
func TestACustomPropertyTheFileDoesNotNameIsRefused(t *testing.T) {
	fileSide, _, _ := published(t)
	page := strings.Replace(tree(t, Page), "--ring:3px;", "--ring:3px; --tint:#123456;", 1)
	if page == tree(t, Page) {
		t.Fatal("the planted property changed nothing, so this proves nothing")
	}

	pageSide, err := FromPage(page, Kinds(fileSide))
	if err != nil {
		t.Fatalf("reading the planted page: %v", err)
	}
	found, _, _, err := Check(fileSide, pageSide)
	if err != nil {
		t.Fatalf("comparing: %v", err)
	}

	if !refused(found, "--tint", RuleKnownVar) {
		t.Errorf("an untracked custom property was not refused:\n%s", lines(found))
	}
}

// The other direction, which is the one a page-only check would miss: the file
// keeps a value nothing renders.
func TestATokenWithNoUseInThePageIsRefused(t *testing.T) {
	_, pageSide, _ := published(t)
	file := strings.Replace(tree(t, File), `"srgb": "#26262E"`, `"srgb": "#26262F"`, 1)
	if file == tree(t, File) {
		t.Fatal("the planted value changed nothing, so this proves nothing")
	}

	fileSide, _, err := FromFile(strings.NewReader(file))
	if err != nil {
		t.Fatalf("reading the planted file: %v", err)
	}
	found, _, _, err := Check(fileSide, pageSide)
	if err != nil {
		t.Fatalf("comparing: %v", err)
	}

	if !refused(found, "--raise2", RuleUsed) {
		t.Errorf("a token value the page never renders was not refused:\n%s", lines(found))
	}
}

// A token the page does not carry at all, which is the shape a value takes when
// somebody adds it to the file and forgets the stylesheet.
func TestATokenThePageNeverDeclaresIsRefused(t *testing.T) {
	_, pageSide, _ := published(t)
	file := strings.Replace(tree(t, File), `"css-var": "--rs"`, `"css-var": "--radius-small"`, 1)
	if file == tree(t, File) {
		t.Fatal("the planted name changed nothing, so this proves nothing")
	}

	fileSide, _, err := FromFile(strings.NewReader(file))
	if err != nil {
		t.Fatalf("reading the planted file: %v", err)
	}
	found, _, _, err := Check(fileSide, pageSide)
	if err != nil {
		t.Fatalf("comparing: %v", err)
	}

	if !refused(found, "--radius-small", RuleUsed) {
		t.Errorf("a token the page never declares was not refused:\n%s", lines(found))
	}
	if !refused(found, "--rs", RuleKnownVar) {
		t.Errorf("the page's now-unnamed property was not refused:\n%s", lines(found))
	}
}

// The context half. Both values are in the file and both are in the page, so a
// comparison of value sets alone passes a page that has swapped the two schemes
// over, and every reader then gets the palette meant for the other one.
func TestTheSchemesSwappedOverIsRefusedEvenThoughEveryValueIsTracked(t *testing.T) {
	fileSide, _, _ := published(t)
	page := tree(t, Page)
	page = strings.Replace(page,
		`:root[data-theme="dark"]{ --ground:#121216;`,
		`:root[data-theme="dark"]{ --ground:#FAFAFA;`, 1)
	pageSide, err := FromPage(page, Kinds(fileSide))
	if err != nil {
		t.Fatalf("reading the planted page: %v", err)
	}
	found, _, _, err := Check(fileSide, pageSide)
	if err != nil {
		t.Fatalf("comparing: %v", err)
	}

	if !refused(found, "--ground", RuleInPlace) {
		t.Errorf("the light ground declared in the dark block was not refused:\n%s", lines(found))
	}
}

// The same, one level in: an accent that belongs to another preset. Every value
// is in the file, so only the context catches it, and the preset it hurts is the
// one whose reader can least compensate.
func TestAnAccentFromAnotherPresetIsRefused(t *testing.T) {
	fileSide, _, _ := published(t)
	page := strings.Replace(tree(t, Page),
		`:root[data-cvd="tritan"]{ --sel:#FF6F7D;`,
		`:root[data-cvd="tritan"]{ --sel:#46B8E8;`, 1)
	if page == tree(t, Page) {
		t.Fatal("the planted accent changed nothing, so this proves nothing")
	}

	pageSide, err := FromPage(page, Kinds(fileSide))
	if err != nil {
		t.Fatalf("reading the planted page: %v", err)
	}
	found, _, _, err := Check(fileSide, pageSide)
	if err != nil {
		t.Fatalf("comparing: %v", err)
	}

	if !refused(found, "--sel", RuleInPlace) {
		t.Errorf("one preset's accent used for another was not refused:\n%s", lines(found))
	}
}

// The normalisation the comparison cannot work without. #fff and #FFFFFF are one
// colour, .07 and 0.07 are one alpha, and 12px and 12 are one length; a byte
// comparison reds on a page that is correct.
func TestTwoSpellingsOfOneValueAreOneValue(t *testing.T) {
	for _, c := range []struct{ a, b string }{
		{"#fff", "#FFFFFF"},
		{"rgba(255,255,255,.07)", "rgba(255, 255, 255, 0.07)"},
		{"#000", "rgb(0,0,0)"},
	} {
		got, err := parseColour(c.a)
		if err != nil {
			t.Fatalf("%s: %v", c.a, err)
		}
		want, err := parseColour(c.b)
		if err != nil {
			t.Fatalf("%s: %v", c.b, err)
		}
		if got != want {
			t.Errorf("%s reads as %s and %s reads as %s, and they are one colour", c.a, got, c.b, want)
		}
	}

	px, err := parseLength("12px")
	if err != nil {
		t.Fatalf("12px: %v", err)
	}
	bare, err := parseLength("12")
	if err != nil {
		t.Fatalf("12: %v", err)
	}
	if px != bare {
		t.Errorf("12px reads as %s and 12 reads as %s", px, bare)
	}

	if canonStack([]string{"Segoe UI", "sans-serif"}) != parseStack(`"Segoe UI", sans-serif`) {
		t.Error("a family name is not the quotes around it")
	}
}

// A colour written where a length belongs must not be classified into
// agreement. The kind comes from the file, so this is a refusal that says what
// it read.
func TestAValueOfTheWrongKindIsARefusalRatherThanAGuess(t *testing.T) {
	fileSide, _, _ := published(t)
	page := strings.Replace(tree(t, Page), "--ring:3px;", "--ring:#5B9CFF;", 1)
	if page == tree(t, Page) {
		t.Fatal("the planted value changed nothing, so this proves nothing")
	}

	if _, err := FromPage(page, Kinds(fileSide)); err == nil {
		t.Error("a colour declared as the ring width was read without complaint")
	}
}

// The page's light query carries selectors that exclude the dark theme, and a
// reader that looked for the attribute anywhere would put them in the wrong
// scheme. This is the one piece of selector syntax the reader understands.
func TestALightBlockExcludingTheDarkThemeIsNotReadAsDark(t *testing.T) {
	got := schemeOf([]string{
		"@media (prefers-color-scheme: light)",
		`:root[data-cvd="protan"]:not([data-theme="dark"])`,
	})
	if got != "light" {
		t.Errorf("the block reads as %s, want light", got)
	}
	if p := presetOf([]string{`:root[data-cvd="protan"]:not([data-theme="dark"])`}); p != "protan" {
		t.Errorf("the preset reads as %s, want protan", p)
	}
}

// A section added to the file later must not be compared against nothing in
// silence. This is the case that makes the reader's ignorance visible.
func TestASectionTheReaderDoesNotKnowIsRefused(t *testing.T) {
	file := strings.Replace(tree(t, File), `  "surface": {`, `  "motion": { "css-var": "--speed" },`+"\n"+`  "surface": {`, 1)
	if file == tree(t, File) {
		t.Fatal("the planted section changed nothing, so this proves nothing")
	}

	_, _, err := FromFile(strings.NewReader(file))
	if err == nil {
		t.Fatal("a section this reader does not know was read without complaint")
	}
	if !strings.Contains(err.Error(), "motion") {
		t.Errorf("the refusal does not name the section: %v", err)
	}
}

// A comment carrying a brace or a colon must not reach the parse. The page's own
// stylesheet opens with one.
func TestACommentIsNotReadAsCSS(t *testing.T) {
	fileSide, _, _ := published(t)
	page := `<style>` + "\n" +
		`/* --ground:#000000; and a stray { */` + "\n" +
		`:root{ --r:12px; }` + "\n" +
		`</style>`

	got, err := FromPage(page, Kinds(fileSide))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if len(got) != 1 || got[0].Var != "--r" {
		t.Errorf("read %d declaration(s) and the first is %+v, want only --r", len(got), got[0])
	}
}

// A second stylesheet would carry values this reader never saw, and a green over
// half a page is the shape of green that means nothing.
func TestASecondStylesheetIsRefusedRatherThanIgnored(t *testing.T) {
	fileSide, _, _ := published(t)
	if _, err := FromPage(`<style>:root{--r:12px;}</style><style>:root{--rs:8px;}</style>`, Kinds(fileSide)); err == nil {
		t.Error("a page with two stylesheets was read as one")
	}
}

func refused(found []Finding, name, rule string) bool {
	for _, f := range found {
		if f.Var == name && f.Rule == rule {
			return true
		}
	}
	return false
}

func lines(found []Finding) string {
	if len(found) == 0 {
		return "  nothing was refused"
	}
	var out []string
	for _, f := range found {
		out = append(out, "  "+f.String())
	}
	return strings.Join(out, "\n")
}
