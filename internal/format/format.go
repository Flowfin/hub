// Package format holds the whitespace rules .editorconfig states and the check
// that refuses a tracked file breaking one.
//
// The rules are here rather than in a workflow step because an editor
// configuration is a request and this is the refusal, and because a refusal
// that cannot be run against a planted input cannot be shown to bite. Nothing
// outside the standard library is needed to decide any of them.
//
// gofmt is not re-implemented here. It ships with the toolchain, it already
// decides Go formatting, and decisions/means.md names it. What this adds is the
// part gofmt does not reach: the HTML, the JSON, the workflow YAML and the
// prose, which are most of the tree.
package format

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// Finding is one broken rule at one place. Line is 1-based, or 0 when the
// finding is about the file as a whole.
type Finding struct {
	Path   string
	Line   int
	Rule   string
	Detail string
}

func (f Finding) String() string {
	if f.Line == 0 {
		return fmt.Sprintf("%s: %s: %s", f.Path, f.Rule, f.Detail)
	}
	return fmt.Sprintf("%s:%d: %s: %s", f.Path, f.Line, f.Rule, f.Detail)
}

// Rule names, which are what a failure prints. They are constants so a message
// and a test cannot drift apart.
const (
	RuleFinalNewline       = "final-newline"
	RuleTrailingWhitespace = "trailing-whitespace"
	RuleTabIndent          = "tab-indent"
)

// CheckFile judges one file's bytes under the rules for its path.
//
// CRLF is normalised to LF before anything is judged, so the verdict does not
// depend on the checkout that produced the bytes. That is deliberate and it is
// the reason this check can run on Windows and on Linux and agree: what is
// stored is fixed by .gitattributes, and a contributor whose working copy has
// CRLF is not told their tree is broken. A lone carriage return is not
// normalised and not judged; nothing in this tree has one.
//
// A file containing a NUL byte is treated as binary and judged not at all,
// because these rules are about text and a false finding on a binary file is
// how a check gets switched off.
func CheckFile(name string, content []byte) []Finding {
	if bytes.IndexByte(content, 0) >= 0 {
		return nil
	}
	content = bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	var findings []Finding
	if len(content) > 0 && content[len(content)-1] != '\n' {
		findings = append(findings, Finding{
			Path:   name,
			Rule:   RuleFinalNewline,
			Detail: "the file does not end with a newline",
		})
	}

	tabsAllowed := tabIndented(name)
	lines := strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t") {
			findings = append(findings, Finding{
				Path:   name,
				Line:   i + 1,
				Rule:   RuleTrailingWhitespace,
				Detail: "the line ends in a space or a tab",
			})
		}
		if !tabsAllowed && strings.HasPrefix(line, "\t") {
			findings = append(findings, Finding{
				Path:   name,
				Line:   i + 1,
				Rule:   RuleTabIndent,
				Detail: "the line is indented with a tab, and .editorconfig indents this file type with spaces",
			})
		}
	}
	return findings
}

// tabIndented says whether .editorconfig indents this path with tabs. Go is the
// only such type in the tree, because gofmt indents with tabs and rewriting it
// would be a fight with the toolchain's own formatter rather than a style.
func tabIndented(name string) bool {
	return filepath.Ext(name) == ".go"
}

// TrackedFiles returns the paths git tracks under root, so an untracked scratch
// file in somebody's working copy never reds the check and every file that will
// reach a reviewer does.
func TrackedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "-C", root, "ls-files", "-z")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing tracked files in %s: %w", root, err)
	}
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// CheckTree judges every tracked file under root. Paths in the findings are the
// slash-separated ones git reports, so a failure reads the same on either
// platform.
func CheckTree(root string) ([]Finding, error) {
	paths, err := TrackedFiles(root)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	for _, p := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", p, err)
		}
		findings = append(findings, CheckFile(path.Clean(p), content)...)
	}
	return findings, nil
}
