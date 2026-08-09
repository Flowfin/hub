package manifest

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// Runs is how many times the determinism tests below build the file.
//
// The issue asks for two and this is more, for a measured reason. The failure
// these tests are aimed at is a pass that iterates a map, and Go randomises map
// iteration per range rather than per process, so two runs over a fixture of six
// plugins agree by luck about once in every seven hundred and twenty attempts. A
// test that passes 99.9 percent of the time on broken code is worse than no
// test, because the one red looks like the flake.
const Runs = 64

// fixture builds the input the determinism tests run over.
//
// It is built here rather than read from a file because what it has to be is
// large and shapeless rather than exemplary: enough plugins that an unordered
// iteration reorders something, enough versions per plugin that an unordered
// versions array does too, and more versions per target than the cap so the
// selection has work to do. The specimen that fixes the byte format is
// testdata/golden-manifest.json and this is not a second one.
//
// Nothing in it is a claim about any real release. The names, guids, digests and
// hosts are invented and the host is the domain reserved for exactly that.
func fixture() []Plugin {
	const (
		plugins  = 6
		targets  = 3
		versions = 8
	)

	// Not in guid order, so the pass has the plugin ordering to do as well.
	order := []int{3, 0, 5, 2, 4, 1}

	out := make([]Plugin, 0, plugins)
	for _, p := range order {
		plugin := Plugin{
			GUID:        fmt.Sprintf("%08x-%04d-4a00-9a00-%012x", 0x11111111*(p+1), p, p*111111),
			Name:        fmt.Sprintf("Fixture Plugin %d", p+1),
			Description: "Ampersand & and <angle brackets> stay literal.\n",
			Overview:    "A fixture plugin. Its versions are in no order and there are more of them than the cap keeps.",
			Owner:       "Example Owner",
			Category:    "General",
		}
		if p%2 == 0 {
			plugin.ImageURL = fmt.Sprintf("https://example.com/plugins/fixture-%d/logo.png", p+1)
		}

		for target := range targets {
			abi := fmt.Sprintf("10.%d.0.0", 9+target)
			// Ascending, so the versions array arrives oldest first and the
			// ordering rule has something to reverse. The last one is the .10
			// that a string comparison sorts below the .9 before it.
			for v := 2; v < 2+versions; v++ {
				n := v
				if v == 2+versions-1 {
					n = 10
				}
				plugin.Versions = append(plugin.Versions, Version{
					Version:   fmt.Sprintf("%d.0.%d", target+1, n),
					Changelog: fmt.Sprintf("%d.0.%d on the %s line.\n", target+1, n, abi),
					TargetABI: abi,
					SourceURL: fmt.Sprintf("https://example.com/plugins/fixture-%d_%d.0.%d.zip", p+1, target+1, n),
					Checksum:  fmt.Sprintf("%032x", p*1000+target*100+n),
					Timestamp: fmt.Sprintf("202%d-%02d-%02dT%02d:%02d:%02dZ", n%5+1, target+1, n, n, n%60, n%60),
				})
			}
		}
		out = append(out, plugin)
	}
	return out
}

// generate is the run: the fixture through the pass and the encoder, exactly as
// a published file would be built.
//
// It goes through a map on the way, keyed by guid, because that is how a
// generator accumulates plugins it read one repository at a time, and because a
// map is where the nondeterminism this test is about comes from. Ranging one is
// the difference between a test that could catch an unordered pass and a test
// that hands it a slice already in the right order and proves nothing.
func generate(t *testing.T) []byte {
	t.Helper()

	byGUID := map[string]Plugin{}
	for _, p := range fixture() {
		copied := p
		copied.Versions = append([]Version{}, p.Versions...)
		byGUID[p.GUID] = copied
	}

	collected := make([]Plugin, 0, len(byGUID))
	for _, p := range byGUID {
		collected = append(collected, p)
	}

	var out bytes.Buffer
	if err := Encode(&out, Select(collected, Cap)); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return out.Bytes()
}

// TestRegeneratingFromUnchangedInputsChangesNoBytes is the whole issue.
//
// Several things elsewhere rest on it. The freshness check reads a diff of the
// published file and concludes something from it, and the tamper argument says
// an edit made outside the generator becomes visible. Both are worthless if a
// second run over the same inputs moves bytes for reasons that have nothing to
// do with the releases.
func TestRegeneratingFromUnchangedInputsChangesNoBytes(t *testing.T) {
	first := generate(t)
	if len(first) < 1000 {
		t.Fatalf("the fixture produced %d bytes; two entries can agree by luck and this test needs a file with something in it", len(first))
	}

	for run := 2; run <= Runs; run++ {
		again := generate(t)
		if bytes.Equal(first, again) {
			continue
		}
		at := firstDifference(first, again)
		t.Fatalf("run %d of %d differs from run 1 at byte %d\n run 1 %q\n run %d %q\n %d bytes against %d",
			run, Runs, at, window(first, at), run, window(again, at), len(first), len(again))
	}
}

// TestTheOutputCarriesNoTimestampTheInputDidNotHave aims at the second of the
// uninteresting reasons: a timestamp taken from the clock rather than from the
// release. Such a field moves on its own between two runs, and a run fast enough
// to produce the same second twice hides it, so this reads the values instead of
// racing them.
func TestTheOutputCarriesNoTimestampTheInputDidNotHave(t *testing.T) {
	known := map[string]bool{}
	for _, p := range fixture() {
		for _, v := range p.Versions {
			known[v.Timestamp] = true
		}
	}

	out := string(generate(t))
	const key = `"timestamp": "`
	found := 0
	for rest := out; ; {
		i := strings.Index(rest, key)
		if i < 0 {
			break
		}
		rest = rest[i+len(key):]
		end := strings.IndexByte(rest, '"')
		if end < 0 {
			t.Fatal("a timestamp field is not terminated, so the output is not the format")
		}
		value := rest[:end]
		found++
		if !known[value] {
			t.Errorf("the output carries the timestamp %q, which no version in the fixture has", value)
		}
		rest = rest[end:]
	}
	if found == 0 {
		t.Fatal("the output carries no timestamp at all, so this test read nothing")
	}
}

// TestPluginOrderIsBytewiseAndNotCollated aims at the third: a locale-dependent
// sort, which gives one answer on the machine that generated the file and
// another on the machine that reviewed it.
//
// Byte order puts every capital before every lower-case letter. Every collation
// a person would call alphabetical does not, so the two disagree on this input
// and agree on almost any other, which is why the fixture is this and not a list
// of plausible names.
func TestPluginOrderIsBytewiseAndNotCollated(t *testing.T) {
	plugins := []Plugin{
		{GUID: "aaaa1111-0000-4000-8000-000000000000"},
		{GUID: "Bbbb0000-0000-4000-8000-000000000000"},
		{GUID: "aaaa0000-0000-4000-8000-000000000000"},
	}
	OrderPlugins(plugins)

	got := []string{plugins[0].GUID[:8], plugins[1].GUID[:8], plugins[2].GUID[:8]}
	want := []string{"Bbbb0000", "aaaa0000", "aaaa1111"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ordered %v, want %v; a collation that ignores case orders them %v",
			got, want, []string{"aaaa0000", "aaaa1111", "Bbbb0000"})
	}
}
