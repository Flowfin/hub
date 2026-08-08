package names

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowfin.dev/hub/internal/sources"
)

// planted is the account the fixtures are written against. The check reads the
// names it refuses out of the declarations rather than out of a list, so a test
// supplies its own set the same way a second organisation would arrive.
var planted = []string{"an-account"}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	return src
}

func TestRefusesTheNameInASourceString(t *testing.T) {
	// The Done-when's first half: a planted account literal in a covered path
	// reds it.
	found, err := CheckGo("hardcoded_in_source.go", fixture(t, "hardcoded_in_source.go"), planted)
	if err != nil {
		t.Fatalf("CheckGo: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("found %d refusals, want 1: %v", len(found), found)
	}
	if found[0].Line == 0 || !strings.Contains(found[0].String(), sources.Dir) {
		t.Fatalf("the refusal does not say where the name comes from instead: %s", found[0])
	}
}

func TestRefusesTheNameInAWorkflowStep(t *testing.T) {
	found := CheckWorkflow("planted.yml",
		fixture(t, "workflow_with_the_name_in_a_step.yml"), planted)
	if len(found) != 1 {
		t.Fatalf("found %d refusals, want 1: %v", len(found), found)
	}
	// Line 11 is the step. Line 3 is a comment citing the same command, and the
	// refusal naming only one of the two is the property.
	if found[0].Line != 11 {
		t.Fatalf("the refusal names line %d; the step is on line 11 and line 3 is a comment", found[0].Line)
	}
}

func TestSparesTheNameInAComment(t *testing.T) {
	// A comment quoting the command that produced a number is what
	// CONTRIBUTING.md requires of an asserted fact. Refusing it would put the two
	// rules against each other.
	found, err := CheckGo("only_in_a_comment.go", fixture(t, "only_in_a_comment.go"), planted)
	if err != nil {
		t.Fatalf("CheckGo: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("a comment was refused: %v", found)
	}

	if found := CheckWorkflow("planted.yml",
		fixture(t, "workflow_with_the_name_only_in_a_comment.yml"), planted); len(found) != 0 {
		t.Fatalf("a workflow comment was refused: %v", found)
	}
}

func TestSparesTheModulePath(t *testing.T) {
	// The organisation appears in the module path as a domain. Refusing it would
	// red every import in the tree, and a check that reds on everything is one
	// somebody turns off.
	found, err := CheckGo("the_module_path.go", fixture(t, "the_module_path.go"), planted)
	if err != nil {
		t.Fatalf("CheckGo: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("the module path was refused: %v", found)
	}
}

func TestTheNameIsMatchedAsAWholeWord(t *testing.T) {
	// A rule matching a substring refuses a plugin whose name happens to contain
	// the account, and a rule matching too little misses the case it is for.
	refused := []string{
		"repos/an-account/hub",
		"AN-ACCOUNT/jellyfin-plugin-sso",
		"https://api.example.com/users/an-account",
	}
	for _, value := range refused {
		if len(find(value, planted)) == 0 {
			t.Errorf("%q was not refused", value)
		}
	}

	spared := []string{
		"an-account.dev/hub",
		"an-accountant/plugin",
		"not-an-account-really",
		"sources",
	}
	for _, value := range spared {
		if hits := find(value, planted); len(hits) != 0 {
			t.Errorf("%q was refused: %v", value, hits)
		}
	}
}

func TestTheNamesComeFromTheDeclarationsRatherThanFromAList(t *testing.T) {
	declarations, err := sources.Load(os.DirFS("../../" + sources.Dir))
	if err != nil {
		t.Fatalf("loading the declared set: %v", err)
	}
	declared, err := Declared(declarations)
	if err != nil {
		t.Fatalf("Declared: %v", err)
	}
	if len(declared) == 0 {
		t.Fatal("no name was read from the declarations")
	}
	// Whatever the account is called today, the check is looking for that and
	// this test does not restate it.
	for _, d := range declarations {
		found := false
		for _, name := range declared {
			if strings.EqualFold(name, d.Account) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s declares an account the check would not look for", d.Slug)
		}
	}
}

func TestAnEmptyNameSetIsAnErrorRatherThanACheckWithNothingToDo(t *testing.T) {
	// A run that refused nothing because it read no names reports the same thing
	// as a clean tree.
	if _, err := Declared(nil); err == nil {
		t.Fatal("a check with no name to look for was allowed")
	}
}

func TestTheTreeItselfPasses(t *testing.T) {
	// The leg, against this repository.
	declarations, err := sources.Load(os.DirFS("../../" + sources.Dir))
	if err != nil {
		t.Fatalf("loading the declared set: %v", err)
	}
	declared, err := Declared(declarations)
	if err != nil {
		t.Fatalf("Declared: %v", err)
	}

	found, err := CheckTree("../..", declared)
	if err != nil {
		t.Fatalf("checking the tree: %v", err)
	}
	if len(found) != 0 {
		var lines []string
		for _, f := range found {
			lines = append(lines, f.String())
		}
		t.Fatalf("a name that belongs in %s/ is written into the tree:\n%s",
			sources.Dir, strings.Join(lines, "\n"))
	}
}

func TestTheSparedDirectoriesAreNotWalked(t *testing.T) {
	// The evidence scope, asserted rather than assumed. Pointed at testdata,
	// which holds files that violate the rule, CheckTree finds nothing.
	found, err := CheckTree("testdata", planted)
	if err != nil {
		t.Fatalf("checking testdata: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("testdata was walked after all: %v", found)
	}
	if len(Spared) == 0 {
		t.Fatal("the check declares no evidence scope")
	}
}
