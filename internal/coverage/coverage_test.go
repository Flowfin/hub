package coverage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	return string(body)
}

func TestRefusesAPackageNobodyTested(t *testing.T) {
	// The case the floor exists for. A package with no test file fails nothing
	// and prints no percentage, so a reader who only looks for a low number
	// never sees it.
	err := Judge(fixture(t, "a-package-nobody-tested.txt"), Floor)
	if err == nil {
		t.Fatal("a package with no test file was not refused")
	}
	if !strings.Contains(err.Error(), "internal/newthing") {
		t.Fatalf("the refusal does not name the package: %v", err)
	}
	if !strings.Contains(err.Error(), "no test file") {
		t.Fatalf("the refusal does not say what is missing: %v", err)
	}
}

func TestRefusesAPackageUnderTheFloor(t *testing.T) {
	err := Judge(fixture(t, "a-package-under-the-floor.txt"), Floor)
	if err == nil {
		t.Fatal("a package under the floor was not refused")
	}
	if !strings.Contains(err.Error(), "internal/thin") || !strings.Contains(err.Error(), "12.5%") {
		t.Fatalf("the refusal does not name the package and its number: %v", err)
	}
	if strings.Contains(err.Error(), "internal/reader") {
		t.Fatalf("a package above the floor was named in the refusal: %v", err)
	}
}

func TestSparesATreeWhereEveryPackageIsAboveTheFloor(t *testing.T) {
	if err := Judge(fixture(t, "every-package-above-the-floor.txt"), Floor); err != nil {
		t.Fatalf("a tree above the floor was refused: %v", err)
	}
}

func TestARunThatWasNotAskedForCoverageIsARefusal(t *testing.T) {
	// The half that keeps this leg from going quiet. Drop the flag and every
	// line still says ok, so a reader of the exit status alone sees the same
	// green as a run that measured everything.
	err := Judge(fixture(t, "the-run-was-not-asked-for-coverage.txt"), Floor)
	if err == nil {
		t.Fatal("a run carrying no coverage numbers reported every package above the floor")
	}
	if !strings.Contains(err.Error(), "judged nothing") {
		t.Fatalf("the refusal does not say that nothing was judged: %v", err)
	}
}

func TestAFailingPackageIsNotThisLegsToReport(t *testing.T) {
	// The test leg already refused it. A second leg naming the same package
	// sends the reader to the wrong file.
	const summary = "--- FAIL: TestSomething (0.00s)\nFAIL\texample.test/tree/internal/broken\t0.4s\nok  \texample.test/tree\t1.9s\tcoverage: 48.1% of statements\n"
	packages := Read(summary)
	if len(packages) != 1 || packages[0].Path != "example.test/tree" {
		t.Fatalf("read %v, want the one package that reported coverage", packages)
	}
}

func TestTheFloorIsBelowWhatTheTreeHolds(t *testing.T) {
	// The floor is a floor and not a target, and this is the sentence in
	// executable form: if somebody raises it to the current numbers, this says
	// so before the leg starts refusing packages nobody changed.
	if Floor > 45 {
		t.Fatalf("the floor is %.1f%%, which is at or above the lowest package measured when it was set; raising it is a decision to record in decisions/gate-parity.md rather than a number to nudge", Floor)
	}
}

func TestRefusesAPackageNobodyTestedInTheShapeTheFlagPrints(t *testing.T) {
	// The shape that matters, and the one a reader expecting `ok` and `?` skips
	// entirely. Asking for coverage changes how a package with no test file is
	// printed: no status word, an indented path, and a zero percentage. A
	// reader that missed it would let exactly the package this leg exists for
	// through, and the run would be green.
	err := Judge(fixture(t, "a-package-nobody-tested-under-cover.txt"), Floor)
	if err == nil {
		t.Fatal("a package printed with no status word and 0.0% was not refused")
	}
	if !strings.Contains(err.Error(), "internal/newthing") {
		t.Fatalf("the refusal does not name the package: %v", err)
	}
	if !strings.Contains(err.Error(), "no test file") {
		t.Fatalf("the refusal does not say what is missing: %v", err)
	}

	// The two packages that are fine are read and are not named in the refusal,
	// so the count in it is a count of something.
	if got := len(Read(fixture(t, "a-package-nobody-tested-under-cover.txt"))); got != 3 {
		t.Fatalf("read %d package(s), want 3", got)
	}
	if strings.Contains(err.Error(), "internal/reader") {
		t.Fatalf("a package above the floor was named in the refusal: %v", err)
	}
}
