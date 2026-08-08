// Package reach holds gate-tests-reach-nothing, the check
// decisions/headless-and-unelevated.md names and #20 builds.
//
// It refuses a test compiled into the gate that reaches for the network, a
// display, an elevated operation, a second program, or a privileged port. What
// it judges is the reach rather than the result, and that is the whole design:
// a test that actually attempts one of those things fails on a bare runner
// because the thing is absent, and passes on a desktop that happens to have it,
// so judging by outcome is loudest exactly where the rule is already being kept
// and silent where it is being broken. Reading the source instead gives the same
// verdict on both machines.
//
// Elevation is the one that must never be attempted to be measured. On a machine
// with somebody sitting at it, an elevated call is a consent prompt that takes
// the screen, which is why the decision singles it out and why nothing here runs
// one.
package reach

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// The rules, named. A refusal quotes the rule it is under, because
// decisions/headless-and-unelevated.md asks the refusal to name the rule and a
// message saying only that a line is forbidden sends a reader to the wrong file.
const (
	RuleNetworkImport    = "no gate test reaches the network"
	RuleOffRunnerAddress = "no gate test reaches the network"
	RulePrivilegedPort   = "no gate test binds a port below 1024"
	RuleElevation        = "no gate test elevates or needs a second program"
	RuleDisplay          = "no gate test needs a display server"
)

// HarnessTagPrefix marks a test file as belonging to the harness of
// decisions/headless-and-unelevated.md rather than to the gate, and is how the
// scope of this check is drawn.
//
// The decision names the requirements needs-network, needs-browser and
// needs-jellyfin, and hands the exact spelling of the build tags to #21. A Go
// build constraint cannot carry a hyphen, so the spelling has to move; this
// check therefore requires only the prefix and leaves the rest of each name to
// #21. A file whose constraint carries any tag beginning with it is out of
// scope, because the harness contains exactly the tests this check refuses and
// that is by design rather than by accident.
const HarnessTagPrefix = "needs_"

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

// networkImports are the packages whose whole purpose is to leave the runner.
//
// net/http and net/http/httptest are deliberately absent. A test that stands up
// an httptest server and points its own client at it never leaves the runner,
// and refusing the pair would push the generator's tests away from covering the
// layer that talks to a release API at all. What catches the misuse instead is
// the address rule below: an off-runner host written into a gate test is
// refused whichever package carries it. The gap that leaves is stated in the
// package's own tests and in #20: an address assembled from parts at run time is
// not visible to a reader of the source, and this check is a floor rather than a
// proof of absence.
var networkImports = map[string]bool{
	"net":      true,
	"net/rpc":  true,
	"net/smtp": true,
}

// reservedHosts are the suffixes RFC 2606 and RFC 6761 set aside so that they
// cannot resolve to anything real. A URL under one of them in a gate test is a
// fixture value rather than a reach, which is what manifest/manifest_test.go
// already uses one for.
var reservedHosts = []string{
	"example.com", "example.net", "example.org",
	".example", ".invalid", ".test", ".localhost",
}

// loopbackHosts never leave the machine.
var loopbackHosts = map[string]bool{
	"localhost": true,
	"127.0.0.1": true,
	"::1":       true,
	"0.0.0.0":   true,
}

// elevationWords are literals that name an elevated operation or a second
// program that would have to be running. Each one is a whole word in the
// literal, so a plugin called "sudoku" is not an elevation.
var elevationWords = []string{
	"sudo", "runas", "sc.exe", "netsh", "dev-certs",
	"start-process", "docker", "podman", "systemctl", "setcap",
}

// displayWords are literals that only appear when something wants a screen.
var displayWords = []string{
	"display", "wayland_display", "xvfb", "xdg-open",
	"chromedriver", "playwright", "puppeteer",
}

var (
	wordish  = regexp.MustCompile(`[a-z0-9_.-]+`)
	hostPort = regexp.MustCompile(`^[A-Za-z0-9_.\-\[\]:]*:([0-9]{1,5})$`)
)

// InScope reports whether a parsed test file is one the gate compiles.
func InScope(f *ast.File) bool {
	for _, group := range f.Comments {
		for _, c := range group.List {
			if !constraint.IsGoBuild(c.Text) {
				continue
			}
			expr, err := constraint.Parse(c.Text)
			if err != nil {
				continue
			}
			if namesAHarnessTag(expr) {
				return false
			}
		}
	}
	return true
}

