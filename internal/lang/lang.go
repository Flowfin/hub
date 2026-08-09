// Package lang refuses a served page that does not say what language it is in.
//
// A page with no language attribute is not a formatting defect. A screen reader
// picks a voice and a pronunciation dictionary from it, and with nothing to read
// it guesses, usually from the user's own locale. The result is English prose
// read out with German phonetics or the other way round, which is not slightly
// worse than the alternative: it is unintelligible, and it is silent for
// everybody who does not use a screen reader, so nothing else in the tree would
// ever notice.
//
// It also refuses a page in a language the site does not publish in.
// decisions/site-language.md settles which one that is, and the point of
// checking it rather than only checking that something is declared is that the
// two pages were written in one language and the site is published in another.
// A page left behind in the old one would otherwise announce itself correctly
// and still be the wrong page.
package lang

import (
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"

	"flowfin.dev/hub/internal/site"
)

// Dir is the subject: the served pages. It is read from internal/site rather
// than written again, so the served directory has one authority.
const Dir = site.Dir

// Tag is the language the site publishes in, as a BCP 47 primary subtag.
//
// English, decided by the maintainer on 2026-08-09 in entry 3 of #1 and recorded
// in decisions/site-language.md. That file is where the reasoning is; this
// constant is what a machine reads, and changing one without the other is what
// this sentence exists against.
const Tag = "en"

// Finding is one refusal.
type Finding struct {
	File   string
	Line   int
	Rule   string
	Detail string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: %s (%s)", f.File, f.Line, f.Detail, f.Rule)
}

const (
	RuleDeclared = "a served page declares the language it is written in"
	RuleTag      = "a served page declares the language the site publishes in"
)

var (
	htmlElement = regexp.MustCompile(`(?is)<html\b[^>]*>`)
	langAttr    = regexp.MustCompile(`(?is)\blang\s*=\s*["']([^"']*)["']`)
)

// CheckPage reads one served page.
//
// The html element is found rather than assumed to be the first line, because
// these pages open with a content policy and a title and let the parser supply
// the element. That is valid HTML and it is exactly the case this refuses: an
// element the parser invented carries no attribute anybody wrote.
func CheckPage(name string, src []byte) []Finding {
	body := string(src)
	loc := htmlElement.FindStringIndex(body)
	if loc == nil {
		return []Finding{{
			File: name, Line: 1, Rule: RuleDeclared,
			Detail: "the page has no html element, so there is nowhere for a language to be declared and a reader guesses one",
		}}
	}

	line := 1 + strings.Count(body[:loc[0]], "\n")
	m := langAttr.FindStringSubmatch(body[loc[0]:loc[1]])
	if m == nil {
		return []Finding{{
			File: name, Line: line, Rule: RuleDeclared,
			Detail: "the html element declares no lang attribute, so a reader guesses the language from its own locale",
		}}
	}

	declared := strings.TrimSpace(m[1])
	if primary(declared) != Tag {
		return []Finding{{
			File: name, Line: line, Rule: RuleTag,
			Detail: fmt.Sprintf("the page declares %q and the site publishes in %q", declared, Tag),
		}}
	}
	return nil
}

// primary is the first subtag, lower-cased. A region or a script after it is
// somebody being more specific about the same language, which is not a defect,
// so en-GB passes and de does not.
func primary(tag string) string {
	if i := strings.IndexAny(tag, "-_"); i >= 0 {
		tag = tag[:i]
	}
	return strings.ToLower(tag)
}

// CheckTree reads every served page, where fsys is rooted at the served
// directory.
func CheckTree(fsys fs.FS) ([]Finding, error) {
	var (
		found []Finding
		pages int
	)
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".html") {
			return nil
		}
		src, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		pages++
		found = append(found, CheckPage(name, src)...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if pages == 0 {
		return nil, fmt.Errorf("no page found in the site")
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].File != found[j].File {
			return found[i].File < found[j].File
		}
		return found[i].Line < found[j].Line
	})
	return found, nil
}
