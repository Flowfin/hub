// Package pairing selects a release's plugin archive and the checksum that
// belongs to it.
//
// The manifest publishes one archive URL and one checksum per version entry, and
// the two have to describe the same bytes. They come apart when the checksum is
// chosen by position instead of by name: picking the last asset whose name ends
// in a checksum suffix, or the first one a sort returns, works until a rename
// moves a different file into that position. When that happened in a
// neighbouring project the manifest carried the SBOM's checksum against the
// plugin archive, and every install on every server failed verification with
// nothing in the interface saying which of the two was wrong.
//
// So the archive is selected first, on its own, by a stated predicate over the
// asset list. The checksum is then derived from the selected archive's own name,
// read out of the sidecar's CONTENTS rather than out of its filename, because
// the filename convention people assume is there is not there.
// decisions/artifact-checksum-pairing.md is where that is settled and carries
// the release it was measured against.
//
// Nothing here reaches the network. Sidecar bodies arrive through a Fetch the
// caller supplies, so every rule below is judged against a fixture in the gate
// and the one thing that leaves the runner stays outside this package.
package pairing

import (
	"fmt"
	"sort"
	"strings"
)

// ArchiveSuffix is the predicate that selects the archive, and it is a suffix
// rather than a substring. In the release the decision measured, two other
// assets carry ".zip" in the middle of their names and neither is the archive.
const ArchiveSuffix = ".zip"

// DigestLength is how many hexadecimal characters the published checksum has.
//
// MD5, and not because it is a good digest. It is the digest the server
// computes: it downloads the package, hashes it with MD5 and compares the result
// against the manifest's checksum field as a string. A SHA-256 sidecar in the
// release is a better thing to ship and is not a candidate for this field,
// because publishing its value produces a manifest that looks stronger and fails
// every install.
const DigestLength = 32

// MaxSidecarBytes bounds which assets are read looking for a checksum line.
//
// A checksum line is under a hundred bytes and an SBOM is not, so this keeps the
// search from downloading a release's whole payload to discover that none of it
// parses. The bound is stated rather than hidden: an asset larger than this is
// not read at all, and a checksum sidecar somebody pads past it would not be
// found. Nothing in the releases this reads is near it.
const MaxSidecarBytes = 4096

// Asset is one file attached to a release.
type Asset struct {
	Name string
	URL  string
	Size int64
}

// Fetch reads one asset's bytes. It is a function the caller supplies rather
// than a client this package holds, so the rules here are decided against
// fixtures and the network stays in one place.
type Fetch func(Asset) ([]byte, error)

// Pair is a resolved archive and the checksum that names it.
type Pair struct {
	Archive Asset

	// Checksum is lower-case hexadecimal, DigestLength characters. The server
	// compares case-insensitively, and one spelling in the file makes a diff
	// between two runs mean what it says.
	Checksum string

	// Sidecars names the assets whose contents were read and agreed, in the
	// order they were read, so a run can say what it believed and why.
	Sidecars []string
}

// Reason is why a release could not be paired. It is an enumeration rather than
// a string because what the run does with each case differs, and telling them
// apart is decisions/failure-posture.md's question and #28's to report.
type Reason int

const (
	// NoArchive means no asset ends in ArchiveSuffix. The release ships nothing
	// this generator recognises, which is a different thing from a repository
	// with no releases at all.
	NoArchive Reason = iota

	// AmbiguousArchive means more than one asset ends in ArchiveSuffix. The
	// tie is not broken here, because every tie-break available is an ordering
	// nobody chose, and choosing is the failure this package exists against.
	AmbiguousArchive

	// NoUsableSidecar means an archive was selected and nothing readable names
	// it in the digest the server computes. The detail says which of the two
	// shapes it was: nothing named the archive at all, or the only things that
	// named it carried a digest of another length.
	NoUsableSidecar

	// DisagreeingSidecars means two or more sidecars name the archive and their
	// digests differ. Two different answers about the same bytes is the one
	// case where publishing either would be a coin toss.
	DisagreeingSidecars
)

func (r Reason) String() string {
	switch r {
	case NoArchive:
		return "no-archive"
	case AmbiguousArchive:
		return "ambiguous-archive"
	case NoUsableSidecar:
		return "no-usable-sidecar"
	case DisagreeingSidecars:
		return "disagreeing-sidecars"
	}
	return "unknown"
}

// Refusal is a release that was not paired. It names the release, because a run
// that says a version was skipped without saying which one sends a reader to
// every release page in turn.
type Refusal struct {
	Release string
	Reason  Reason
	Detail  string
}

func (r *Refusal) Error() string {
	return fmt.Sprintf("%s: %s: %s", r.Release, r.Reason, r.Detail)
}

