package publish_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"flowfin.dev/hub/internal/publish"
	"flowfin.dev/hub/internal/site"
	"flowfin.dev/hub/manifest"
)

// served is a root with the published directory in it, and the target inside
// that directory. Every test starts from one because a target pointed at a
// directory that does not exist is its own case, tested separately.
func served(t *testing.T) (string, publish.Target) {
	t.Helper()
	root := t.TempDir()
	target := publish.Target{Dir: publish.Stable.Dir, Name: publish.Stable.Name}
	if err := os.MkdirAll(filepath.Join(root, target.Dir), 0o755); err != nil {
		t.Fatalf("preparing the served directory: %v", err)
	}
	return root, target
}

func write(b []byte) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := w.Write(b)
		return err
	}
}

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return b
}

// entries is what the served directory holds, sorted, so a test can say that a
// run left a temporary file behind rather than only that the target is right.
func entries(t *testing.T, root string, target publish.Target) []string {
	t.Helper()
	found, err := os.ReadDir(filepath.Join(root, target.Dir))
	if err != nil {
		t.Fatalf("reading the served directory: %v", err)
	}
	var names []string
	for _, e := range found {
		names = append(names, e.Name())
	}
	return names
}

func TestPlacePutsTheProducedBytesAtTheTarget(t *testing.T) {
	root, target := served(t)

	want := []byte("[]\n")
	if _, err := publish.Place(root, target, write(want)); err != nil {
		t.Fatalf("placing: %v", err)
	}

	if got := read(t, target.Path(root)); !bytes.Equal(got, want) {
		t.Errorf("the target holds %q and the run produced %q", got, want)
	}
	if names := entries(t, root, target); len(names) != 1 || names[0] != target.Name {
		t.Errorf("the served directory holds %v, and a finished run leaves only %s", names, target.Name)
	}
}

// TestTheEncoderIsThePipelineThatFeedsThePlacement runs the two halves the way
// the route is meant to be run, generate and then place, so that the encoder's
// bytes are what arrives rather than a string a test made up.
func TestTheEncoderIsThePipelineThatFeedsThePlacement(t *testing.T) {
	root, target := served(t)

	plugins := []manifest.Plugin{{
		GUID:        "8c95c4d2-e50c-4fb0-a4f3-6c06ff0f9a1a",
		Name:        "Example",
		Description: "A fixture plugin, and the ampersand & is here because the encoder is judged on it",
		Overview:    "A fixture plugin",
		Owner:       "an-account",
		Category:    "General",
		Versions: []manifest.Version{{
			Version:   "1.0.0.0",
			Changelog: "First",
			TargetABI: "10.9.0.0",
			SourceURL: "https://example.com/plugins/example_1.0.0.0.zip",
			Checksum:  "0123456789abcdef0123456789abcdef",
			Timestamp: "2026-08-10T00:00:00Z",
		}},
	}}

	var direct bytes.Buffer
	if err := manifest.Encode(&direct, plugins); err != nil {
		t.Fatalf("encoding: %v", err)
	}
	if _, err := publish.Place(root, target, func(w io.Writer) error {
		return manifest.Encode(w, plugins)
	}); err != nil {
		t.Fatalf("placing: %v", err)
	}

	if got := read(t, target.Path(root)); !bytes.Equal(got, direct.Bytes()) {
		t.Errorf("the placed file is\n%s\nand the encoder produced\n%s", got, direct.Bytes())
	}
}

// TestARunThatFailsLeavesThePublishedFileByteIdentical is the second clause of
// #31 and the reason the write goes through a temporary file at all.
func TestARunThatFailsLeavesThePublishedFileByteIdentical(t *testing.T) {
	root, target := served(t)

	published := []byte("[\n    {\n        \"guid\": \"kept\"\n    }\n]\n")
	if _, err := publish.Place(root, target, write(published)); err != nil {
		t.Fatalf("publishing the first file: %v", err)
	}

	broke := fmt.Errorf("the run gave up after writing half of it")
	placed, err := publish.Place(root, target, func(w io.Writer) error {
		if _, err := w.Write([]byte("[\n    {\n        \"guid\": \"half")); err != nil {
			return err
		}
		return broke
	})
	if err == nil {
		t.Fatal("a run whose producer failed reported success")
	}
	if placed != publish.NotPlaced {
		t.Errorf("a run that gave up reports %q, and a caller reading it is told a failed run left something at the address", placed)
	}

	if got := read(t, target.Path(root)); !bytes.Equal(got, published) {
		t.Errorf("the failed run left %q where the published file was %q", got, published)
	}
	if names := entries(t, root, target); len(names) != 1 || names[0] != target.Name {
		t.Errorf("the served directory holds %v, and a failed run leaves only %s", names, target.Name)
	}
}

