// Package keyboard says what a keyboard has to be able to reach on the served
// pages, and keeps that answer apart from the browser that measures it.
//
// The design system promises that everything is operable from the keyboard
// alone. The page is where a reader first meets that promise, so it is the worst
// place for it to be untrue, and it is also the only place in this tree where
// deciding it needs a page rendered and its script executed. That half is the
// file beside this one, behind the needs_browser tag; this half is the
// expectation, and it runs in the gate like anything else.
//
// The expectation is DERIVED FROM THE MARKUP rather than written down. A list of
// controls maintained beside a page stops matching it, and the control that gets
// added without keyboard reach is exactly the one nobody remembers to add to the
// list. So a page growing a button grows this expectation in the same edit, and
// the browser has to find that button reachable or the check reds.
//
// What it does not decide. Whether the focus ring is drawn, whether the label a
// screen reader announces is the text this reader takes, and whether the control
// does anything useful once operated are all outside it. The first needs painted
// pixels, the second needs an accessibility tree, and the third is what the
// browser-side assertions are for.
package keyboard

import (
	"fmt"
	"sort"
	"strings"
)

// Pages are the served pages this package judges, relative to the repository
// root.
//
// Both, and not only the one carrying the script. A landing page with one link
// is the easier of the two to get wrong, because there is nothing on it that
// looks like it needs checking.
var Pages = []string{"docs/index.html", "docs/design-system.html"}

// Control is one thing on a page that a keyboard has to reach.
type Control struct {
	// Tag is the element's tag name, lower case.
	Tag string
	// Label is the text a reader sees on it, with whitespace collapsed. It is
	// the key the rendered page is compared on, because it is the one property
	// a person and a browser agree about without an accessibility tree.
	Label string
}

func (c Control) String() string {
	if c.Label == "" {
		return c.Tag + " with no text on it"
	}
	return c.Tag + " " + quoted(c.Label)
}

func quoted(s string) string { return "\"" + s + "\"" }

// focusable are the tag names a browser puts in the tab order without being
// asked. Anything else needs a tabindex, and carrying one is what puts it in the
// list below.
//
// The set is the natively focusable form controls plus the anchor, which is
// focusable only when it carries an href. A list that included the anchor
// unconditionally would expect focus on every in-page target name and find none.
var focusable = map[string]bool{
	"button": true, "a": true, "input": true, "select": true, "textarea": true,
}

// void are the tags that carry no closing tag, so their label cannot come from
// text between one.
var void = map[string]bool{"input": true}

// Interactive lists the controls a page declares, in document order.
//
// A duplicate label is kept rather than deduplicated. Two controls reading the
// same are two things a keyboard has to reach, and collapsing them here would
// let one of them fall out of the tab order with nothing to say so.
func Interactive(html string) ([]Control, error) {
	var out []Control
	for i := 0; i < len(html); i++ {
		if html[i] != '<' {
			continue
		}
		name, attrs, end, ok := openTag(html, i)
		if !ok {
			continue
		}

		// A disabled control is out of the tab order by the specification, and
		// correctly so, so expecting focus on it would refuse a page that is
		// right. It is read before the tabindex, because a disabled control
		// carrying a positive one is still not focusable.
		if hasWord(attrs, "disabled") {
			i = end
			continue
		}

		tabindex, hasTabindex := attr(attrs, "tabindex")
		switch {
		case hasTabindex && tabindex == "-1":
			// Deliberately out of the tab order. Reachable by script and by a
			// pointer, and this check is about the keyboard. A roving tabindex,
			// which is the pattern a listbox is built with, is made of exactly
			// these.
			i = end
			continue
		case hasTabindex:
		case !focusable[name]:
			i = end
			continue
		case name == "a":
			if _, has := attr(attrs, "href"); !has {
				i = end
				continue
			}
		}

		label, next, err := labelOf(html, name, attrs, end)
		if err != nil {
			return nil, fmt.Errorf("the element <%s> at byte %d: %w", name, i, err)
		}
		out = append(out, Control{Tag: name, Label: label})
		i = next
	}
	return out, nil
}

