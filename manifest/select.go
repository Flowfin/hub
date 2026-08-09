package manifest

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Cap is how many versions of one plugin are published per target ABI, from
// decisions/version-cap.md. It is a number in one place, and changing it is one
// edit.
const Cap = 5

// Components is how many parts a version number carries. The server parses this
// field as a version, and decisions/manifest-schema.md fixes it at four.
const Components = 4

// Number is a version parsed into its components, which is what the ordering
// compares. A string comparison puts 2.4.0.10 below 2.4.0.9 and the server puts
// it above, so the two would disagree about which build is newest.
type Number [Components]int

// ParseNumber reads a version string into its components.
//
// Fewer than four components is read as the missing trailing ones being zero,
// because 1.0.9 and 1.0.9.0 are the same build and a reader writing either of
// them means the same thing. More than four, a component that is not a
// non-negative integer, or an empty string is refused: this function decides
// order and cannot invent one for a string it cannot read.
//
// Refusing to PUBLISH such a version is not this function's job.
// decisions/failure-posture.md holds what a run does with a version it cannot
// use, and #28 is where that is reported. What happens here is only that the
// order stays total, which Order does by putting the unreadable ones last.
func ParseNumber(version string) (Number, error) {
	var n Number
	if version == "" {
		return n, fmt.Errorf("an empty version has no order")
	}
	parts := strings.Split(version, ".")
	if len(parts) > Components {
		return n, fmt.Errorf("version %q has %d components and the format carries %d", version, len(parts), Components)
	}
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 || strings.TrimSpace(p) != p {
			return Number{}, fmt.Errorf("version %q: component %d is %q, which is not a non-negative integer", version, i+1, p)
		}
		n[i] = v
	}
	return n, nil
}

// CompareNumbers orders two parsed versions, oldest first: negative when a is
// older than b, positive when it is newer, zero when they are equal.
func CompareNumbers(a, b Number) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// newerFirst reports whether version a sorts before version b under the rule in
// decisions/manifest-schema.md: newest first by the four-component number, ties
// broken by timestamp descending and then by targetAbi descending, so the order
// is total and does not depend on the order the releases came back from the API.
//
// A version string that does not parse sorts after every one that does, and
// those are ordered against each other by their raw string descending. That is
// not a judgement about such a version; it is what keeps the order total so that
// two runs over the same input cannot disagree.
func newerFirst(a, b Version) bool {
	na, aErr := ParseNumber(a.Version)
	nb, bErr := ParseNumber(b.Version)
	switch {
	case aErr == nil && bErr != nil:
		return true
	case aErr != nil && bErr == nil:
		return false
	case aErr == nil && bErr == nil:
		if c := CompareNumbers(na, nb); c != 0 {
			return c > 0
		}
	default:
		if a.Version != b.Version {
			return a.Version > b.Version
		}
	}
	if a.Timestamp != b.Timestamp {
		return a.Timestamp > b.Timestamp
	}
	if a.TargetABI != b.TargetABI {
		return a.TargetABI > b.TargetABI
	}
	// Everything the rule names is equal. The remaining fields decide, so that
	// two such entries do not sit in whatever order the input happened to hold,
	// which is the difference between a stable sort and a total order.
	if a.SourceURL != b.SourceURL {
		return a.SourceURL > b.SourceURL
	}
	if a.Checksum != b.Checksum {
		return a.Checksum > b.Checksum
	}
	return a.Changelog > b.Changelog
}

// OrderVersions sorts one plugin's versions in place, newest first.
func OrderVersions(versions []Version) {
	sort.SliceStable(versions, func(i, j int) bool { return newerFirst(versions[i], versions[j]) })
}

// OrderPlugins sorts plugins in place by guid, ascending, byte by byte.
//
// Not by name. A guid is fixed for the life of a plugin and a name is not, so
// ordering by name means a rename moves an entry and produces a diff whose size
// has nothing to do with what changed. Byte-wise rather than by any collation,
// because a locale-dependent order differs between the machine that generated
// the file and the machine that reviewed it.
func OrderPlugins(plugins []Plugin) {
	sort.SliceStable(plugins, func(i, j int) bool { return plugins[i].GUID < plugins[j].GUID })
}

// CapPerTarget keeps the newest n versions of each target ABI and drops the
// rest.
//
// Per target rather than across the list, which is the whole of
// decisions/version-cap.md. A server filters the versions array by comparing its
// own version against each entry's targetAbi, so an overall cap lets a
// fast-publishing target line push a slow one off the end. From then on a server
// on the slower line sees a plugin with no installable version at all: nothing is
// red, the file is well-formed, and the plugin has quietly disappeared for one
// group of users.
//
// The returned slice is ordered newest first across every target, because the
// cap decides which entries are present and the ordering rule decides where they
// sit. n below one keeps nothing, which is refused rather than published as an
// empty versions array; decisions/failure-posture.md is where that is decided.
func CapPerTarget(versions []Version, n int) []Version {
	if n < 1 {
		return nil
	}

	byTarget := map[string][]Version{}
	var targets []string
	for _, v := range versions {
		if _, seen := byTarget[v.TargetABI]; !seen {
			targets = append(targets, v.TargetABI)
		}
		byTarget[v.TargetABI] = append(byTarget[v.TargetABI], v)
	}

	kept := make([]Version, 0, len(versions))
	for _, t := range targets {
		group := byTarget[t]
		OrderVersions(group)
		if len(group) > n {
			group = group[:n]
		}
		kept = append(kept, group...)
	}
	OrderVersions(kept)
	return kept
}

// Select is the pass that turns whatever the releases produced into the list the
// file publishes: the cap applied per target, the versions ordered newest first
// across every target, and the plugins ordered by guid.
//
// It does not copy. The caller's slices are reordered and each plugin's versions
// slice is replaced, which is what makes running it twice over the same input
// produce the same bytes rather than the same values in a new order.
func Select(plugins []Plugin, n int) []Plugin {
	for i := range plugins {
		plugins[i].Versions = CapPerTarget(plugins[i].Versions, n)
	}
	OrderPlugins(plugins)
	return plugins
}