// TestTwoRunsDoNotInterleave holds the first run halfway through its bytes,
// finishes a second run over the top of it, and then lets the first one
// through. Every read between the two is a whole file.
func TestTwoRunsDoNotInterleave(t *testing.T) {
	root, target := served(t)

	published := []byte("[]\n")
	if _, err := publish.Place(root, target, write(published)); err != nil {
		t.Fatalf("publishing the first file: %v", err)
	}

	slow := bytes.Repeat([]byte("a"), 8192)
	quick := bytes.Repeat([]byte("b"), 8192)

	halfway := make(chan struct{})
	carryOn := make(chan struct{})
	finished := make(chan error, 1)

	go func() {
		_, err := publish.Place(root, target, func(w io.Writer) error {
			if _, err := w.Write(slow[:len(slow)/2]); err != nil {
				return err
			}
			close(halfway)
			<-carryOn
			_, err := w.Write(slow[len(slow)/2:])
			return err
		})
		finished <- err
	}()

	<-halfway
	if got := read(t, target.Path(root)); !bytes.Equal(got, published) {
		t.Errorf("a run halfway through its bytes has already changed the published file to %d byte(s)", len(got))
	}

	if _, err := publish.Place(root, target, write(quick)); err != nil {
		t.Fatalf("the second run: %v", err)
	}
	if got := read(t, target.Path(root)); !bytes.Equal(got, quick) {
		t.Errorf("the second run left %d byte(s) and produced %d", len(got), len(quick))
	}

	close(carryOn)
	if err := <-finished; err != nil {
		t.Fatalf("the first run: %v", err)
	}
	if got := read(t, target.Path(root)); !bytes.Equal(got, slow) {
		t.Errorf("the first run finished and left %d byte(s) rather than its own %d", len(got), len(slow))
	}
}

// TestTheProducerIsUnchangedWhenTheLocationMoves is the third clause of #31.
// One producer, two locations, and nothing about the producer differs between
// them.
func TestTheProducerIsUnchangedWhenTheLocationMoves(t *testing.T) {
	root := t.TempDir()
	produce := write([]byte("[]\n"))

	moved := publish.Target{Dir: "somewhere-else", Name: "catalogue.json"}
	for _, target := range []publish.Target{publish.Stable, moved} {
		if err := os.MkdirAll(filepath.Join(root, target.Dir), 0o755); err != nil {
			t.Fatalf("preparing %s: %v", target.Dir, err)
		}
		if _, err := publish.Place(root, target, produce); err != nil {
			t.Fatalf("placing at %s: %v", target.Path(root), err)
		}
		if got := read(t, target.Path(root)); !bytes.Equal(got, []byte("[]\n")) {
			t.Errorf("%s holds %q", target.Path(root), got)
		}
	}
}

// TestTheStableTargetSitsInWhatTheSitePublishes is what keeps the location from
// being a second copy of the served directory that a move would leave behind.
func TestTheStableTargetSitsInWhatTheSitePublishes(t *testing.T) {
	if publish.Stable.Dir != site.Dir {
		t.Errorf("the stable target is published from %q and the site is served from %q", publish.Stable.Dir, site.Dir)
	}
	if publish.Stable.Name == "" {
		t.Error("the stable target has no file name, so no address ends in one")
	}
}

func TestADirectoryThatIsNotThereIsRefused(t *testing.T) {
	root := t.TempDir()
	_, err := publish.Place(root, publish.Stable, write([]byte("[]\n")))
	if err == nil {
		t.Fatal("placing into a directory that does not exist reported success")
	}
	if _, statErr := os.Stat(filepath.Join(root, publish.Stable.Dir)); statErr == nil {
		t.Error("the run created the directory it was pointed at rather than refusing")
	}
}

func TestAFileWhereTheDirectoryShouldBeIsRefused(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, publish.Stable.Dir), []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if _, err := publish.Place(root, publish.Stable, write([]byte("[]\n"))); err == nil {
		t.Fatal("placing into a file reported success")
	}
}

func TestATargetMissingEitherHalfIsRefused(t *testing.T) {
	root := t.TempDir()
	for _, target := range []publish.Target{
		{Dir: "", Name: "manifest.json"},
		{Dir: publish.Stable.Dir, Name: ""},
	} {
		if _, err := publish.Place(root, target, write([]byte("[]\n"))); err == nil {
			t.Errorf("the target %+v was accepted", target)
		}
	}
}

// TestThePlacedFileIsReadable is the case os.CreateTemp's own mode would
// produce: a file the owner can read and a server cannot.
func TestThePlacedFileIsReadable(t *testing.T) {
	root, target := served(t)
	if _, err := publish.Place(root, target, write([]byte("[]\n"))); err != nil {
		t.Fatalf("placing: %v", err)
	}
	info, err := os.Stat(target.Path(root))
	if err != nil {
		t.Fatalf("reading the placed file: %v", err)
	}
	if info.Mode().Perm()&0o044 == 0 {
		t.Errorf("the placed file is %v, which nothing but its owner can read", info.Mode().Perm())
	}
}

