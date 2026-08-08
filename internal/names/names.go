// Package names holds no-hardcoded-names, the check
// decisions/names-are-data.md names and #30 builds.
//
// It refuses an account or organisation name written into generator source or
// into the logic of a workflow step. Both names come from the declaration files
// and from nowhere else, so a name reaching the generator arrives as a value it
// read, and pointing the tool at a second organisation is editing one directory
// rather than auditing the tree.
//
// The names it refuses are read from the declarations rather than written here.
// A check carrying its own copy of the name would be the thing it exists to
// refuse, and it would go stale on the same day everything else did.
package names

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"flowfin.dev/hub/internal/sources"
)

// Finding is one refusal.
type Finding struct {
	File   string
	Line   int
	Name   string
	Detail string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: %s (%s)", filepath.ToSlash(f.File), f.Line, f.Detail, f.Name)
}

// Declared is the set of names the declarations carry, which is the set this
// check refuses everywhere else.
//
// It is derived rather than listed, so adding a plugin under a second account
// widens the check by itself. An empty result is an error rather than a check
// with nothing to look for: a run that refused nothing because it read no names
// reports the same thing as a clean tree.
func Declared(declarations []sources.Declaration) ([]string, error) {
	seen := map[string]bool{}
	for _, d := range declarations {
		if d.Account != "" {
			seen[strings.ToLower(d.Account)] = true
		}
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("the declarations carry no account name, so this check has nothing to refuse")
	}

	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// word is what counts as one word.
//
// A dot is inside a word rather than between two, and that is the whole of why
// the module path is not a violation: `flowfin.dev/hub/internal/gate` is the
// words `flowfin.dev`, `hub`, `internal` and `gate`, none of which is an account
// name, while `repos/Flowfin/hub` is `repos`, `Flowfin` and `hub`, and the middle
// one is. A tokeniser splitting on dots would refuse every import in the tree and
// would have to be turned off.
var word = regexp.MustCompile(`[A-Za-z0-9_.-]+`)

// find reports every declared name appearing as a whole word in a value.
func find(value string, declared []string) []string {
	var hit []string
	for _, w := range word.FindAllString(value, -1) {
		lower := strings.ToLower(w)
		for _, name := range declared {
			if lower == name {
				hit = append(hit, name)
			}
		}
	}
	return hit
}

// CheckGo reads one Go file and refuses a declared name in a string literal.
//
// Comments are not read, and that is required rather than convenient.
// decisions/names-are-data.md refuses a name in generator source and in the
// logic of a workflow step, and CONTRIBUTING.md requires an asserted fact to
// carry the command that produced it. Several commands in this tree address a
// repository by name, so a check that refused them would be a check against
// citing evidence, and the two rules would have to be traded off against each
// other rather than both kept.
func CheckGo(name string, src []byte, declared []string) ([]Finding, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", name, err)
	}

	var found []Finding
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		for _, hit := range find(value, declared) {
			found = append(found, Finding{
				File: name, Line: fset.Position(lit.Pos()).Line, Name: hit,
				Detail: fmt.Sprintf("the name %q is written into a string here; it comes from %s/", hit, sources.Dir),
			})
		}
		return true
	})
	return found, nil
}

// CheckWorkflow reads one workflow file and refuses a declared name outside a
// comment.
//
// The comment rule is the same one, for the same reason: the comment at the top
// of a workflow file is where it says what it does, and several of them cite a
// command. What is refused is the step's logic.
func CheckWorkflow(name string, src []byte, declared []string) []Finding {
	var found []Finding
	for i, line := range strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n") {
		code := withoutComment(line)
		for _, hit := range find(code, declared) {
			found = append(found, Finding{
				File: name, Line: i + 1, Name: hit,
				Detail: fmt.Sprintf("the name %q is written into a step here; it comes from %s/", hit, sources.Dir),
			})
		}
	}
	return found
}

// withoutComment drops a YAML comment.
//
// It cuts at a hash that starts a line or follows a space, which is the shape
// every comment in this tree has. A hash inside a quoted value would be cut too;
// that is a false negative rather than a false positive, and it is stated rather
// than left for somebody to find.
func withoutComment(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, "#") {
		return ""
	}
	if i := strings.Index(line, " #"); i >= 0 {
		return line[:i]
	}
	return line
}

// Spared are the directories this check does not read, and naming them is part
// of the check rather than an exception carved out of it.
//
// sources is where the names belong. testdata is where a fixture deliberately
// names a fixture, which decisions/names-are-data.md separates from a test
// asserting on the real account. decisions and docs are prose rather than
// generator source or step logic, and the decision that states this rule spells
// the name out itself while arguing for it:
//
//	grep -o -i 'Flowfin' decisions/names-are-data.md | wc -l
//	3
var Spared = []string{sources.Dir, "testdata", "decisions", "docs", ".git"}

// CheckTree walks root and checks everything in scope.
func CheckTree(root string, declared []string) ([]Finding, error) {
	var found []Finding
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			for _, skip := range Spared {
				if d.Name() == skip {
					return fs.SkipDir
				}
			}
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)

		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		switch {
		case strings.HasSuffix(d.Name(), ".go"):
			f, err := CheckGo(rel, src, declared)
			if err != nil {
				return err
			}
			found = append(found, f...)
		case strings.HasPrefix(rel, ".github/workflows/") &&
			(strings.HasSuffix(d.Name(), ".yml") || strings.HasSuffix(d.Name(), ".yaml")):
			found = append(found, CheckWorkflow(rel, src, declared)...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}