// openTag reads the opening tag starting at html[i], returning its lower-case
// name, its attribute text and the index of the closing angle bracket.
func openTag(html string, i int) (name, attrs string, end int, ok bool) {
	if i+1 >= len(html) {
		return "", "", 0, false
	}
	if !isNameStart(html[i+1]) {
		return "", "", 0, false
	}
	j := i + 1
	for j < len(html) && isNameByte(html[j]) {
		j++
	}
	shut := strings.IndexByte(html[j:], '>')
	if shut < 0 {
		return "", "", 0, false
	}
	end = j + shut
	return strings.ToLower(html[i+1 : j]), html[j:end], end, true
}

func isNameStart(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func isNameByte(b byte) bool {
	return isNameStart(b) || b >= '0' && b <= '9' || b == '-'
}

// attr reads one attribute out of an opening tag's attribute text. It reads the
// double-quoted form, which is the one .editorconfig and every page in this tree
// use, and treats any other spelling as absent rather than guessing at it.
func attr(attrs, name string) (string, bool) {
	want := name + "=\""
	for i := 0; ; {
		at := strings.Index(attrs[i:], want)
		if at < 0 {
			return "", false
		}
		at += i
		// The match has to start a word, so href does not match data-href.
		if at > 0 && (isNameByte(attrs[at-1]) || attrs[at-1] == '-') {
			i = at + len(want)
			continue
		}
		rest := attrs[at+len(want):]
		shut := strings.IndexByte(rest, '"')
		if shut < 0 {
			return "", false
		}
		return rest[:shut], true
	}
}

// hasWord reports whether an opening tag's attribute text carries a bare
// attribute of that name, in the boolean form HTML writes one.
//
// It matches the whole word rather than a substring, so aria-disabled is not
// read as disabled. The two mean different things: one takes the control out of
// the tab order and the other only says so to a screen reader, and a control
// carrying the second is still something a keyboard reaches.
func hasWord(attrs, name string) bool {
	for _, f := range strings.Fields(attrs) {
		if f == name || strings.HasPrefix(f, name+"=") {
			return true
		}
	}
	return false
}

// labelOf reads the text a reader sees on a control, and returns the index to
// carry on scanning from.
func labelOf(html, name, attrs string, end int) (label string, next int, err error) {
	if aria, ok := attr(attrs, "aria-label"); ok {
		return collapse(aria), end, nil
	}
	if void[name] {
		if v, ok := attr(attrs, "value"); ok {
			return collapse(v), end, nil
		}
		return "", end, nil
	}
	closing := "</" + name
	at := indexFold(html[end:], closing)
	if at < 0 {
		return "", 0, fmt.Errorf("is never closed, so the text on it cannot be read")
	}
	return collapse(stripTags(html[end+1 : end+at])), end + at, nil
}

// indexFold finds needle in s, case-insensitively on the ASCII letters, which is
// what an HTML tag name is.
func indexFold(s, needle string) int {
	return strings.Index(strings.ToLower(s), strings.ToLower(needle))
}

// stripTags removes markup, leaving the text between it.
func stripTags(s string) string {
	var b strings.Builder
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteByte(s[i])
			}
		}
	}
	return b.String()
}

// collapse turns every run of whitespace into one space and trims the ends, so
// a label that wraps in the source is the label a browser reports.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// Missing is what a rendered page failed to put in the tab order: every control
// the markup declares that the browser never gave focus to.
//
// It compares MULTISETS rather than sets, so a page declaring two controls that
// read the same and reaching one of them is a finding rather than a match.
func Missing(declared, reached []Control) []Control {
	have := map[Control]int{}
	for _, c := range reached {
		have[c]++
	}
	var out []Control
	for _, c := range declared {
		if have[c] > 0 {
			have[c]--
			continue
		}
		out = append(out, c)
	}
	return out
}

// Extra is the other direction: focus landing somewhere the markup does not
// declare a control.
//
// It is reported rather than ignored because it is how this check goes quietly
// blind. A page that put a div in the tab order with a tabindex the scan above
// does not read would show up here, and treating that as nothing to say would
// leave the comparison passing on a page it no longer describes.
func Extra(declared, reached []Control) []Control {
	want := map[Control]int{}
	for _, c := range declared {
		want[c]++
	}
	var out []Control
	for _, c := range reached {
		if want[c] > 0 {
			want[c]--
			continue
		}
		out = append(out, c)
	}
	return out
}

// Format renders a list of controls for a failure message, in a stable order.
func Format(cs []Control) string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.String())
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}