// TestTheFirstRunAtAnAddressWroteNewBytes is the case where there is nothing to
// compare against. It is a change rather than an absence matching an absence,
// because what an operator gets goes from no catalogue to a catalogue.
func TestTheFirstRunAtAnAddressWroteNewBytes(t *testing.T) {
	root, target := served(t)

	placed, err := publish.Place(root, target, write([]byte("[]\n")))
	if err != nil {
		t.Fatalf("placing: %v", err)
	}
	if placed != publish.Changed {
		t.Errorf("the first run at an address reports %q", placed)
	}
}

// TestARunThatReplacesThePublishedBytesSaysSo and the test below it are the two
// halves of #32's fourth clause, and they are only worth anything together: a
// verdict that answers the same way for both distinguishes nothing.
func TestARunThatReplacesThePublishedBytesSaysSo(t *testing.T) {
	root, target := served(t)

	if _, err := publish.Place(root, target, write([]byte("[]\n"))); err != nil {
		t.Fatalf("publishing the first file: %v", err)
	}

	second := []byte("[\n    {\n        \"guid\": \"released-since\"\n    }\n]\n")
	placed, err := publish.Place(root, target, write(second))
	if err != nil {
		t.Fatalf("the second run: %v", err)
	}
	if placed != publish.Changed {
		t.Errorf("a run that replaced the published bytes reports %q", placed)
	}
	if got := read(t, target.Path(root)); !bytes.Equal(got, second) {
		t.Errorf("the address answers with %q and the run produced %q", got, second)
	}
}

func TestARunThatProducesWhatIsAlreadyPublishedHasNothingToWrite(t *testing.T) {
	root, target := served(t)

	same := []byte("[\n    {\n        \"guid\": \"nothing-released-since\"\n    }\n]\n")
	if _, err := publish.Place(root, target, write(same)); err != nil {
		t.Fatalf("publishing the first file: %v", err)
	}

	placed, err := publish.Place(root, target, write(same))
	if err != nil {
		t.Fatalf("the second run: %v", err)
	}
	if placed != publish.Unchanged {
		t.Errorf("a run that produced the published bytes reports %q", placed)
	}
	if got := read(t, target.Path(root)); !bytes.Equal(got, same) {
		t.Errorf("the address answers with %q after a run that changed nothing", got)
	}
	if names := entries(t, root, target); len(names) != 1 || names[0] != target.Name {
		t.Errorf("the served directory holds %v after a run with nothing to write", names)
	}
}

// TestARunWithNothingToWriteStillReplacesTheFile holds the decision recorded at
// Place. Returning early on identical bytes is cheaper and leaves the file the
// last run left, so a mode, an owner or any damage done to it since survives
// for as long as nothing is released, and the catalogue a server is refused
// stays refused.
//
// The planted time is what says which of the two happened. A file the run
// renamed its own over carries a time from the run, and a file the run left
// alone still carries the one planted here, whatever the clock did in between.
func TestARunWithNothingToWriteStillReplacesTheFile(t *testing.T) {
	root, target := served(t)

	same := []byte("[]\n")
	if _, err := publish.Place(root, target, write(same)); err != nil {
		t.Fatalf("publishing the first file: %v", err)
	}

	planted := time.Date(2001, time.September, 9, 1, 46, 40, 0, time.UTC)
	if err := os.Chtimes(target.Path(root), planted, planted); err != nil {
		t.Fatalf("planting a modification time on the published file: %v", err)
	}

	placed, err := publish.Place(root, target, write(same))
	if err != nil {
		t.Fatalf("the second run: %v", err)
	}
	if placed != publish.Unchanged {
		t.Fatalf("the second run reports %q, so this test is not measuring what it says", placed)
	}

	info, err := os.Stat(target.Path(root))
	if err != nil {
		t.Fatalf("reading the published file: %v", err)
	}
	if info.ModTime().Equal(planted) {
		t.Error("a run with nothing to write left the file it found rather than placing its own over it")
	}
}

// TestTheZeroVerdictSaysNothingWasPlaced pins the order the constants are
// declared in. With Changed first, a caller that ignores the error and prints
// the verdict is told by a run that never reached the address that it wrote a
// new catalogue.
func TestTheZeroVerdictSaysNothingWasPlaced(t *testing.T) {
	var unset publish.Placement
	if unset != publish.NotPlaced {
		t.Errorf("the zero verdict is %q", unset)
	}
}

// TestEachVerdictReadsAsItself is the whole clause reduced to the words. Two
// verdicts printing one string distinguish nothing in the output, however
// carefully the comparison behind them is taken.
func TestEachVerdictReadsAsItself(t *testing.T) {
	seen := map[string]publish.Placement{}
	for _, p := range []publish.Placement{publish.NotPlaced, publish.Changed, publish.Unchanged} {
		word := p.String()
		if word == "" {
			t.Errorf("a verdict prints nothing")
		}
		if other, ok := seen[word]; ok {
			t.Errorf("two verdicts both print %q, and one of them is %d and the other %d", word, other, p)
		}
		seen[word] = p
	}
}
