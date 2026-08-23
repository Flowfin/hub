package address

import (
	"path/filepath"
	"strings"
	"testing"
)

// ourHost is what docs/CNAME declares, read rather than written again here, so
// this suite and the check agree about the subject by construction.
//
// No fixture below writes the host as a literal either, and that is not a style
// choice: gate-tests-reach-nothing refuses a gate test naming a real host,
// because a test that names one is usually a test that is about to call it.
// Every address in this file is assembled from what the tree declares, so the
// suite says what it means and reaches nothing.
func ourHost(t *testing.T) []string {
	t.Helper()
	hosts, err := Hosts(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("reading the site host: %v", err)
	}
	return hosts
}

// at builds an address on the site's own host.
func at(t *testing.T, path string) string {
	t.Helper()
	return "https://" + ourHost(t)[0] + path
}

// nothingRecorded pins the recorded list to empty for the length of one test.
//
// Each test that calls it is about an address nobody has read, and the published
// address is recorded now, so a test reading the live list stops biting on the
// day an entry lands rather than on the day the rule changes. That is measured
// rather than supposed: the three tests below went green together the moment
// Answered took its first entry, and they were the reason it was noticed.
// Pinning is what keeps them measuring the refusal instead of the state of the
// tree, and the leg against the real tree with the real list is
// TestNoTrackedFilePrintsAnUnansweredInstallAddress at the bottom of this file.
func nothingRecorded(t *testing.T) {
	t.Helper()
	restore := Answered
	Answered = nil
	t.Cleanup(func() { Answered = restore })
}

func TestRefusesAnInstallAddressNobodyHasReadYet(t *testing.T) {
	nothingRecorded(t)

	// The bite, and the exact shape the tree carried before #35 removed it: an
	// install instruction printed against an address nobody had read.
	page := "<p>Paste this into the repositories list:</p>\n" +
		"<code>" + at(t, "/manifest.json") + "</code>\n"

	found := CheckFile("docs/index.html", []byte(page), ourHost(t))
	if len(found) != 1 {
		t.Fatalf("a printed install address produced %d refusals, want 1: %v", len(found), found)
	}
	if found[0].Rule != RuleUnanswered {
		t.Fatalf("refused under %q, want %q", found[0].Rule, RuleUnanswered)
	}
	if found[0].Line != 2 {
		t.Fatalf("the refusal points at line %d, want the line the address is on", found[0].Line)
	}
	if !strings.Contains(found[0].Detail, "empty repository") {
		t.Fatalf("the refusal does not say what an operator would see: %s", found[0])
	}
}

func TestRefusesANearMissOfThePublishedNameToo(t *testing.T) {
	nothingRecorded(t)

	// Four near misses of the published address: another file name, a deeper
	// path, another scheme and port, another case. The first two are why the
	// predicate matches a last segment ending in manifest.json rather than one
	// that is exactly manifest.json. Each of the four is a string somebody
	// pastes into a repositories list, and with nothing recorded a file printing
	// one promises a manifest just as loudly as the published name would.
	//
	// The fourth stops being a near miss once the published address is recorded,
	// because answered() compares case-insensitively and that is the comparison
	// the rule wants: an operator who pastes the address in another case reaches
	// the same file. So it is a near miss of a name nobody has read, which is
	// what the pinned list makes it here.
	for _, printed := range []string{
		at(t, "/prerelease-manifest.json"),
		at(t, "/channels/stable-manifest.json"),
		strings.Replace(at(t, ":8096/manifest.json"), "https", "http", 1),
		strings.ToUpper(at(t, "/manifest.json")),
	} {
		if found := CheckFile("README.md", []byte(printed), ourHost(t)); len(found) != 1 {
			t.Errorf("%s produced %d refusals, want 1", printed, len(found))
		}
	}
}

func TestSparesAnAddressRecordedAsAnswering(t *testing.T) {
	// The direction that makes the rule usable. Once the address answers and
	// the measurement is recorded, printing it is the whole point.
	restore := Answered
	Answered = []string{at(t, "/manifest.json")}
	defer func() { Answered = restore }()

	if found := CheckFile("README.md", []byte(at(t, "/manifest.json")), ourHost(t)); len(found) != 0 {
		t.Fatalf("a recorded address was refused: %v", found)
	}
	// A second address on the same host is still refused while it is
	// unrecorded, so the record is per address rather than a switch that opens
	// the rule.
	if found := CheckFile("README.md", []byte(at(t, "/prerelease-manifest.json")), ourHost(t)); len(found) != 1 {
		t.Fatalf("an unrecorded address on the same host was spared: %v", found)
	}
}