// SelectArchive picks the release's one archive.
//
// Position in the asset list decides nothing. Exactly one asset may satisfy the
// predicate: zero is a release the generator cannot use, and two or more is the
// ambiguity the package exists against.
func SelectArchive(release string, assets []Asset) (Asset, error) {
	var matched []Asset
	for _, a := range assets {
		if strings.HasSuffix(a.Name, ArchiveSuffix) {
			matched = append(matched, a)
		}
	}

	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return Asset{}, &Refusal{
			Release: release,
			Reason:  NoArchive,
			Detail:  fmt.Sprintf("no asset ends in %s, among %s", ArchiveSuffix, names(assets)),
		}
	default:
		return Asset{}, &Refusal{
			Release: release,
			Reason:  AmbiguousArchive,
			Detail:  fmt.Sprintf("%d assets end in %s: %s", len(matched), ArchiveSuffix, names(matched)),
		}
	}
}

// ChecksumLine is what one sidecar says: a digest and the filename it is about.
type ChecksumLine struct {
	Digest  string
	Subject string
}

// ParseChecksumLine reads a sidecar body.
//
// The format is a hexadecimal digest, whitespace, an optional asterisk marking
// binary mode, and a filename. Anything else is not a checksum line, which is
// how an SBOM, a build descriptor and a JSON document are refused on their
// contents rather than on a naming convention. That is the whole reason the
// pairing is made this way: the failure this package opens with cannot happen
// under it whatever anybody renames.
//
// Only the first line is read. A multi-file sidecar is not a shape this reads,
// and reading only the first line of one would pair the archive with whichever
// entry happened to be written first.
func ParseChecksumLine(body []byte) (ChecksumLine, bool) {
	text := strings.TrimRight(string(body), "\r\n")
	if text == "" || strings.ContainsAny(text, "\r\n") {
		return ChecksumLine{}, false
	}

	digest, rest, found := strings.Cut(strings.TrimLeft(text, " \t"), " ")
	if !found {
		digest, rest, found = strings.Cut(text, "\t")
	}
	if !found || !isHex(digest) {
		return ChecksumLine{}, false
	}

	subject := strings.TrimLeft(rest, " \t")
	subject = strings.TrimPrefix(subject, "*")
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return ChecksumLine{}, false
	}
	return ChecksumLine{Digest: strings.ToLower(digest), Subject: subject}, true
}

func isHex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// Resolve selects the archive and pairs it with its checksum.
//
// Every asset that is not the archive and is small enough to be a sidecar is
// read, and the ones whose contents name the selected archive are the
// candidates. Where more than one names it, all of them have to agree: the two
// spellings a real release carries do agree, and this does not assume it.
//
// A fetch that fails is returned as an error rather than as a Refusal. A read
// that did not happen is not a release that cannot be paired, and treating it as
// one publishes a short list that looks exactly like success.
func Resolve(release string, assets []Asset, fetch Fetch) (Pair, error) {
	archive, err := SelectArchive(release, assets)
	if err != nil {
		return Pair{}, err
	}

	var (
		agreed     string
		sidecars   []string
		disagreed  []string
		otherNamed []string
	)
	for _, a := range assets {
		if a.Name == archive.Name || a.Size > MaxSidecarBytes {
			continue
		}
		body, err := fetch(a)
		if err != nil {
			return Pair{}, fmt.Errorf("%s: reading %s: %w", release, a.Name, err)
		}
		line, ok := ParseChecksumLine(body)
		if !ok || line.Subject != archive.Name {
			continue
		}
		if len(line.Digest) != DigestLength {
			otherNamed = append(otherNamed, fmt.Sprintf("%s (%d hex characters)", a.Name, len(line.Digest)))
			continue
		}
		if agreed == "" {
			agreed = line.Digest
		} else if line.Digest != agreed {
			disagreed = append(disagreed, fmt.Sprintf("%s says %s", a.Name, line.Digest))
		}
		sidecars = append(sidecars, a.Name)
	}

	if len(disagreed) > 0 {
		sort.Strings(disagreed)
		return Pair{}, &Refusal{
			Release: release,
			Reason:  DisagreeingSidecars,
			Detail: fmt.Sprintf("%s says %s, and %s, about %s",
				sidecars[0], agreed, strings.Join(disagreed, ", "), archive.Name),
		}
	}
	if agreed == "" {
		detail := fmt.Sprintf("no asset's contents name %s", archive.Name)
		if len(otherNamed) > 0 {
			detail = fmt.Sprintf("only %s name %s, and the field carries the %d-character digest the server computes",
				strings.Join(otherNamed, ", "), archive.Name, DigestLength)
		}
		return Pair{}, &Refusal{Release: release, Reason: NoUsableSidecar, Detail: detail}
	}

	return Pair{Archive: archive, Checksum: agreed, Sidecars: sidecars}, nil
}

func names(assets []Asset) string {
	if len(assets) == 0 {
		return "no assets at all"
	}
	out := make([]string, 0, len(assets))
	for _, a := range assets {
		out = append(out, a.Name)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}
