// Package identity reads a plugin's identity out of the descriptor a release
// ships, and refuses a plugin whose identity cannot be read rather than
// publishing an entry with holes in it.
//
// The rule for which release supplies the values, which asset carries them, and
// which of them may be absent is decisions/plugin-identity.md. The short form is
// that the newest release carrying a readable descriptor answers for the plugin,
// whichever channel that release is in, and that there is no fallback to the one
// behind it.
//
// Nothing here reaches the network. Descriptor bodies arrive as bytes the caller
// read, for the same reason internal/pairing takes a Fetch: every rule below is
// then judged against a fixture in the gate, and decisions/headless-and-unelevated.md
// keeps the one thing that leaves the runner out of it.
//
// The descriptor also carries the version entry's fields. Reading those is not
// this package's, and none of them is looked at here except `version`, which the
// ordering rule above is written in terms of.
package identity

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"flowfin.dev/hub/internal/pairing"
	"flowfin.dev/hub/manifest"
)

// DescriptorSuffix is what a descriptor's name carries after the archive's own.
//
// A suffix on the archive name rather than a name of its own, so selecting it is
// the archive selection that already exists plus one string. A release that ever
// ships two archives then has two descriptors and neither of them is ambiguous,
// which is the property decisions/artifact-checksum-pairing.md spends its whole
// argument on and the reason this file is preferred to the per-release one.
const DescriptorSuffix = ".meta.json"

// Fields are the plugin-level values of a manifest entry.
//
// Not manifest.Plugin, because a Plugin without versions is a thing this package
// would be inviting a caller to publish. Entry is where the two are joined, so a
// field added to the schema is a compile error here rather than a key that
// quietly stops being emitted.
type Fields struct {
	GUID        string
	Name        string
	Description string
	Overview    string
	Owner       string
	Category    string

	// ImageURL is the one value that may be absent. Where a plugin has no
	// artwork the manifest omits the key rather than emitting it empty.
	ImageURL string
}

// Entry returns the manifest entry these fields identify, carrying versions.
func (f Fields) Entry(versions []manifest.Version) manifest.Plugin {
	return manifest.Plugin{
		GUID:        f.GUID,
		Name:        f.Name,
		Description: f.Description,
		Overview:    f.Overview,
		Owner:       f.Owner,
		Category:    f.Category,
		ImageURL:    f.ImageURL,
		Versions:    versions,
	}
}

// Reason is why a plugin's identity could not be read. An enumeration rather
// than a string because decisions/failure-posture.md does different things with
// a defect in the newest release and a defect in an older one, and #28 is where
// the two are told apart.
type Reason int

const (
	// NoDescriptor means no asset is named after the archive with
	// DescriptorSuffix behind it. The release ships an archive nothing
	// describes.
	NoDescriptor Reason = iota

	// UnreadableDescriptor means the descriptor is not JSON.
	UnreadableDescriptor

	// IncompleteDescriptor means the descriptor parsed and a value the schema
	// requires is absent or is whitespace. Every missing field is named at
	// once, so a release is fixed in one pass rather than one field per run.
	IncompleteDescriptor

	// MalformedIdentifier means guid is present and is not the shape a server
	// parses. Its own reason rather than part of the one above, because a
	// missing identifier is a descriptor somebody has not finished writing and
	// a malformed one is a descriptor somebody believes is finished.
	MalformedIdentifier

	// NoReleaseCarriesOne means the plugin has releases and not one of them
	// carries a descriptor this package can read. Different from a plugin with
	// no releases at all, which is decisions/failure-posture.md's counted and
	// reported case rather than a refusal.
	NoReleaseCarriesOne
)

func (r Reason) String() string {
	switch r {
	case NoDescriptor:
		return "no-descriptor"
	case UnreadableDescriptor:
		return "unreadable-descriptor"
	case IncompleteDescriptor:
		return "incomplete-descriptor"
	case MalformedIdentifier:
		return "malformed-identifier"
	case NoReleaseCarriesOne:
		return "no-release-carries-one"
	}
	return "unknown"
}

