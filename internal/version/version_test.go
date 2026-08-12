package version

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"flowfin.dev/hub/internal/pairing"
	"flowfin.dev/hub/manifest"
)

// The values here are invented and the host is the domain reserved for
// documentation. A fixture built out of a real release would prove the state of
// that release on the day it was written rather than the rule being tested.

const archive = "https://example.com/download/a-plugin_2.4.0.10.zip"

const digest = "0123456789abcdef0123456789abcdef"

func paired() pairing.Pair {
	return pairing.Pair{
		Archive:  pairing.Asset{Name: "a-plugin_2.4.0.10.zip", URL: archive, Size: 1024},
		Checksum: digest,
		Sidecars: []string{"a-plugin_2.4.0.10.zip.md5sum"},
	}
}

// body writes a descriptor carrying the four fields this package reads, with
// whatever the caller wants in them.
func body(t *testing.T, fields map[string]string) []byte {
	t.Helper()
	full := map[string]string{
		"version":   "2.4.0.10",
		"changelog": "A line about what changed.",
		"targetAbi": "10.11.0.0",
		"timestamp": "2026-08-12T05:47:26Z",
	}
	for k, v := range fields {
		if v == "" {
			delete(full, k)
			continue
		}
		full[k] = v
	}
	out, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("building the fixture: %v", err)
	}
	return out
}

func refusal(t *testing.T, err error) *Refusal {
	t.Helper()
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("the error is %v, which is not a refusal naming the release", err)
	}
	return r
}

func TestAVersionEntryIsBuiltFromTheDescriptorAndThePairing(t *testing.T) {
	got, err := Read("a-plugin", "2.4.0-stable", body(t, nil), paired())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	want := manifest.Version{
		Version:   "2.4.0.10",
		Changelog: "A line about what changed.",
		TargetABI: "10.11.0.0",
		SourceURL: archive,
		Checksum:  digest,
		Timestamp: "2026-08-12T05:47:26Z",
	}
	if got != want {
		t.Fatalf("the entry is %+v, want %+v", got, want)
	}
}

func TestTheArchiveAndItsChecksumComeFromThePairingRatherThanTheDescriptor(t *testing.T) {
	// The two have to describe the same bytes. A descriptor that also named an
	// archive would be a second opinion about which file the release ships, and
	// the manifest carries one address and one digest.
	said := body(t, map[string]string{
		"sourceUrl": "https://example.com/download/something-else.zip",
		"checksum":  "ffffffffffffffffffffffffffffffff",
	})
	got, err := Read("a-plugin", "2.4.0-stable", said, paired())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.SourceURL != archive || got.Checksum != digest {
		t.Fatalf("the entry took %q and %q out of the descriptor", got.SourceURL, got.Checksum)
	}
}

func TestABodyThatIsNotJsonIsRefusedRatherThanReadAsEmpty(t *testing.T) {
	// A descriptor nothing can parse read as a descriptor with no fields is an
	// entry published with holes in it, which is the one thing
	// decisions/failure-posture.md says is never done.
	_, err := Read("a-plugin", "2.4.0-stable", []byte("<!doctype html>"), paired())
	if err == nil {
		t.Fatal("a body that is not JSON was read")
	}
	if r := refusal(t, err); r.Reason != UnreadableDescriptor {
		t.Fatalf("the reason is %s", r.Reason)
	}
}

func TestAVersionTheServerCannotParseIsRefusedAndTheTagIsNoFallback(t *testing.T) {
	// The tag and the version are different strings in the one sourced release
	// that exists, which decisions/manifest-schema.md measured: the tag carries
	// a pre-release suffix and the version is four components. A version field
	// holding the tag is an entry the server drops.
	_, err := Read("a-plugin", "2.4.0-beta.10", body(t, map[string]string{"version": "2.4.0-beta.10"}), paired())
	if err == nil {
		t.Fatal("a version the server cannot parse was published")
	}
	r := refusal(t, err)
	if r.Reason != MalformedNumber {
		t.Fatalf("the reason is %s", r.Reason)
	}
	if !strings.Contains(r.Detail, "version") {
		t.Fatalf("the refusal does not say which field it was: %s", r.Detail)
	}
}

func TestATargetTheServerCannotParseIsRefused(t *testing.T) {
	// The server filters on this field, so an entry carrying one it cannot read
	// is invisible to every server rather than reported to anybody.
	_, err := Read("a-plugin", "2.4.0-stable", body(t, map[string]string{"targetAbi": "10.11"}), paired())
	if err != nil {
		t.Fatalf("a three-component target is the missing components read as zero: %v", err)
	}

	_, err = Read("a-plugin", "2.4.0-stable", body(t, map[string]string{"targetAbi": "latest"}), paired())
	if err == nil {
		t.Fatal("a target that is not a number was published")
	}
	r := refusal(t, err)
	if r.Reason != MalformedNumber {
		t.Fatalf("the reason is %s", r.Reason)
	}
	if !strings.Contains(r.Detail, "targetAbi") {
		t.Fatalf("the refusal does not say which field it was: %s", r.Detail)
	}
}

