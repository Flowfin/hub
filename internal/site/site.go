// Package site holds site-fetches-nothing-outside, the check
// decisions/data-posture.md asks for and #50 builds.
//
// Two halves, and only the first one is load-bearing. The scan refuses a served
// page that would load anything from a host other than the one serving it. The
// policy check refuses a served page that does not carry a restrictive
// Content-Security-Policy, which is the same rule expressed where a browser
// enforces it for a visitor who never reads this repository.
//
// Nobody adds analytics to a plugin catalogue on purpose. What arrives is a
// webfont, an icon set, an embedded video or a stylesheet from a content
// delivery network, and each of those sends the visitor's address, their user
// agent and the time of the request to somebody else the moment the page opens.
// From the visitor's side there is no difference between that and analytics.
package site

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Dir is what is published, and every file in it is served.
const Dir = "docs"

// Finding is one refusal.
type Finding struct {
	File   string
	Line   int
	Rule   string
	Detail string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: %s (%s)", filepath.ToSlash(f.File), f.Line, f.Detail, f.Rule)
}

const (
	RuleOutsideLoad = "a served page loads nothing from another host"
	RuleNoPolicy    = "a served page carries a restrictive content policy"
)

// loaders are the places a page says "fetch this". Each pattern captures the
// address in its last group.
//
// An href on an anchor is deliberately not among them. A hyperlink is a
// navigation the visitor chooses to follow, and nothing is sent anywhere until
// they do; a load happens before the visitor has done anything at all, which is
// the whole distinction decisions/data-posture.md draws. An href on a link
// element is a load and is matched.
var loaders = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<link\b[^>]*?\bhref\s*=\s*["']([^"']+)["']`),
	regexp.MustCompile(`(?i)\b(?:src|srcset|poster|formaction)\s*=\s*["']([^"']+)["']`),
	regexp.MustCompile(`(?i)<(?:object|embed)\b[^>]*?\bdata\s*=\s*["']([^"']+)["']`),
	regexp.MustCompile(`(?i)@import\s+(?:url\()?\s*["']?([^"');]+)`),
	regexp.MustCompile(`(?i)\burl\(\s*["']?([^"')]+)`),
}

// hasHost reports whether an address names a host other than the page's own.
//
// A scheme-relative address counts, because "//somewhere/font.woff2" is a load
// from somewhere and looks like a path at a glance. A data URI does not: the
// bytes are in the page and no request leaves the machine.
func hasHost(address string) (string, bool) {
	value := strings.TrimSpace(address)
	lower := strings.ToLower(value)

	switch {
	case strings.HasPrefix(lower, "data:"), strings.HasPrefix(lower, "mailto:"),
		strings.HasPrefix(lower, "#"), value == "":
		return "", false
	case strings.HasPrefix(value, "//"):
		return hostOf(value[2:]), true
	}

	if i := strings.Index(value, "://"); i > 0 {
		return hostOf(value[i+3:]), true
	}
	return "", false
}

func hostOf(rest string) string {
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

// CheckPage reads one served page.
func CheckPage(name string, src []byte) []Finding {
	var found []Finding
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")

	for i, line := range lines {
		for _, pattern := range loaders {
			for _, m := range pattern.FindAllStringSubmatch(line, -1) {
				address := m[len(m)-1]
				host, outside := hasHost(address)
				if !outside {
					continue
				}
				found = append(found, Finding{
					File: name, Line: i + 1, Rule: RuleOutsideLoad,
					Detail: fmt.Sprintf("loads from %s", host),
				})
			}
		}
	}

	found = append(found, checkPolicy(name, string(src))...)
	return found
}

var (
	// The content attribute is read with a class matching its own delimiter
	// rather than either quote. A policy is full of single quotes, so a pattern
	// ending at the first quote of either kind captures "default-src " and then
	// refuses every correct page for declaring no default.
	metaPolicy = regexp.MustCompile(`(?is)<meta\b[^>]*?http-equiv\s*=\s*["']Content-Security-Policy["'][^>]*?content\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	schemeHost = regexp.MustCompile(`(?i)(^|\s)(\*|[a-z][a-z0-9+.-]*://|//)`)
)

// checkPolicy refuses a page with no policy, and a policy that is not
// restrictive by default.
//
// A default-allow policy with a list of blocked hosts passes on the day somebody
// adds a host nobody listed, which is the only day it would have mattered. So
// what is required is a default-src of none or self and no host named anywhere
// in the policy.
func checkPolicy(name, src string) []Finding {
	m := metaPolicy.FindStringSubmatch(src)
	if m == nil {
		return []Finding{{
			File: name, Line: 1, Rule: RuleNoPolicy,
			Detail: "carries no Content-Security-Policy element",
		}}
	}

	policy := m[1]
	if policy == "" {
		policy = m[2]
	}
	line := 1 + strings.Count(src[:strings.Index(src, m[0])], "\n")

	var found []Finding
	defaultSrc := ""
	for _, directive := range strings.Split(policy, ";") {
		fields := strings.Fields(directive)
		if len(fields) == 0 {
			continue
		}
		if strings.EqualFold(fields[0], "default-src") {
			defaultSrc = strings.Join(fields[1:], " ")
		}
		if schemeHost.MatchString(" " + strings.Join(fields[1:], " ")) {
			found = append(found, Finding{
				File: name, Line: line, Rule: RuleNoPolicy,
				Detail: fmt.Sprintf("the %s directive names a host", fields[0]),
			})
		}
	}

	switch strings.ToLower(strings.TrimSpace(defaultSrc)) {
	case "'none'", "'self'":
	case "":
		found = append(found, Finding{
			File: name, Line: line, Rule: RuleNoPolicy,
			Detail: "the policy declares no default-src, so anything not named is allowed",
		})
	default:
		found = append(found, Finding{
			File: name, Line: line, Rule: RuleNoPolicy,
			Detail: fmt.Sprintf("default-src is %q rather than 'none' or 'self'", defaultSrc),
		})
	}

	return found
}

// CheckTree reads every served page under root/Dir.
func CheckTree(root string) ([]Finding, error) {
	dir := filepath.Join(root, Dir)
	var found []Finding
	var pages int

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".html") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		pages++
		found = append(found, CheckPage(filepath.ToSlash(rel), src)...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if pages == 0 {
		// A run that read no page reports what a clean site reports.
		return nil, fmt.Errorf("no page found under %s", dir)
	}

	sort.Slice(found, func(i, j int) bool {
		if found[i].File != found[j].File {
			return found[i].File < found[j].File
		}
		return found[i].Line < found[j].Line
	})
	return found, nil
}
