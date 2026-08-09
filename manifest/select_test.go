package manifest

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// TestVersionsOrderNumericallyRatherThanAsStrings is the near miss the issue
// names, and it is one character of code away in either direction: sorting the
// version field as a string puts 1.0.10 below 1.0.9, and the server compares the
// same two numerically and puts it above. The manifest would then present the
// older build as the newest one, which is not a parse error anywhere.
func TestVersionsOrderNumericallyRatherThanAsStrings(t *testing.T) {
	versions := []Version{
		{Version: "1.0.9", TargetABI: "10.11.0.0", Timestamp: "2026-01-01T00:00:00Z"},
		{Version: "1.0.10", TargetABI: "10.11.0.0", Timestamp: "2026-01-02T00:00:00Z"},
		{Version: "1.0.2", TargetABI: "10.11.0.0", Timestamp: "2026-01-03T00:00:00Z"},
	}
	OrderVersions(versions)

	got := versionStrings(versions)
	want := []string{"1.0.10", "1.0.9", "1.0.2"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ordered %v, want %v; a string comparison produces %v", got, want, []string{"1.0.9", "1.0.2", "1.0.10"})
	}
	if strings.Join(got, ",") == "1.0.9,1.0.2,1.0.10" {
		t.Fatal("the versions came out in string order, which is the failure this test exists for")
	}
}

func TestTheTimestampBreaksATieAndTheTargetBreaksThat(t *testing.T) {
	// Two builds of one version happen: a rebuild for a second target line, and
	// a republished archive. Without a total order the two runs of the generator
	// can differ, which is the determinism claim in #29 gone before it is made.
	versions := []Version{
		{Version: "2.0.0.0", TargetABI: "10.10.0.0", Timestamp: "2026-01-01T00:00:00Z"},
		{Version: "2.0.0.0", TargetABI: "10.11.0.0", Timestamp: "2026-01-01T00:00:00Z"},
		{Version: "2.0.0.0", TargetABI: "10.11.0.0", Timestamp: "2026-02-01T00:00:00Z"},
	}
	OrderVersions(versions)

	var got []string
	for _, v := range versions {
		got = append(got, v.Timestamp+"/"+v.TargetABI)
	}
	want := "2026-02-01T00:00:00Z/10.11.0.0,2026-01-01T00:00:00Z/10.11.0.0,2026-01-01T00:00:00Z/10.10.0.0"
	if strings.Join(got, ",") != want {
		t.Fatalf("ordered %v\n want %s", got, want)
	}
}

func TestOrderIsTotalWhateverTheInputOrderWas(t *testing.T) {
	// The property that makes two runs agree. Every rotation of one list has to
	// come out the same way, so no entry's position can depend on where it
	// started.
	base := []Version{
		{Version: "1.0.10", TargetABI: "10.11.0.0", Timestamp: "2026-01-02T00:00:00Z"},
		{Version: "1.0.9", TargetABI: "10.11.0.0", Timestamp: "2026-01-01T00:00:00Z"},
		{Version: "1.0.9", TargetABI: "10.10.0.0", Timestamp: "2026-01-01T00:00:00Z"},
		{Version: "not-a-version", TargetABI: "10.11.0.0", Timestamp: "2026-03-01T00:00:00Z"},
		{Version: "1.0.9", TargetABI: "10.11.0.0", Timestamp: "2026-01-01T00:00:00Z", SourceURL: "b"},
	}
	first := ""
	for rotation := range base {
		rotated := append(append([]Version{}, base[rotation:]...), base[:rotation]...)
		OrderVersions(rotated)
		got := strings.Join(versionStrings(rotated), ",") + "|" +
			strings.Join(fieldStrings(rotated), ",")
		if rotation == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("rotation %d ordered as %s, rotation 0 ordered as %s", rotation, got, first)
		}
	}
	// The unreadable version sorts last rather than anywhere: it is not a
	// judgement about the version, only a place that keeps the order total.
	OrderVersions(base)
	if last := base[len(base)-1]; last.Version != "not-a-version" {
		t.Fatalf("the version that does not parse sorted to %q, not last", last.Version)
	}
}