func namesAHarnessTag(expr constraint.Expr) bool {
	switch e := expr.(type) {
	case *constraint.TagExpr:
		return strings.HasPrefix(e.Tag, HarnessTagPrefix)
	case *constraint.NotExpr:
		return namesAHarnessTag(e.X)
	case *constraint.AndExpr:
		return namesAHarnessTag(e.X) || namesAHarnessTag(e.Y)
	case *constraint.OrExpr:
		return namesAHarnessTag(e.X) || namesAHarnessTag(e.Y)
	}
	return false
}

// CheckFile reads one test file and returns what it refuses.
//
// An unparseable file is an error rather than a pass, because a check that reads
// nothing and reports nothing is indistinguishable from a clean tree.
func CheckFile(name string, src []byte) ([]Finding, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", name, err)
	}
	if !InScope(file) {
		return nil, nil
	}

	var found []Finding
	at := func(pos token.Pos) int { return fset.Position(pos).Line }

	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if networkImports[path] {
			found = append(found, Finding{
				File: name, Line: at(imp.Pos()), Rule: RuleNetworkImport,
				Detail: fmt.Sprintf("imports %q", path),
			})
		}
	}

	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		for _, f := range judge(value) {
			f.File, f.Line = name, at(lit.Pos())
			found = append(found, f)
		}
		return true
	})

	return found, nil
}

// judge is every rule that reads a single string literal. It is separate from
// the walk so that each rule can be run against a value directly.
func judge(value string) []Finding {
	var found []Finding
	lower := strings.ToLower(value)

	if host, ok := offRunnerHost(value); ok {
		found = append(found, Finding{
			Rule: RuleOffRunnerAddress, Detail: fmt.Sprintf("names the host %s", host),
		})
	}
	if port, ok := privilegedPort(value); ok {
		found = append(found, Finding{
			Rule: RulePrivilegedPort, Detail: fmt.Sprintf("names port %d", port),
		})
	}
	for _, word := range elevationWords {
		if containsWord(lower, word) {
			found = append(found, Finding{
				Rule: RuleElevation, Detail: fmt.Sprintf("names %q", word),
			})
			break
		}
	}
	for _, word := range displayWords {
		if containsWord(lower, word) {
			found = append(found, Finding{
				Rule: RuleDisplay, Detail: fmt.Sprintf("names %q", word),
			})
			break
		}
	}
	return found
}

// offRunnerHost reports the host of an absolute URL that is neither loopback nor
// a name a standards body has set aside as unresolvable.
func offRunnerHost(value string) (string, bool) {
	if !strings.Contains(value, "://") {
		return "", false
	}
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Host == "" {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if loopbackHosts[host] {
		return "", false
	}
	for _, reserved := range reservedHosts {
		if host == reserved || strings.HasSuffix(host, reserved) {
			return "", false
		}
	}
	return host, true
}

// privilegedPort reports a port below 1024 written into an address literal.
// Port 0 is not one: it is how a test asks the kernel for a free high port.
func privilegedPort(value string) (int, bool) {
	m := hostPort.FindStringSubmatch(strings.TrimSpace(value))
	if m == nil {
		return 0, false
	}
	port, err := strconv.Atoi(m[1])
	if err != nil || port == 0 || port >= 1024 {
		return 0, false
	}
	return port, true
}

// containsWord matches a whole lowercase word inside a literal, so that a
// substring of a longer name is not a refusal.
func containsWord(lower, word string) bool {
	for _, w := range wordish.FindAllString(lower, -1) {
		if w == word {
			return true
		}
		// A dotted or hyphenated name such as sc.exe or xdg-open is one word to
		// the reader and several to the splitter, so match it whole as well.
		if strings.Contains(word, ".") || strings.Contains(word, "-") {
			if strings.Contains(lower, word) {
				return true
			}
		}
	}
	return false
}

// CheckTree walks root and checks every Go test file the gate would compile.
//
// The walk is of the filesystem rather than of what git tracks, deliberately:
// `go test ./...` compiles an untracked test file in somebody's clone too, so a
// check whose scope was the tracked set would pass a file the gate then runs.
// testdata is skipped because the toolchain skips it, which is what makes it the
// place this package's own violating fixtures can live.
func CheckTree(root string) ([]Finding, error) {
	var found []Finding
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == ".git" || name == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") {
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
		f, err := CheckFile(rel, src)
		if err != nil {
			return err
		}
		found = append(found, f...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}
