// Package links reads every reference the served site makes and says which of
// them are facts about this tree.
//
// A link is a claim that something exists somewhere, and the claim rots without
// anything going red. Two kinds of rot, and the split between them is the whole
// design of this package rather than a detail of it.
//
// A reference to a file inside the site is a tree fact. Nothing has to be
// running for it to be decided, so Check decides it and the merge gate runs
// Check under a leg of its own.
//
// A reference to somewhere else needs a request, and
// decisions/headless-and-unelevated.md keeps that out of the gate: a merge
// blocked by somebody else's outage teaches people to ignore red checks. So
// External only sorts the references and never fetches one, and the fetching
// lives in the harness under needs-network, where a red is somebody asking for
// it rather than a merge waiting on it.
//
// The subject is the served site and nothing else. Markdown at the repository
// root carries links too and they rot the same way; they are not the site, they
// are not what an operator reads in a browser, and a leg that quietly covered
// them would report a scope no one declared. Widening this is a change to Dir
// and to the sentence above it, in that order.
package links

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"flowfin.dev/hub/internal/site"
)

// Dir is the subject: what is published, and therefore what is served. It is
// read from internal/site rather than written again here, because two constants
// naming the served directory is one of them being wrong after the first move.
const Dir = site.Dir

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

// RuleDangling is the one rule this package refuses under.
const RuleDangling = "a link in a served page resolves to a file the site carries"

// Reference is one address a page names, wherever it names it.
type Reference struct {
	// File is the page, relative to the served directory.
	File string

	// Line is where the address appears, so a refusal points at a line rather
	// than at a file.
	Line int

	// Address is the address exactly as the page writes it, before any
	// fragment or query is taken off it.
	Address string
}

// references are the attributes a page hangs an address on.
//
// An anchor's href is included here and is deliberately excluded from
// internal/site's list, and the two are right for their own questions. That
// check asks what the page fetches before the visitor has done anything, and a
// hyperlink fetches nothing. This one asks whether the page's claims are true,
// and a hyperlink to a page that is not there is exactly the claim that rots.
var references = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:href|src|poster|formaction|action)\s*=\s*["']([^"']*)["']`),
	regexp.MustCompile(`(?i)<(?:object|embed)\b[^>]*?\bdata\s*=\s*["']([^"']*)["']`),
	regexp.MustCompile(`(?i)@import\s+(?:url\()?\s*["']([^"']*)["']`),
	regexp.MustCompile(`(?i)\burl\(\s*["']?([^"')]*)`),
}

// ReferencesIn reads one page.
func ReferencesIn(name string, src []byte) []Reference {
	var out []Reference
	for i, line := range strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n") {
		for _, pattern := range references {
			for _, m := range pattern.FindAllStringSubmatch(line, -1) {
				address := strings.TrimSpace(m[len(m)-1])
				if address == "" {
					continue
				}
				out = append(out, Reference{File: name, Line: i + 1, Address: address})
			}
		}
	}
	return out
}

// References reads every page of a site, where fsys is rooted at the served
// directory.
//
// It takes a file system rather than a path so that the fixture proving this
// works is a tree of its own. A dangling link committed under docs/ would be
// published to visitors as a broken link for as long as the fixture lived, which
// is a defect in the site in exchange for a proof about the check.
func References(fsys fs.FS) ([]Reference, error) {
	var (
		out   []Reference
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
		out = append(out, ReferencesIn(name, src)...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if pages == 0 {
		// A run that read no page produces the same empty result as a clean
		// site, so it says which of the two it was.
		return nil, fmt.Errorf("no page found in the site")
	}
	return out, nil
}

// Elsewhere reports whether an address leaves this site, and gives it back in
// the form a request would use.
//
// A scheme-relative address counts, because "//somewhere.test/x" is a request to
// somewhere and reads as a path at a glance. It is returned with a scheme in
// front of it, since a fetch cannot be made without one.
func Elsewhere(address string) (string, bool) {
	value := strings.TrimSpace(address)
	lower := strings.ToLower(value)

	switch {
	case strings.HasPrefix(value, "//"):
		return "https:" + value, true
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		return value, true
	}
	return "", false
}

// Inside resolves an address to a path within the site, and says whether it is
// one this check decides at all.
//
// Three shapes are neither inside nor elsewhere, and each is skipped for its own
// reason. An address with a scheme this package does not fetch, mailto and tel
// and data among them, names no file and no host to ask. A bare fragment names
// a place on the page it is written on. And an empty address is nothing.
func Inside(from, address string) (string, bool) {
	value := strings.TrimSpace(address)
	if value == "" || strings.HasPrefix(value, "#") {
		return "", false
	}
	if _, outside := Elsewhere(value); outside {
		return "", false
	}
	// A colon before the first slash is a scheme. Checked in that order because
	// "a:b/c" is a scheme and "a/b:c" is a path with a colon in the name.
	if i := strings.IndexAny(value, ":/"); i >= 0 && value[i] == ':' {
		return "", false
	}

	if i := strings.IndexAny(value, "?#"); i >= 0 {
		value = value[:i]
	}
	if value == "" {
		return "", false
	}

	if strings.HasPrefix(value, "/") {
		// The site is served at the root of its own host, so a leading slash is
		// the served directory and not the repository root. Reading it as the
		// repository root would resolve /manifest.json to a file that is never
		// tracked and always missing.
		return path.Clean(strings.TrimPrefix(value, "/")), true
	}
	return path.Clean(path.Join(path.Dir(from), value)), true
}

// Check refuses every reference inside the site that resolves to nothing.
func Check(fsys fs.FS) ([]Finding, error) {
	found, err := References(fsys)
	if err != nil {
		return nil, err
	}

	var out []Finding
	for _, r := range found {
		target, inside := Inside(r.File, r.Address)
		if !inside {
			continue
		}
		if _, err := fs.Stat(fsys, target); err == nil {
			continue
		}
		out = append(out, Finding{
			File: r.File, Line: r.Line, Rule: RuleDangling,
			Detail: fmt.Sprintf("%q resolves to %s, which the site does not carry", r.Address, target),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}