func TestTheCapIsPerTargetAndNotAcrossTheList(t *testing.T) {
	// The failure decisions/version-cap.md is about. The fast line publishes
	// eight times while the slow line publishes twice; an overall cap of five
	// keeps five entries and none of them is installable on the slow line's
	// servers, and nothing anywhere is red.
	var versions []Version
	for i := 8; i >= 1; i-- {
		versions = append(versions, Version{
			Version:   "3.0.0." + strconv.Itoa(i),
			TargetABI: "10.11.0.0",
			Timestamp: "2026-0" + strconv.Itoa(i%9+1) + "-01T00:00:00Z",
		})
	}
	versions = append(versions,
		Version{Version: "1.1.0.0", TargetABI: "10.10.0.0", Timestamp: "2025-01-01T00:00:00Z"},
		Version{Version: "1.0.0.0", TargetABI: "10.10.0.0", Timestamp: "2024-01-01T00:00:00Z"},
	)

	kept := CapPerTarget(versions, Cap)

	perTarget := map[string]int{}
	for _, v := range kept {
		perTarget[v.TargetABI]++
	}
	if perTarget["10.11.0.0"] != Cap {
		t.Errorf("the fast target kept %d entries, want %d", perTarget["10.11.0.0"], Cap)
	}
	if perTarget["10.10.0.0"] != 2 {
		t.Errorf("the slow target kept %d entries, want both of its 2; an overall cap keeps 0 of them",
			perTarget["10.10.0.0"])
	}
	if len(kept) != Cap+2 {
		t.Errorf("kept %d entries in total, want %d; a cap across the list keeps %d", len(kept), Cap+2, Cap)
	}
	// What survives is the newest of each line, not whichever happened to be
	// first in the input.
	if kept[0].Version != "3.0.0.8" {
		t.Errorf("the newest kept entry is %s, want 3.0.0.8", kept[0].Version)
	}
	for _, v := range kept {
		if v.TargetABI == "10.11.0.0" && (v.Version == "3.0.0.1" || v.Version == "3.0.0.2" || v.Version == "3.0.0.3") {
			t.Errorf("%s survived the cap, and it is not among the newest %d of its line", v.Version, Cap)
		}
	}
}

func TestTheCapKeepsTheNewestOfEachLineWhicheverOrderTheyArrivedIn(t *testing.T) {
	versions := []Version{
		{Version: "1.0.0.0", TargetABI: "a", Timestamp: "2026-01-01T00:00:00Z"},
		{Version: "1.0.0.9", TargetABI: "a", Timestamp: "2026-01-09T00:00:00Z"},
		{Version: "1.0.0.10", TargetABI: "a", Timestamp: "2026-01-10T00:00:00Z"},
	}
	kept := CapPerTarget(versions, 2)
	if got := strings.Join(versionStrings(kept), ","); got != "1.0.0.10,1.0.0.9" {
		t.Fatalf("kept %s, want 1.0.0.10,1.0.0.9", got)
	}
}

// TestSelectBuildsTheGoldenFixtureFromAnUnorderedOvercappedInput is the byte
// half of the issue's done-condition. The input is the specimen's own content
// with the plugins reversed, every versions array reversed, and three entries
// added to the target line that is already at the cap. Nothing but the pass
// under test is between that and the published bytes.
func TestSelectBuildsTheGoldenFixtureFromAnUnorderedOvercappedInput(t *testing.T) {
	golden := readGolden(t)

	var want []Plugin
	if err := json.Unmarshal(golden, &want); err != nil {
		t.Fatalf("decoding %s: %v", goldenPath, err)
	}
	if len(want) < 2 {
		t.Fatalf("%s holds %d plugins; this test needs at least two to see the plugin order", goldenPath, len(want))
	}

	got := Select(disorder(want), Cap)

	var encoded bytes.Buffer
	if err := Encode(&encoded, got); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if bytes.Equal(golden, encoded.Bytes()) {
		return
	}
	at := firstDifference(golden, encoded.Bytes())
	t.Errorf("the pass does not rebuild %s: first difference at byte %d\n want %q\n  got %q\n golden is %d bytes, output is %d bytes",
		goldenPath, at, window(golden, at), window(encoded.Bytes(), at), len(golden), encoded.Len())
}