// Refusal is a plugin whose identity was not read.
//
// It names the plugin first and always. A run that says identity could not be
// read without saying whose sends a reader to every declared repository in turn,
// which is the reporting failure #28 exists against one layer up.
type Refusal struct {
	Plugin string

	// Release is the release the refusal is about, and is empty only for
	// NoReleaseCarriesOne, which is about the plugin rather than about one
	// release.
	Release string

	Reason Reason
	Detail string
}

func (r *Refusal) Error() string {
	if r.Release == "" {
		return fmt.Sprintf("%s: %s: %s", r.Plugin, r.Reason, r.Detail)
	}
	return fmt.Sprintf("%s: %s: %s: %s", r.Plugin, r.Release, r.Reason, r.Detail)
}

// Required is the set of descriptor fields a manifest entry cannot be published
// without, in the order the manifest emits them.
//
// Listed here rather than derived from the struct, because the struct also holds
// the one optional field and a check that read the struct would either demand
// the image or demand nothing.
var Required = []string{"guid", "name", "description", "overview", "owner", "category"}

// descriptor is the part of the sidecar this package reads. The version entry's
// own fields are deliberately absent: `version` is here because the rule for
// which release answers is written in terms of it, and targetAbi, changelog and
// timestamp are not this package's to read.
type descriptor struct {
	GUID        string `json:"guid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Overview    string `json:"overview"`
	Owner       string `json:"owner"`
	Category    string `json:"category"`
	ImageURL    string `json:"imageUrl"`
	Version     string `json:"version"`
}

// SelectDescriptor names the asset that describes the release's archive.
//
// The archive is selected first, by internal/pairing, so that this package does
// not grow a second opinion about which file the release ships. A descriptor
// selected by its own predicate would be a second predicate to keep in step with
// the first, and the two coming apart is exactly the failure that package opens
// with.
func SelectDescriptor(plugin, release string, assets []pairing.Asset) (pairing.Asset, error) {
	archive, err := pairing.SelectArchive(release, assets)
	if err != nil {
		return pairing.Asset{}, err
	}

	want := archive.Name + DescriptorSuffix
	for _, a := range assets {
		if a.Name == want {
			return a, nil
		}
	}
	return pairing.Asset{}, &Refusal{
		Plugin:  plugin,
		Release: release,
		Reason:  NoDescriptor,
		Detail:  fmt.Sprintf("no asset is named %s, among %s", want, names(assets)),
	}
}

// Read parses one descriptor body into the plugin-level fields.
//
// Every value is trimmed of surrounding whitespace before it is judged and
// before it is kept, so a field holding a newline is the absence it looks like
// rather than a value that renders as a blank line on every server.
func Read(plugin, release string, body []byte) (Fields, error) {
	var d descriptor
	if err := json.Unmarshal(body, &d); err != nil {
		return Fields{}, &Refusal{
			Plugin:  plugin,
			Release: release,
			Reason:  UnreadableDescriptor,
			Detail:  err.Error(),
		}
	}

	f := Fields{
		GUID:        strings.TrimSpace(d.GUID),
		Name:        strings.TrimSpace(d.Name),
		Description: strings.TrimSpace(d.Description),
		Overview:    strings.TrimSpace(d.Overview),
		Owner:       strings.TrimSpace(d.Owner),
		Category:    strings.TrimSpace(d.Category),
		ImageURL:    strings.TrimSpace(d.ImageURL),
	}

	present := map[string]string{
		"guid":        f.GUID,
		"name":        f.Name,
		"description": f.Description,
		"overview":    f.Overview,
		"owner":       f.Owner,
		"category":    f.Category,
	}
	var missing []string
	for _, field := range Required {
		if present[field] == "" {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return Fields{}, &Refusal{
			Plugin:  plugin,
			Release: release,
			Reason:  IncompleteDescriptor,
			Detail: fmt.Sprintf("%d of %d required field(s) absent or blank: %s",
				len(missing), len(Required), strings.Join(missing, ", ")),
		}
	}

	guid, ok := canonicalGUID(f.GUID)
	if !ok {
		return Fields{}, &Refusal{
			Plugin:  plugin,
			Release: release,
			Reason:  MalformedIdentifier,
			Detail: fmt.Sprintf("guid is %q, and the field a server matches an installed plugin against is %s",
				f.GUID, guidShape),
		}
	}
	f.GUID = guid

	return f, nil
}

// guidShape says what a guid looks like, in the words the refusal prints.
const guidShape = "8-4-4-4-12 hexadecimal characters separated by hyphens"

// guidGroups is that shape as the lengths of its groups.
var guidGroups = []int{8, 4, 4, 4, 12}

// canonicalGUID reports whether the value is the shape a server parses, and
// returns it lowercased.
//
// Lowercased rather than refused for case, because decisions/manifest-schema.md
// fixes the published spelling and a descriptor written in upper case is a
// spelling rather than a different plugin. The shape itself is refused, because
// a value a server cannot parse is an entry it treats as a plugin it has never
// seen, and nothing in the interface says so.
func canonicalGUID(value string) (string, bool) {
	groups := strings.Split(value, "-")
	if len(groups) != len(guidGroups) {
		return "", false
	}
	for i, g := range groups {
		if len(g) != guidGroups[i] || !isHex(g) {
			return "", false
		}
	}
	return strings.ToLower(value), true
}

func isHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return s != ""
}

// Bearing is one release's descriptor, as far as the choice between releases
// needs it: which release it came from and what it says.
type Bearing struct {
	Release string
	Body    []byte
}

// Of reads the plugin's identity from the newest release that carries a
// descriptor.
//
// Newest is the four-component number the descriptor declares in `version`,
// compared as manifest compares versions, so the answer does not depend on the
// order the releases came back from the API. A body whose version cannot be read
// sorts after every one that can, and those are ordered against each other by
// release tag descending, which keeps the order total rather than passing
// judgement on such a release.
//
// There is no fallback. If the newest bearing does not read, the refusal is
// returned rather than the one behind it being tried, because a silently older
// answer is a name the plugin has stopped using published as if it were current.
func Of(plugin string, bearings []Bearing) (Fields, error) {
	if len(bearings) == 0 {
		return Fields{}, &Refusal{
			Plugin: plugin,
			Reason: NoReleaseCarriesOne,
			Detail: "no release of this plugin carries a descriptor beside its archive",
		}
	}

	ordered := make([]Bearing, len(bearings))
	copy(ordered, bearings)
	sort.SliceStable(ordered, func(i, j int) bool { return newerFirst(ordered[i], ordered[j]) })

	return Read(plugin, ordered[0].Release, ordered[0].Body)
}

// declaredVersion reads only the version out of a body, for the ordering. A body
// that is not JSON has no version and is ordered as if the field were unreadable,
// which puts it last rather than first: a descriptor nothing can parse must not
// win the choice and then refuse the plugin from a position it reached by being
// unreadable.
func declaredVersion(body []byte) (manifest.Number, bool) {
	var d descriptor
	if err := json.Unmarshal(body, &d); err != nil {
		return manifest.Number{}, false
	}
	n, err := manifest.ParseNumber(strings.TrimSpace(d.Version))
	if err != nil {
		return manifest.Number{}, false
	}
	return n, true
}

func newerFirst(a, b Bearing) bool {
	na, aOK := declaredVersion(a.Body)
	nb, bOK := declaredVersion(b.Body)
	switch {
	case aOK && !bOK:
		return true
	case !aOK && bOK:
		return false
	case aOK && bOK:
		if c := manifest.CompareNumbers(na, nb); c != 0 {
			return c > 0
		}
	}
	return a.Release > b.Release
}

func names(assets []pairing.Asset) string {
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