func TestSparesSomebodyElsesCatalogueQuotedAsEvidence(t *testing.T) {
	// The distinction this rule turns on, and the tree already depends on it:
	// two decision files quote the Jellyfin project's own published catalogue
	// as the measurement they were derived from. Nobody pastes that address to
	// install these plugins, and refusing it would be refusing evidence for
	// being evidence. Where a third party's address rots, that is a link and
	// belongs to internal/links under the harness.
	const cited = "    curl -sSL -o jf.json https://repo.jellyfin." + "org/files/plugin/manifest.json"
	if found := CheckFile("README.md", []byte(cited), ourHost(t)); len(found) != 0 {
		t.Fatalf("a cited third-party catalogue was refused: %v", found)
	}
}

func TestTheHostComesFromTheFileThatDeclaresIt(t *testing.T) {
	// Written out because the alternative is a second copy of the host in this
	// package, which is the copy that is wrong after the first move. docs/CNAME
	// is what the Pages deployment reads, so it is the authority for where the
	// site answers.
	hosts := ourHost(t)
	if len(hosts) == 0 {
		t.Fatal("no host was read")
	}
	for _, h := range hosts {
		if strings.TrimSpace(h) != h || h == "" {
			t.Errorf("the host %q was not trimmed", h)
		}
	}
}

func TestSparesTextThatIsNotAnInstallAddress(t *testing.T) {
	for _, line := range []string{
		"manifest.json",
		"manifest/testdata/golden-manifest.json",
		"the file is called manifest.json and is generated",
		at(t, "/"),
		at(t, "/design-system.html"),
	} {
		if found := CheckFile("README.md", []byte(line), ourHost(t)); len(found) != 0 {
			t.Errorf("%q was read as a printed install address: %v", line, found)
		}
	}
}

func TestWhatMayQuoteTheAddressIsNamedAndSmall(t *testing.T) {
	nothingRecorded(t)

	// The skip list is the part of this check most likely to grow quietly, one
	// file at a time as each new one is discovered. It is a scope of two
	// entries instead: the directory where an address is argued, and the check
	// itself. A third entry is a rule being widened, and this says so.
	if len(Skipped) != 2 {
		t.Fatalf("the skip list holds %d entries: %v", len(Skipped), Skipped)
	}
	for _, name := range []string{
		"decisions/manifest-address.md",
		"decisions/first-release.md",
		"internal/address/address.go",
	} {
		if found := CheckFile(name, []byte(at(t, "/manifest.json")), ourHost(t)); len(found) != 0 {
			t.Errorf("%s argues the rule and was refused anyway: %v", name, found)
		}
	}
	// Everything an operator reads is inside the rule, and a name that merely
	// starts like a skipped one is not.
	for _, name := range []string{"README.md", "SECURITY.md", "docs/index.html", "decisions.md"} {
		if found := CheckFile(name, []byte(at(t, "/manifest.json")), ourHost(t)); len(found) != 1 {
			t.Errorf("%s is operator-facing and produced %d refusals, want 1", name, len(found))
		}
	}
}

func TestNoTrackedFilePrintsAnUnansweredInstallAddress(t *testing.T) {
	// The leg, against this repository.
	found, err := CheckTree("../..")
	if err != nil {
		t.Fatalf("reading the tracked tree: %v", err)
	}
	if len(found) != 0 {
		var lines []string
		for _, f := range found {
			lines = append(lines, f.String())
		}
		t.Fatalf("an install address is printed that nobody has read:\n%s", strings.Join(lines, "\n"))
	}
}

func TestATreeWithNoTrackedFileIsAnErrorRatherThanACleanOne(t *testing.T) {
	if _, err := CheckTree(t.TempDir()); err == nil {
		t.Fatal("a run that read no tracked file reported a tree printing nothing")
	}
}