func TestATimestampThatIsNotATimeIsRefusedRatherThanGuessed(t *testing.T) {
	_, err := Read("a-plugin", "2.4.0-stable", body(t, map[string]string{"timestamp": "12 August 2026"}), paired())
	if err == nil {
		t.Fatal("a timestamp that is not a time was published")
	}
	if r := refusal(t, err); r.Reason != MalformedTimestamp {
		t.Fatalf("the reason is %s", r.Reason)
	}
}

func TestATimestampIsEmittedInTheShapeTheSchemaFixes(t *testing.T) {
	// The instant is the descriptor's and the shape is the manifest's. Twenty
	// characters ending in Z is what every entry in the published catalogue
	// carries, and an offset or a fraction of a second states the same instant
	// in a shape this file does not use.
	for _, said := range []string{
		"2026-08-12T07:47:26+02:00",
		"2026-08-12T05:47:26.500Z",
	} {
		got, err := Read("a-plugin", "2.4.0-stable", body(t, map[string]string{"timestamp": said}), paired())
		if err != nil {
			t.Fatalf("%s: %v", said, err)
		}
		if got.Timestamp != "2026-08-12T05:47:26Z" {
			t.Errorf("%s was emitted as %q", said, got.Timestamp)
		}
		if len(got.Timestamp) != 20 {
			t.Errorf("%s was emitted as %d characters", said, len(got.Timestamp))
		}
	}
}

func TestAnAbsentRequiredFieldNamesItselfRatherThanBeingEmitted(t *testing.T) {
	for _, field := range Required {
		// An empty value deletes the key from the fixture rather than blanking
		// it, so this is the field being absent and not present and empty.
		_, err := Read("a-plugin", "2.4.0-stable", body(t, map[string]string{field: ""}), paired())
		if err == nil {
			t.Fatalf("an entry with no %s was published", field)
		}
		r := refusal(t, err)
		if r.Reason != MissingField {
			t.Fatalf("%s: the reason is %s", field, r.Reason)
		}
		if !strings.Contains(r.Detail, field) {
			t.Fatalf("%s: the refusal does not name it: %s", field, r.Detail)
		}
	}
}

func TestAFieldHoldingOnlyWhitespaceIsTheAbsenceItLooksLike(t *testing.T) {
	_, err := Read("a-plugin", "2.4.0-stable", body(t, map[string]string{"version": "\n  "}), paired())
	if err == nil {
		t.Fatal("a version holding a newline was published")
	}
	if r := refusal(t, err); r.Reason != MissingField {
		t.Fatalf("the reason is %s", r.Reason)
	}
}

func TestAnEntryIsNeverBuiltWithoutTheArchiveOrItsChecksum(t *testing.T) {
	// Both come from the caller, so this is the caller's mistake rather than the
	// release's. An entry offering an install a server cannot perform is worse
	// than an entry that was never offered, which is why it is checked here
	// rather than assumed.
	for _, broken := range []pairing.Pair{
		{Archive: pairing.Asset{Name: "a-plugin_2.4.0.10.zip"}, Checksum: digest},
		{Archive: pairing.Asset{Name: "a-plugin_2.4.0.10.zip", URL: archive}},
	} {
		_, err := Read("a-plugin", "2.4.0-stable", body(t, nil), broken)
		if err == nil {
			t.Fatalf("an entry was built from %+v", broken)
		}
		if r := refusal(t, err); r.Reason != MissingField {
			t.Fatalf("the reason is %s", r.Reason)
		}
	}
}

func TestAReleaseWithNoChangelogIsPublishedRatherThanSkipped(t *testing.T) {
	// Free text, and a release carrying none is making a true statement about
	// itself. Dropping an installable build over a field nothing installs is a
	// worse answer than an empty string.
	got, err := Read("a-plugin", "2.4.0-stable", body(t, map[string]string{"changelog": ""}), paired())
	if err != nil {
		t.Fatalf("a release with no changelog was refused: %v", err)
	}
	if got.Changelog != "" {
		t.Fatalf("the changelog came back as %q", got.Changelog)
	}
	if got.Version != "2.4.0.10" {
		t.Fatalf("the rest of the entry did not survive: %+v", got)
	}
}

func TestARefusalNamesThePluginAndTheRelease(t *testing.T) {
	// A run saying a version was skipped without saying which one sends whoever
	// reads it to every release page in turn.
	_, err := Read("a-plugin", "2.4.0-beta.10", []byte("{"), paired())
	if err == nil {
		t.Fatal("a truncated body was read")
	}
	message := err.Error()
	for _, want := range []string{"a-plugin", "2.4.0-beta.10", "unreadable-descriptor"} {
		if !strings.Contains(message, want) {
			t.Errorf("the refusal does not carry %q: %s", want, message)
		}
	}
}

func TestEveryReasonPrintsItsOwnName(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range []Reason{UnreadableDescriptor, MissingField, MalformedNumber, MalformedTimestamp} {
		name := r.String()
		if name == "unknown" || seen[name] {
			t.Errorf("reason %d prints %q", r, name)
		}
		seen[name] = true
	}
	if got := Reason(99).String(); got != "unknown" {
		t.Errorf("an unnamed reason prints %q", got)
	}
}
