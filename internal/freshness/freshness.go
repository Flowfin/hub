// Package freshness judges a published manifest from outside.
//
// A green publish run is not proof that the published file changed. In a
// neighbouring project a publish reported success while the manifest write had
// failed, so the release existed and nothing installable came out of it, and it
// was found when somebody asked why a version was old.
//
// What closes that gap is a check that reads this repository's own state not at
// all. It takes the bytes an operator's server would get from the address they
// pasted, parses them as that server would, and asks whether the newest finished
// release of each declared plugin is listed under each target line the catalogue
// carries for it.
//
// Per target, because two target lines share one versions array and are told
// apart only by targetAbi. A check asking only "is the newest version present"
// is satisfied by one line while the other has silently gone missing, and a
// server on the missing line sees a plugin with no installable version at all.
//
// A release is named here by the version its tag carries and not by the whole
// tag. One version is released once per live server line, each under its own
// tag, so the newest finished release reaches the catalogue under a different
// tag on each line and a comparison of whole tags refuses a catalogue that is
// current. releaseVersion is where that is decided and what it costs.
//
// It fails closed everywhere. A body that does not parse, a catalogue with no
// plugins, a plugin the catalogue does not list: each is a refusal and none is a
// pass with a shrug. The likeliest real failure of this check is a network blip,
// and reading that as evidence of freshness is worse than a false alarm.
package freshness

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Expected is one declared plugin and the newest finished release it has.
type Expected struct {
	// Slug is the name this project uses for the plugin in its own output.
	Slug string

	// Path is the declared repository, account and repository, as it appears
	// inside a release download address.
	Path string

	// Tag is the newest finished release's tag. The finished set is
	// decisions/channel-model.md's, read from the tag rather than from the
	// release's pre-release flag.
	//
	// What is compared against a catalogue entry is the release this tag names
	// rather than the tag itself, which releaseVersion below decides and says
	// why. The whole tag is still carried, because it is what a refusal quotes
	// and what somebody repairing one goes looking for.
	Tag string
}

// downloadSegment is what every release download address carries between the
// repository and the tag. Matching on it rather than on a host keeps the check
// working if the archives ever move, and keeps it from matching a link to a
// release page, which carries no asset.
const downloadSegment = "/releases/download/"

// entry is one version object, read with the field names a server reads. Only
// the two fields this check judges are decoded, so a manifest that grows a field
// does not have to be reflected here to keep this working.
type entry struct {
	Version   string `json:"version"`
	TargetABI string `json:"targetAbi"`
	SourceURL string `json:"sourceUrl"`
}

type plugin struct {
	GUID     string  `json:"guid"`
	Name     string  `json:"name"`
	Versions []entry `json:"versions"`
}

// Judge reads the published bytes and says whether every expectation holds.
//
// It returns one error naming every failure rather than the first, because
// somebody repairing three of them should not need three runs to find out.
func Judge(body []byte, expected []Expected) error {
	var plugins []plugin
	if err := json.Unmarshal(body, &plugins); err != nil {
		return fmt.Errorf("the published body does not parse as a manifest: %w", err)
	}
	if len(plugins) == 0 {
		return fmt.Errorf("the published manifest lists no plugins at all, which a server shows as an empty repository")
	}
	if len(expected) == 0 {
		return fmt.Errorf("nothing was expected of the published manifest, so this check read %d plugin(s) and judged none of them", len(plugins))
	}

	var failures []string
	for _, want := range expected {
		if err := judgeOne(plugins, want); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) == 0 {
		return nil
	}
	sort.Strings(failures)
	return fmt.Errorf("the published manifest is not current:\n  %s", strings.Join(failures, "\n  "))
}

func judgeOne(plugins []plugin, want Expected) error {
	needle := "/" + strings.Trim(want.Path, "/") + downloadSegment

	// tagsPerTarget is what the catalogue offers for this plugin, per target
	// line. The target set is derived from the published file rather than
	// declared, which is the bound this check has: a target line that vanished
	// from the manifest entirely leaves nothing here to compare against.
	tagsPerTarget := map[string]map[string]bool{}
	for _, p := range plugins {
		for _, v := range p.Versions {
			i := strings.Index(v.SourceURL, needle)
			if i < 0 {
				continue
			}
			tag, _, _ := strings.Cut(v.SourceURL[i+len(needle):], "/")
			if tag == "" {
				return fmt.Errorf("%s: an entry's sourceUrl names no release tag: %s", want.Slug, v.SourceURL)
			}
			if tagsPerTarget[v.TargetABI] == nil {
				tagsPerTarget[v.TargetABI] = map[string]bool{}
			}
			tagsPerTarget[v.TargetABI][tag] = true
		}
	}

	if len(tagsPerTarget) == 0 {
		return fmt.Errorf("%s: the published manifest lists nothing from %s, so a server sees no plugin rather than an old one",
			want.Slug, want.Path)
	}

	wanted := releaseVersion(want.Tag)
	var missing []string
	for target, tags := range tagsPerTarget {
		if !carries(tags, wanted) {
			missing = append(missing, fmt.Sprintf("%s (it carries %s)", target, joinSorted(tags)))
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("%s: the newest finished release %s is not listed under %s",
		want.Slug, want.Tag, strings.Join(missing, ", "))
}

// carries says whether a target line offers the release wanted names.
//
// The comparison is per tag rather than over a set of stripped tags, because
// two tags naming one release differ only in segments releaseVersion drops, and
// a set of those would have to be built to be thrown away.
func carries(tags map[string]bool, wanted string) bool {
	for tag := range tags {
		if releaseVersion(tag) == wanted {
			return true
		}
	}
	return false
}

// releaseVersion is the part of a tag that says WHICH release it is, with the
// parts that say where the release goes removed.
//
// A tag admitted by a declaration's stable_tags pattern carries three kinds of
// thing, and only one of them identifies the release. An optional leading v is
// spelling. A trailing -stable names the channel, which
// decisions/channel-model.md reads out of the tag rather than out of the
// release's flag. And a -jf12 segment names the server line a file was cut for,
// under the one-version-many-lines decision the requests declaration's note
// carries: one version is released once per live line, each release under its
// own tag, and the catalogue tells the lines apart by targetAbi rather than by
// that segment.
//
// So one release version reaches the catalogue under several tags, and a check
// asking for one tag under every target line asks for a tag that was never cut
// for most of them. That is what this repairs, and it is the whole of the
// change: the numeric components are compared and the decoration is not.
//
// What it gives up is the ability to tell two finished releases apart that
// share a version and differ only in a suffix. Under the decision above that is
// the intended reading, because such a pair IS one version cut for two lines.
// A pair that is not - one version retagged for some other reason - is read as
// current here when only one of the two is published, and no reading of a tag
// separates those two cases.
//
// It refuses nothing this rule did not refuse before. Equal tags have equal
// versions, so every catalogue the whole-tag comparison passed is passed here
// too, and the change can only stop refusals rather than start them.
//
// A tag with no numeric run at its front is handed back whole. Returning the
// empty string for it would make every such tag equal to every other, which is
// the one direction a comparison here must not fail in.
func releaseVersion(tag string) string {
	rest := strings.TrimPrefix(tag, "v")
	end := 0
	for end < len(rest) && (rest[end] == '.' || (rest[end] >= '0' && rest[end] <= '9')) {
		end++
	}
	if end == 0 {
		return tag
	}
	return rest[:end]
}

func joinSorted(set map[string]bool) string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}