// disorder returns the fixture's plugins with everything the pass is supposed to
// fix put wrong: the plugin order reversed, each versions array reversed, and
// three extra entries on whichever target line already holds the most, so the
// cap has something to remove.
func disorder(plugins []Plugin) []Plugin {
	out := make([]Plugin, 0, len(plugins))
	for i := len(plugins) - 1; i >= 0; i-- {
		p := plugins[i]

		reversed := make([]Version, 0, len(p.Versions)+3)
		for j := len(p.Versions) - 1; j >= 0; j-- {
			reversed = append(reversed, p.Versions[j])
		}

		if fullest, count := fullestTarget(p.Versions); count >= Cap {
			for _, extra := range []string{"0.0.0.3", "0.0.0.2", "0.0.0.1"} {
				reversed = append(reversed, Version{
					Version:   extra,
					Changelog: "older than everything on this line, so the cap removes it\n",
					TargetABI: fullest,
					SourceURL: "https://example.com/plugins/dropped_" + extra + ".zip",
					Checksum:  "ffffffffffffffffffffffffffffffff",
					Timestamp: "2020-01-01T00:00:00Z",
				})
			}
		}

		p.Versions = reversed
		out = append(out, p)
	}
	return out
}

func fullestTarget(versions []Version) (string, int) {
	count := map[string]int{}
	for _, v := range versions {
		count[v.TargetABI]++
	}
	best, most := "", 0
	for target, n := range count {
		if n > most || (n == most && target > best) {
			best, most = target, n
		}
	}
	return best, most
}

func TestParseNumberReadsWhatTheFormatCarriesAndRefusesTheRest(t *testing.T) {
	for _, c := range []struct {
		in   string
		want Number
	}{
		{"1.2.3.4", Number{1, 2, 3, 4}},
		{"1.0.9", Number{1, 0, 9, 0}},
		{"2", Number{2, 0, 0, 0}},
		{"0.0.0.0", Number{}},
	} {
		got, err := ParseNumber(c.in)
		if err != nil {
			t.Errorf("ParseNumber(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseNumber(%q) = %v, want %v", c.in, got, c.want)
		}
	}
	for _, in := range []string{"", "1.2.3.4.5", "1.0.0-beta", "v1.0.0.0", "1. 0.0.0", "1.-2.0.0"} {
		if _, err := ParseNumber(in); err == nil {
			t.Errorf("ParseNumber(%q) was accepted; an order invented for a string nobody can read is an order two runs can disagree about", in)
		}
	}
}

func TestACapBelowOneKeepsNothingRatherThanEverything(t *testing.T) {
	// The off-by-one that would publish the whole history. An empty result is
	// refused before publication by decisions/failure-posture.md; a full one
	// would not be refused anywhere.
	versions := []Version{{Version: "1.0.0.0", TargetABI: "a"}, {Version: "2.0.0.0", TargetABI: "a"}}
	if kept := CapPerTarget(versions, 0); len(kept) != 0 {
		t.Fatalf("a cap of zero kept %d entries", len(kept))
	}
}

func versionStrings(versions []Version) []string {
	out := make([]string, 0, len(versions))
	for _, v := range versions {
		out = append(out, v.Version)
	}
	return out
}

func fieldStrings(versions []Version) []string {
	out := make([]string, 0, len(versions))
	for _, v := range versions {
		out = append(out, v.TargetABI+"/"+v.Timestamp+"/"+v.SourceURL)
	}
	return out
}
