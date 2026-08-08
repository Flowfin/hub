package reach

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// No violating literal appears in this file. Every one of them lives in
// testdata, which the toolchain does not compile, because a test asserting that
// an off-runner address is refused would otherwise contain an off-runner address
// and gate-tests-reach-nothing would refuse its own suite.

func check(t *testing.T, fixture string) []Finding {
	t.Helper()
	path := filepath.Join("testdata", fixture)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	found, err := CheckFile(path, src)
	if err != nil {
		t.Fatalf("checking %s: %v", fixture, err)
	}
	return found
}

// rules returns the distinct rules a fixture was refused under, so a fixture is
// asserted to trip exactly one thing rather than at least one.
func rules(found []Finding) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range found {
		if !seen[f.Rule] {
			seen[f.Rule] = true
			out = append(out, f.Rule)
		}
	}
	return out
}

func wantOneRule(t *testing.T, fixture, rule string) {
	t.Helper()
	found := check(t, fixture)
	got := rules(found)
	if len(got) != 1 || got[0] != rule {
		t.Fatalf("%s was refused under %v, want exactly [%s]\nfindings: %v", fixture, got, rule, found)
	}
	if found[0].Line == 0 {
		t.Fatalf("%s: the refusal names no line", fixture)
	}
	if !strings.Contains(found[0].String(), rule) {
		t.Fatalf("%s: the refusal does not name the rule: %s", fixture, found[0])
	}
}

func TestRefusesAGateTestImportingTheNetwork(t *testing.T) {
	wantOneRule(t, "reaches_the_network_test.go", RuleNetworkImport)
}

func TestRefusesAGateTestNamingAHostOutsideTheRunner(t *testing.T) {
	// The package that carries the reach is one this check allows on purpose, so
	// the address rule is the only thing standing between the gate and a test
	// that calls a real release API.
	wantOneRule(t, "reaches_a_real_host_test.go", RuleOffRunnerAddress)
}

func TestRefusesAGateTestBindingBelowTheReservedPortBoundary(t *testing.T) {
	wantOneRule(t, "binds_a_privileged_port_test.go", RulePrivilegedPort)
}

func TestRefusesAGateTestThatWouldElevate(t *testing.T) {
	// Nothing in this test runs the fixture, and no elevated call was made to
	// produce the refusal. That is the point: the reach is judged, not attempted.
	wantOneRule(t, "elevates_test.go", RuleElevation)
}

func TestRefusesAGateTestThatWantsADisplay(t *testing.T) {
	wantOneRule(t, "needs_a_display_test.go", RuleDisplay)
}

func TestSparesTheHarnessItsOwnRequirements(t *testing.T) {
	// The harness contains exactly the tests this check refuses. A scope that did
	// not spare it would make #21 unbuildable, and a check with no scope at all
	// would be turned off on the day the harness lands, which is worse.
	if found := check(t, "harness_out_of_scope_test.go"); len(found) != 0 {
		t.Fatalf("a harness file was refused: %v", found)
	}
}

func TestSparesAnOrdinaryGateTest(t *testing.T) {
	// A documentation host and a loopback address are values, not reaches. If
	// this reds, the check has started refusing the fixtures the generator's
	// tests are made of, which is how a guard gets switched off.
	if found := check(t, "clean_test.go"); len(found) != 0 {
		t.Fatalf("an ordinary gate test was refused: %v", found)
	}
}

func TestReservedNamesAreNotAReach(t *testing.T) {
	// RFC 2606 and RFC 6761 set these aside so they cannot resolve to a real
	// service, which is what makes them safe in a fixture.
	for _, host := range []string{
		"example.com", "example.net", "example.org",
		"plugins.example", "server.test", "nothing.invalid", "jellyfin.localhost",
	} {
		if got, ok := offRunnerHost("https://" + host + "/manifest.json"); ok {
			t.Errorf("%s was read as a reach (%s)", host, got)
		}
	}
}

func TestAHostThatIsNotReservedIsAReach(t *testing.T) {
	// Built from parts so this file carries no off-runner literal of its own.
	for _, host := range []string{"api." + "github.com", "repo." + "jellyfin.org"} {
		if _, ok := offRunnerHost("https://" + host + "/x"); !ok {
			t.Errorf("%s was read as safe", host)
		}
	}
}

// literals reads one of the two vocabulary tables. They are files rather than
// slices in this source for the reason the check itself demonstrates: a table of
// refusable literals written into a test file is refused by the check under
// test, and building each one from concatenated halves to dodge that would hide
// what the table says.
func literals(t *testing.T, name string) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s is empty, so the table proves nothing", name)
	}
	return out
}

func TestEveryLiteralInTheRefusedTableIsRefused(t *testing.T) {
	for _, value := range literals(t, "refused-literals.txt") {
		if found := judge(value); len(found) == 0 {
			t.Errorf("%q was not refused", value)
		}
	}
}

func TestNoLiteralInTheSparedTableIsRefused(t *testing.T) {
	// The half of the vocabulary that goes wrong: a rule matching a substring of
	// a longer name turns an ordinary plugin into an elevation, and port 0 is how
	// a test asks the kernel for a free high port rather than a privileged bind.
	for _, value := range literals(t, "spared-literals.txt") {
		if found := judge(value); len(found) != 0 {
			t.Errorf("%q was refused: %v", value, found)
		}
	}
}

func TestAnUnparseableFileIsAnErrorRatherThanAPass(t *testing.T) {
	// A checker that quietly reads nothing reports the same thing as a clean
	// tree, and that is the failure mode this whole package exists against.
	if _, err := CheckFile("broken_test.go", []byte("package \n\n func (")); err == nil {
		t.Fatal("an unparseable test file was read as clean")
	}
}

func TestTheTreeItselfPasses(t *testing.T) {
	// The leg, run against this repository. Every gate test in the tree has to
	// satisfy the rule the day it lands, not eventually.
	found, err := CheckTree("../..")
	if err != nil {
		t.Fatalf("checking the tree: %v", err)
	}
	if len(found) != 0 {
		var lines []string
		for _, f := range found {
			lines = append(lines, f.String())
		}
		t.Fatalf("gate tests in this tree reach outside the rule:\n%s", strings.Join(lines, "\n"))
	}
}

func TestCheckTreeReadsTheFixturesWhenItIsPointedAtThem(t *testing.T) {
	// CheckTree skips testdata, which is what lets the violating fixtures exist.
	// Pointed at that directory directly it still finds nothing, and a check that
	// can be aimed at a violation and report none is one nobody should trust, so
	// the skip is asserted rather than assumed.
	found, err := CheckTree("testdata")
	if err != nil {
		t.Fatalf("checking testdata: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("testdata was walked after all: %v", found)
	}
	// The fixtures are reached by name instead, which is what every test above
	// does, so nothing in testdata is unread.
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	fixtures := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_test.go") {
			fixtures++
		}
	}
	if fixtures != 7 {
		t.Fatalf("testdata holds %d fixture test files; the suite names seven", fixtures)
	}
}
