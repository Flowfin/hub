// Package address refuses an install address printed in a tracked file before
// that address answers with the file it promises.
//
// decisions/manifest-address.md states the rule and says a check is owed for it.
// The failure it prevents is not a broken link. An operator pastes one address
// into a Jellyfin server and the server polls it and nothing else; if the
// address does not answer, the server shows an empty repository and reports no
// error, so the operator cannot tell a mistyped address from a project that is
// not ready. A printed address that does not answer is worse than no instruction
// at all.
//
// The check is split the way every network-facing rule here is split.
// Whether a tracked file prints an address that has been recorded as answering
// is a tree fact and is a leg of the merge gate. Whether a recorded address
// still answers needs a request and belongs to the harness, because a merge that
// waited on it would be a merge waiting on a certificate, a DNS record and
// somebody else's uptime.
//
// The Answered list below is what joins the two halves, and it is empty. Adding
// an entry is the act that requires the measurement, and the harness is what
// keeps the entry honest afterwards.
package address

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"flowfin.dev/hub/internal/format"
)

// Answered lists the install addresses that have been read and found to answer
// with the manifest they promise. An address here may appear in a tracked file;
// one that is not here may not.
//
// It is empty, and that is the state of the world rather than a placeholder:
//
//	curl -sS -o /dev/null -w "%{http_code}\n" https://flowfin.dev/
//	200
//	curl -sS -o /dev/null -w "%{http_code}\n" https://flowfin.dev/manifest.json
//	404
//
// Recorded in decisions/manifest-address.md and run 2026-08-08 against the tree
// at 6a98de6. The name resolves and the site answers; the address an operator
// would paste does not, and the decision refuses a rule written against DNS
// alone for exactly that reason.
//
// What adding an entry costs: the request above, its output written where the
// address is argued, and this list changed in the same commit. What it buys is
// that the instruction may then be written, which is the other half of #34.
var Answered = []string{}

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

// RuleUnanswered is the one rule this package refuses under.
const RuleUnanswered = "an install address is printed only once it answers with the file it promises"

// installAddress is the shape of an address a server is given as a plugin
// repository.
//
// The last segment ending in manifest.json rather than being exactly
// manifest.json. This was widened for a second published address, and entry 5 of
// #1 settled on 2026-08-11 that there is only one, so that reason is gone and
// the width is kept for the one that remains: the rule is about what an operator
// does with a string on this project's own host, and a file printing a near miss
// of the real name promises a manifest to whoever pastes it just as loudly as
// the real name would.
var installAddress = regexp.MustCompile(`(?i)https?://([a-z0-9.\-_~%]+)(?::\d+)?((?:/[a-z0-9.\-_~%]*)*?/[a-z0-9.\-_~%]*manifest\.json)`)

// HostFile is where the site's own host is declared, and it is the authority
// for which addresses this rule is about.
const HostFile = "docs/CNAME"

// Hosts reads the host the site is served from.
//
// The scope this fixes is the difference between an instruction and a citation.
// This tree quotes the Jellyfin project's own published catalogue in two
// decision files, as the measurement those decisions were derived from, and that
// address is not an install instruction for this project: nobody pastes it to
// install these plugins, and it answers in any case. A rule that refused it
// would be refusing evidence for being evidence.
//
// Where a third party's address rots, that is a link that stopped answering and
// it is internal/links's, under the harness.
func Hosts(root string) ([]string, error) {
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(HostFile)))
	if err != nil {
		return nil, fmt.Errorf("reading the site's host from %s: %w", HostFile, err)
	}
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, strings.ToLower(line))
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s names no host, so there is nothing to recognise an install address by", HostFile)
	}
	return out, nil
}

// Skipped is what this rule does not read, and each entry is a place where the
// address is argued rather than offered.
//
// `decisions/` is where an address is settled, which means it is where the
// request showing that it does NOT answer is recorded. decisions/manifest-address.md
// says it in its own words: recording a 404 is not the same act as printing an
// install instruction. Three decision files quote the address today for exactly
// that reason, and a rule that refused them would be refusing the evidence it
// depends on. Nobody installs anything from a decision file.
//
// This package is the check itself and carries the address in the comment above
// for the same reason.
//
// What is NOT skipped is everything an operator reads: the README, the served
// pages, the contributor and security documents. That is the surface the rule
// exists for.
var Skipped = []string{
	"decisions/",
	"internal/address/address.go",
}

// CheckFile reads one tracked file, against the hosts the site is served from.
func CheckFile(name string, content []byte, hosts []string) []Finding {
	if skipped(name) {
		return nil
	}

	var found []Finding
	for i, line := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		for _, m := range installAddress.FindAllStringSubmatch(line, -1) {
			printed := m[0]
			if !ours(m[1], hosts) || answered(printed) {
				continue
			}
			found = append(found, Finding{
				File: name, Line: i + 1, Rule: RuleUnanswered,
				Detail: fmt.Sprintf("%s is printed here and is not recorded as answering, so an operator pasting it gets an empty repository and no error", printed),
			})
		}
	}
	return found
}

// skipped matches a whole file by name and a directory by its trailing slash,
// so a scope is written once rather than one file at a time as each new one is
// discovered.
func skipped(name string) bool {
	name = filepath.ToSlash(name)
	for _, s := range Skipped {
		if s == name || (strings.HasSuffix(s, "/") && strings.HasPrefix(name, s)) {
			return true
		}
	}
	return false
}

// ours reports whether the address is on a host this project serves.
func ours(host string, hosts []string) bool {
	host = strings.ToLower(host)
	for _, h := range hosts {
		if host == h {
			return true
		}
	}
	return false
}

func answered(printed string) bool {
	for _, a := range Answered {
		if strings.EqualFold(strings.TrimRight(printed, "/"), strings.TrimRight(a, "/")) {
			return true
		}
	}
	return false
}

// CheckTree reads every tracked file under root.
//
// Every tracked file rather than the served pages alone. The address does its
// damage wherever it is printed, and the file an operator is most likely to
// follow is the README rather than the site.
func CheckTree(root string) ([]Finding, error) {
	hosts, err := Hosts(root)
	if err != nil {
		return nil, err
	}
	paths, err := format.TrackedFiles(root)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no tracked file found under %s", root)
	}

	var found []Finding
	for _, p := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			return nil, err
		}
		found = append(found, CheckFile(p, content, hosts)...)
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].File != found[j].File {
			return found[i].File < found[j].File
		}
		return found[i].Line < found[j].Line
	})
	return found, nil
}
