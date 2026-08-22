package format

import (
	"strings"
	"testing"
)

// crlf rewrites LF as CRLF so a test can hand CheckFile the bytes a Windows
// checkout produces. Written as a helper rather than as a literal so no
// carriage return is stored in this file, which .gitattributes fixes as LF.
func crlf(s string) string {
	return strings.ReplaceAll(s, "\n", "\r\n")
}

func rules(findings []Finding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, f.Rule)
	}
	return out
}

func TestCheckFileRefusesAMissingFinalNewline(t *testing.T) {
	findings := CheckFile("docs/index.html", []byte("<p>one line</p>"))
	if got := rules(findings); len(got) != 1 || got[0] != RuleFinalNewline {
		t.Fatalf("rules refused: %v, want exactly [%s]", got, RuleFinalNewline)
	}
	if findings[0].Line != 0 {
		t.Errorf("line %d, want 0: the rule is about the file, not a line", findings[0].Line)
	}
}

func TestCheckFileRefusesTrailingWhitespace(t *testing.T) {
	// A space at the end of the second line, which is what a wrapped paragraph
	// leaves behind and what no diff shows.
	findings := CheckFile("README.md", []byte("first\nsecond \nthird\n"))
	if got := rules(findings); len(got) != 1 || got[0] != RuleTrailingWhitespace {
		t.Fatalf("rules refused: %v, want exactly [%s]", got, RuleTrailingWhitespace)
	}
	if findings[0].Line != 2 {
		t.Errorf("line %d, want 2", findings[0].Line)
	}
}

func TestCheckFileRefusesATabIndentOutsideGo(t *testing.T) {
	findings := CheckFile(".github/workflows/format.yml", []byte("jobs:\n\tformat:\n"))
	if got := rules(findings); len(got) != 1 || got[0] != RuleTabIndent {
		t.Fatalf("rules refused: %v, want exactly [%s]", got, RuleTabIndent)
	}
}

func TestCheckFileAllowsATabIndentInGo(t *testing.T) {
	if findings := CheckFile("manifest/manifest.go", []byte("func f() {\n\treturn\n}\n")); len(findings) != 0 {
		t.Fatalf("refused %v, and gofmt indents Go with tabs", findings)
	}
}

// TestCheckFileGivesTheSameVerdictOnCRLF is the one that decides whether this
// check can be in the gate at all. A formatting rule that reds only on one
// operating system pushes contributors away for a reason they cannot see, so
// the same content judged from a CRLF checkout has to produce the same answer.
func TestCheckFileGivesTheSameVerdictOnCRLF(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"clean.md", "first\nsecond\n"},
		{"trailing.md", "first\nsecond \nthird\n"},
		{"tabbed.yml", "jobs:\n\tformat:\n"},
	}
	for _, c := range cases {
		lf := rules(CheckFile(c.name, []byte(c.content)))
		crlfRules := rules(CheckFile(c.name, []byte(crlf(c.content))))
		if strings.Join(lf, ",") != strings.Join(crlfRules, ",") {
			t.Errorf("%s: LF refused %v and CRLF refused %v", c.name, lf, crlfRules)
		}
	}
}

// TestCheckFileIgnoresBinary keeps a false finding on a binary file from being
// the reason somebody switches the leg off.
func TestCheckFileIgnoresBinary(t *testing.T) {
	if findings := CheckFile("docs/logo.png", []byte("\x89PNG\x00\x1a  ")); len(findings) != 0 {
		t.Fatalf("refused %v on a file containing a NUL byte", findings)
	}
}

func TestCheckFileAllowsAnEmptyFile(t *testing.T) {
	if findings := CheckFile("docs/CNAME", nil); len(findings) != 0 {
		t.Fatalf("refused %v on an empty file", findings)
	}
}

// TestTrackedTreeIsFormatted is the leg itself. It reads what git tracks rather
// than what the working directory happens to hold, so a scratch file nobody
// committed cannot red it.
func TestTrackedTreeIsFormatted(t *testing.T) {
	const root = "../.."
	paths, err := TrackedFiles(root)
	if err != nil {
		t.Fatalf("listing tracked files: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no tracked files found, so this test would pass on an empty tree")
	}

	findings, err := CheckTree(root)
	if err != nil {
		t.Fatalf("checking the tree: %v", err)
	}
	for _, f := range findings {
		t.Errorf("%s", f)
	}
	if len(findings) > 0 {
		t.Logf("%d tracked file(s) checked", len(paths))
	}
}

func TestCheckFileAllowsATabIndentInTheModuleFile(t *testing.T) {
	// The go command writes the require block with tabs and rewrites the file on
	// its own, so a rule refusing that would be undone by the next `go get`
	// anybody ran and the check would be measuring who ran what last.
	mod := "module m\n\ngo 1.25\n\nrequire (\n\tx v1.0.0\n)\n"
	if findings := CheckFile("go.mod", []byte(mod)); len(findings) != 0 {
		t.Fatalf("refused %v, and the go command indents the require block with tabs", findings)
	}
	// The name is matched whole rather than as a substring, so a file that
	// merely carries it is still held to the rule.
	if findings := CheckFile("docs/go.mod.md", []byte("\tindented prose\n")); len(findings) != 1 {
		t.Fatalf("refused %v over a file whose name only contains go.mod, want exactly one finding", findings)
	}
}
