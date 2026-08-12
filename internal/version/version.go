// Package version reads one installable build out of the release that ships it.
//
// internal/identity reads the plugin-level fields out of a release's descriptor
// and this package reads the version-level ones out of the same file. The split
// is the manifest's own: decisions/manifest-schema.md emits one plugin object
// per plugin and one version object per published build, so the rule for which
// release answers for the plugin, which is decisions/plugin-identity.md, is not
// the rule for which releases are published.
//
// Nothing here reaches the network. The descriptor arrives as bytes the caller
// read and the archive arrives as a pairing already made, for the reason
// internal/pairing gives: every rule below is judged against a fixture in the
// gate, and decisions/headless-and-unelevated.md keeps the one thing that leaves
// the runner outside all of it.
//
// What a run does with a refusal from here is not decided here. A defect in the
// newest release for a plugin and channel stops the run and the same defect in
// an older one is a skip that names itself, which is
// decisions/failure-posture.md, and #28 is where it is reported.
package version

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"flowfin.dev/hub/internal/pairing"
	"flowfin.dev/hub/manifest"
)

// Layout is the shape decisions/manifest-schema.md fixes for the timestamp:
// RFC 3339, second precision, a Z suffix, twenty characters. Every one of the
// two hundred and seventy-seven version entries in the ecosystem's own published
// catalogue is that length, and the measurement behind that sentence is in the
// decision file rather than repeated here.
const Layout = time.RFC3339

// Required is the set of descriptor fields a version entry cannot be published
// without, in the order the manifest emits them.
//
// changelog is deliberately absent from it. It is free text, a release that
// carries none is making a true statement about itself, and refusing such a
// release would drop an installable build over a field nothing installs.
// decisions/manifest-schema.md still emits the key, because every entry in the
// published catalogue carries it and an absent key is a second shape for a
// reader to handle.
var Required = []string{"version", "targetAbi", "timestamp"}

// Reason is why a release could not be turned into a version entry. An
// enumeration rather than a string because the run does different things with
// each case depending on which release it was, and telling them apart is
// decisions/failure-posture.md's question.
type Reason int

const (
	// UnreadableDescriptor means the descriptor body is not JSON.
	UnreadableDescriptor Reason = iota

	// MissingField means a field the entry cannot be published without is
	// absent or blank. decisions/failure-posture.md: an entry is never
	// published with a field the generator could not resolve, because a server
	// reports a failed install for that entry and reports nothing at all for an
	// entry that was never offered.
	MissingField

	// MalformedNumber means version or targetAbi is not a number the server can
	// parse. Both are compared numerically by the server: an unreadable version
	// is an entry it drops, and an unreadable targetAbi is an entry it hides
	// from every server it should have been offered to.
	MalformedNumber

	// MalformedTimestamp means the timestamp is not a time. It is not guessed
	// from anything else, because a guessed timestamp is a value that looks
	// resolved and orders entries wrongly for the life of the file.
	MalformedTimestamp
)

func (r Reason) String() string {
	switch r {
	case UnreadableDescriptor:
		return "unreadable-descriptor"
	case MissingField:
		return "missing-field"
	case MalformedNumber:
		return "malformed-number"
	case MalformedTimestamp:
		return "malformed-timestamp"
	}
	return "unknown"
}

// Refusal is a release that could not become a version entry. It names the
// release, because a run saying a version was skipped without saying which one
// sends a reader to every release page in turn.
type Refusal struct {
	Plugin  string
	Release string
	Reason  Reason
	Detail  string
}

func (r *Refusal) Error() string {
	return fmt.Sprintf("%s %s: %s: %s", r.Plugin, r.Release, r.Reason, r.Detail)
}

// descriptor is the part of the sidecar this package reads. The plugin-level
// fields are deliberately absent: internal/identity reads those, and a second
// struct carrying them would be a second answer about the same bytes.
type descriptor struct {
	Version   string `json:"version"`
	Changelog string `json:"changelog"`
	TargetABI string `json:"targetAbi"`
	Timestamp string `json:"timestamp"`
}

// Read builds the version entry for one release out of its descriptor and the
// archive that release publishes.
//
// The archive's address and its checksum come from the pairing rather than from
// the descriptor, because the two have to describe the same bytes and
// decisions/artifact-checksum-pairing.md is where that is decided. Every value
// is trimmed before it is judged and before it is kept, so a field holding a
// newline is the absence it looks like rather than a value that renders as a
// blank line on every server.
func Read(plugin, release string, body []byte, pair pairing.Pair) (manifest.Version, error) {
	var d descriptor
	if err := json.Unmarshal(body, &d); err != nil {
		return manifest.Version{}, &Refusal{
			Plugin:  plugin,
			Release: release,
			Reason:  UnreadableDescriptor,
			Detail:  err.Error(),
		}
	}

	entry := manifest.Version{
		Version:   strings.TrimSpace(d.Version),
		Changelog: strings.TrimSpace(d.Changelog),
		TargetABI: strings.TrimSpace(d.TargetABI),
		SourceURL: strings.TrimSpace(pair.Archive.URL),
		Checksum:  strings.TrimSpace(pair.Checksum),
	}
	timestamp := strings.TrimSpace(d.Timestamp)

	present := map[string]string{
		"version":   entry.Version,
		"targetAbi": entry.TargetABI,
		"timestamp": timestamp,
	}
	var missing []string
	for _, field := range Required {
		if present[field] == "" {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return manifest.Version{}, &Refusal{
			Plugin:  plugin,
			Release: release,
			Reason:  MissingField,
			Detail: fmt.Sprintf("%d of %d required descriptor field(s) absent or blank: %s",
				len(missing), len(Required), strings.Join(missing, ", ")),
		}
	}

	// The pairing is asked for the two fields it owns rather than trusted for
	// them. A pairing that arrived without an archive address or without a
	// digest is a caller's mistake, and the cost of not noticing it here is an
	// entry a server offers and cannot install.
	if entry.SourceURL == "" || entry.Checksum == "" {
		return manifest.Version{}, &Refusal{
			Plugin:  plugin,
			Release: release,
			Reason:  MissingField,
			Detail: fmt.Sprintf("the pairing carries archive address %q and checksum %q, and an entry is published with both or not at all",
				entry.SourceURL, entry.Checksum),
		}
	}

	if _, err := manifest.ParseNumber(entry.Version); err != nil {
		return manifest.Version{}, &Refusal{
			Plugin:  plugin,
			Release: release,
			Reason:  MalformedNumber,
			Detail:  fmt.Sprintf("version: %v, and the tag is not a fallback for it: the server parses this field and drops the entry when it cannot", err),
		}
	}
	if _, err := manifest.ParseNumber(entry.TargetABI); err != nil {
		return manifest.Version{}, &Refusal{
			Plugin:  plugin,
			Release: release,
			Reason:  MalformedNumber,
			Detail:  fmt.Sprintf("targetAbi: %v, and the server filters on it, so an entry it cannot read is invisible rather than refused", err),
		}
	}

	at, err := time.Parse(Layout, timestamp)
	if err != nil {
		return manifest.Version{}, &Refusal{
			Plugin:  plugin,
			Release: release,
			Reason:  MalformedTimestamp,
			Detail:  fmt.Sprintf("timestamp %q is not RFC 3339, and nothing else is read in its place", timestamp),
		}
	}
	// Emitted as the schema fixes it rather than as the descriptor spelled it.
	// An instant stated with an offset or with a fraction of a second is the
	// same instant, and the shape of the field is the manifest's rather than the
	// descriptor's to choose.
	entry.Timestamp = at.UTC().Format(Layout)

	return entry, nil
}
