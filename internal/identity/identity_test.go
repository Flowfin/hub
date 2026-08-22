package identity

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"flowfin.dev/hub/internal/pairing"
	"flowfin.dev/hub/manifest"
)

// The shape of a real descriptor, from the release decisions/plugin-identity.md
// measured. The values are that file's field set; the identifier is invented,
// because a fixture carrying a real guid would be a claim about a plugin rather
// than a fixture.
const (
	archiveName = "community-sso-for-jellyfin_5.0.0.43.zip"
	fixtureGUID = "1f0f5d4a-4a2c-4c6d-8e0a-2b4c6d8e0a2b"
)

func descriptorBody(t *testing.T, edit func(map[string]any)) []byte {
	t.Helper()
	d := map[string]any{
		"guid":        fixtureGUID,
		"name":        "Community SSO for Jellyfin",
		"description": "Single sign-on for Jellyfin via OpenID Connect and SAML 2.0.\n",
		"overview":    "Sign in through an identity provider.",
		"owner":       "publisher",
		"category":    "Authentication",
		"imageUrl":    "https://example.test/logo.png",
		"targetAbi":   "12.0.0.0",
		"timestamp":   "2026-08-09T05:16:30Z",
		"version":     "5.0.0.43",
	}
	if edit != nil {
		edit(d)
	}
	body, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("building the fixture: %v", err)
	}
	return body
}

func refusal(t *testing.T, err error) *Refusal {
	t.Helper()
	if err == nil {
		t.Fatal("wanted a refusal and got none")
	}
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("wanted a *Refusal and got %T: %v", err, err)
	}
	return r
}

func TestAReadableDescriptorSuppliesEveryPluginLevelField(t *testing.T) {
	got, err := Read("sso", "5.0.0-JF12-beta.43", descriptorBody(t, nil))
	if err != nil {
		t.Fatalf("reading a complete descriptor: %v", err)
	}

	want := Fields{
		GUID:        fixtureGUID,
		Name:        "Community SSO for Jellyfin",
		Description: "Single sign-on for Jellyfin via OpenID Connect and SAML 2.0.",
		Overview:    "Sign in through an identity provider.",
		Owner:       "publisher",
		Category:    "Authentication",
		ImageURL:    "https://example.test/logo.png",
	}
	if got != want {
		t.Errorf("read\n got %+v\nwant %+v", got, want)
	}
}

// The owner is read from the release rather than supplied here, so this generator
// is not a second place the name is decided. That is decisions/names-are-data.md
// rather than entry 4 of #1: entry 4 says which name the catalogue carries and
// this says where any name comes from, and the two were once written as one. A
// generator that substituted its own answer would also be refused by
// no-hardcoded-names.
func TestTheOwnerComesFromTheDescriptorAndIsNotSubstituted(t *testing.T) {
	body := descriptorBody(t, func(d map[string]any) { d["owner"] = "somebody-else" })

	got, err := Read("sso", "r", body)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if got.Owner != "somebody-else" {
		t.Errorf("owner is %q, and the descriptor said %q", got.Owner, "somebody-else")
	}
}

func TestAMissingRequiredFieldRefusesThePluginByName(t *testing.T) {
	for _, field := range Required {
		t.Run(field, func(t *testing.T) {
			body := descriptorBody(t, func(d map[string]any) { delete(d, field) })

			_, err := Read("watchlist", "1.0.0.0", body)
			r := refusal(t, err)

			if r.Reason != IncompleteDescriptor {
				t.Errorf("reason is %s, want %s", r.Reason, IncompleteDescriptor)
			}
			if r.Plugin != "watchlist" {
				t.Errorf("refusal names the plugin %q, want %q", r.Plugin, "watchlist")
			}
			if !strings.Contains(r.Detail, field) {
				t.Errorf("detail %q does not name the missing field %q", r.Detail, field)
			}
		})
	}
}

// Whitespace is absence. A descriptor carrying a newline in `overview` renders as
// a blank line on every server that lists the plugin, and a check that only
// tested for the empty string would pass it.
func TestAWhitespaceOnlyRequiredFieldIsTheAbsenceItLooksLike(t *testing.T) {
	body := descriptorBody(t, func(d map[string]any) { d["overview"] = "  \n\t " })

	_, err := Read("stats", "r", body)
	r := refusal(t, err)

	if r.Reason != IncompleteDescriptor {
		t.Fatalf("reason is %s, want %s", r.Reason, IncompleteDescriptor)
	}
	if !strings.Contains(r.Detail, "overview") {
		t.Errorf("detail %q does not name overview", r.Detail)
	}
}

// Every missing field at once, so a release is repaired in one pass. A refusal
// naming the first one it met teaches whoever is fixing it to run the generator
// once per field.
func TestEveryMissingFieldIsNamedInOneRefusal(t *testing.T) {
	body := descriptorBody(t, func(d map[string]any) {
		delete(d, "overview")
		delete(d, "category")
		d["description"] = ""
	})

	_, err := Read("requests", "r", body)
	r := refusal(t, err)

	for _, field := range []string{"overview", "category", "description"} {
		if !strings.Contains(r.Detail, field) {
			t.Errorf("detail %q does not name %q", r.Detail, field)
		}
	}
	if !strings.Contains(r.Detail, fmt.Sprintf("3 of %d", len(Required))) {
		t.Errorf("detail %q does not count the missing fields against the required set", r.Detail)
	}
}

// The image is the one field a plugin may be without, and the manifest omits the
// key rather than emitting it empty. Measured against the encoder, because the
// omission is a struct tag rather than a decision this package can assert.
func TestAPluginWithNoArtworkIsPublishedWithoutTheKey(t *testing.T) {
	body := descriptorBody(t, func(d map[string]any) { delete(d, "imageUrl") })

	got, err := Read("invites", "r", body)
	if err != nil {
		t.Fatalf("reading a descriptor with no image: %v", err)
	}
	if got.ImageURL != "" {
		t.Errorf("image is %q, want empty", got.ImageURL)
	}

	var buf bytes.Buffer
	if err := manifest.Encode(&buf, []manifest.Plugin{got.Entry(nil)}); err != nil {
		t.Fatalf("encoding: %v", err)
	}
	if strings.Contains(buf.String(), "imageUrl") {
		t.Errorf("the entry carries an imageUrl key with no image behind it:\n%s", buf.String())
	}
}

func TestAnIdentifierOfTheWrongShapeIsRefused(t *testing.T) {
	for _, value := range []string{
		"505ce9d1d91642fa86ca673ef241d7df",             // the hyphens gone
		"505ce9d1-d916-42fa-86ca",                      // a group short
		"505ce9d1-d916-42fa-86ca-673ef241d7df-extra",   // a group long
		"505ce9d1-d916-42fa-86ca-673ef241d7dg",         // not hexadecimal
		"505ce9d-1d916-42fa-86ca-673ef241d7df",         // the hyphen in the wrong place
		"the-plugin-identifier-goes-here-000000000000", // a placeholder somebody left in
	} {
		t.Run(value, func(t *testing.T) {
			body := descriptorBody(t, func(d map[string]any) { d["guid"] = value })

			_, err := Read("sso", "r", body)
			r := refusal(t, err)

			if r.Reason != MalformedIdentifier {
				t.Errorf("reason is %s, want %s", r.Reason, MalformedIdentifier)
			}
			if !strings.Contains(r.Detail, value) {
				t.Errorf("detail %q does not quote the value it refused", r.Detail)
			}
		})
	}
}

// Case is a spelling of the same identifier and not a different plugin, so it is
// published in the one spelling decisions/manifest-schema.md fixes rather than
// refused.
func TestAnUpperCaseIdentifierIsPublishedLowerCase(t *testing.T) {
	body := descriptorBody(t, func(d map[string]any) { d["guid"] = strings.ToUpper(fixtureGUID) })

	got, err := Read("sso", "r", body)
	if err != nil {
		t.Fatalf("reading an upper-case identifier: %v", err)
	}
	if got.GUID != fixtureGUID {
		t.Errorf("guid is %q, want %q", got.GUID, fixtureGUID)
	}
}

func TestADescriptorThatIsNotJSONIsRefusedByName(t *testing.T) {
	_, err := Read("share-links", "2.0.0.0", []byte("name: a build descriptor in the other format\n"))
	r := refusal(t, err)

	if r.Reason != UnreadableDescriptor {
		t.Errorf("reason is %s, want %s", r.Reason, UnreadableDescriptor)
	}
	if r.Plugin != "share-links" || r.Release != "2.0.0.0" {
		t.Errorf("refusal is about %s/%s, want share-links/2.0.0.0", r.Plugin, r.Release)
	}
}

func assets(names ...string) []pairing.Asset {
	out := make([]pairing.Asset, 0, len(names))
	for _, n := range names {
		out = append(out, pairing.Asset{Name: n, URL: "https://example.test/download/" + n, Size: 1024})
	}
	return out
}

func TestTheDescriptorIsTheOneNamedAfterTheSelectedArchive(t *testing.T) {
	// Deliberately not in sorted order, and with a second .meta.json that
	// belongs to something else, so a selection reaching for the first or the
	// last file with that suffix is visibly wrong rather than accidentally
	// right.
	got, err := SelectDescriptor("sso", "5.0.0-JF12-beta.43", assets(
		"sbom.cyclonedx.json.meta.json",
		"build.yaml",
		archiveName+DescriptorSuffix,
		archiveName,
		"community-sso-for-jellyfin_5.0.0.43.md5",
	))
	if err != nil {
		t.Fatalf("selecting: %v", err)
	}
	if got.Name != archiveName+DescriptorSuffix {
		t.Errorf("selected %q, want %q", got.Name, archiveName+DescriptorSuffix)
	}
}

// The four finished releases in the declared set are this shape: an archive, an
// md5 and a sha256, and nothing describing any of them.
func TestAReleaseWithNoDescriptorBesideItsArchiveIsRefusedByName(t *testing.T) {
	_, err := SelectDescriptor("sso", "4.2.1-stable", assets(
		"sso-authentication_4.2.1.0.md5",
		"sso-authentication_4.2.1.0.sha256",
		"sso-authentication_4.2.1.0.zip",
	))
	r := refusal(t, err)

	if r.Reason != NoDescriptor {
		t.Errorf("reason is %s, want %s", r.Reason, NoDescriptor)
	}
	if r.Plugin != "sso" || r.Release != "4.2.1-stable" {
		t.Errorf("refusal is about %s/%s, want sso/4.2.1-stable", r.Plugin, r.Release)
	}
	if !strings.Contains(r.Detail, "sso-authentication_4.2.1.0.zip"+DescriptorSuffix) {
		t.Errorf("detail %q does not say which name it looked for", r.Detail)
	}
}

// A release this generator cannot choose an archive in has no descriptor to
// choose either, and the refusal that says so is internal/pairing's rather than a
// second opinion written here.
func TestAnUnselectableArchiveCarriesThePairingRefusalThrough(t *testing.T) {
	_, err := SelectDescriptor("sso", "r", assets("one.zip", "two.zip"))
	if err == nil {
		t.Fatal("wanted a refusal and got none")
	}
	var mine *Refusal
	if errors.As(err, &mine) {
		t.Fatalf("this package answered for an ambiguous archive: %v", err)
	}
	var theirs *pairing.Refusal
	if !errors.As(err, &theirs) {
		t.Fatalf("wanted internal/pairing's refusal and got %T: %v", err, err)
	}
	if theirs.Reason != pairing.AmbiguousArchive {
		t.Errorf("reason is %s, want %s", theirs.Reason, pairing.AmbiguousArchive)
	}
}

func bearing(t *testing.T, release, version, name string) Bearing {
	t.Helper()
	return Bearing{Release: release, Body: descriptorBody(t, func(d map[string]any) {
		d["version"] = version
		d["name"] = name
	})}
}

// The rule is the version the descriptor declares, not the order the API
// returned. The input below is in the order the releases endpoint uses, newest
// creation first, and the newest version is not first in it.
func TestIdentityComesFromTheNewestDeclaredVersionAndNotFromTheInputOrder(t *testing.T) {
	bearings := []Bearing{
		bearing(t, "4.3.0-beta.28", "4.3.0.28", "the old name"),
		bearing(t, "5.0.0-JF12-beta.43", "5.0.0.43", "the current name"),
		bearing(t, "4.2.1-stable", "4.2.1.0", "the older name"),
	}

	got, err := Of("sso", bearings)
	if err != nil {
		t.Fatalf("choosing: %v", err)
	}
	if got.Name != "the current name" {
		t.Errorf("name is %q, want the one the newest version declares", got.Name)
	}
}

// A string comparison puts 5.0.0.9 above 5.0.0.10 and the server puts it below,
// so the two would disagree about which build the catalogue is named after.
func TestTheChoiceComparesVersionsAsNumbers(t *testing.T) {
	got, err := Of("sso", []Bearing{
		bearing(t, "a", "5.0.0.9", "nine"),
		bearing(t, "b", "5.0.0.10", "ten"),
	})
	if err != nil {
		t.Fatalf("choosing: %v", err)
	}
	if got.Name != "ten" {
		t.Errorf("name is %q, want ten", got.Name)
	}
}

// Two runs over one history cannot disagree, so the order is total rather than
// merely stable. Every rotation of the same set gives the same answer.
func TestTheChoiceDoesNotDependOnWhereTheListStarts(t *testing.T) {
	base := []Bearing{
		bearing(t, "a", "1.0.0.0", "one"),
		bearing(t, "b", "2.0.0.0", "two"),
		bearing(t, "c", "2.0.0.0", "two again"),
		bearing(t, "d", "0.9.0.0", "nine"),
	}

	var first string
	for i := range base {
		rotated := append(append([]Bearing{}, base[i:]...), base[:i]...)
		got, err := Of("sso", rotated)
		if err != nil {
			t.Fatalf("rotation %d: %v", i, err)
		}
		if i == 0 {
			first = got.Name
			continue
		}
		if got.Name != first {
			t.Errorf("rotation %d chose %q and rotation 0 chose %q", i, got.Name, first)
		}
	}
}

// A descriptor nothing can parse must not win the choice. If it did, it would
// reach the front by being unreadable and then refuse the plugin from there,
// which is a plugin removed from the catalogue by the one release least able to
// say anything about it.
func TestAnUnreadableDescriptorDoesNotWinTheChoice(t *testing.T) {
	got, err := Of("sso", []Bearing{
		{Release: "broken", Body: []byte("not json at all")},
		bearing(t, "good", "1.0.0.0", "the readable one"),
	})
	if err != nil {
		t.Fatalf("choosing: %v", err)
	}
	if got.Name != "the readable one" {
		t.Errorf("name is %q, want the readable one", got.Name)
	}
}

// No fallback. The newest release answering wrongly is a build published wrong
// today, and reading the one behind it publishes a name the plugin has stopped
// using as if it were current.
func TestABrokenNewestDescriptorIsNotWorkedAroundByReadingAnOlderOne(t *testing.T) {
	newest := bearing(t, "5.0.0-JF12-beta.43", "5.0.0.43", "the current name")
	newest.Body = descriptorBody(t, func(d map[string]any) {
		d["version"] = "5.0.0.43"
		delete(d, "guid")
	})

	_, err := Of("sso", []Bearing{
		bearing(t, "4.2.1-stable", "4.2.1.0", "the older name"),
		newest,
	})
	r := refusal(t, err)

	if r.Reason != IncompleteDescriptor {
		t.Errorf("reason is %s, want %s", r.Reason, IncompleteDescriptor)
	}
	if r.Release != "5.0.0-JF12-beta.43" {
		t.Errorf("refusal is about %q, want the newest release", r.Release)
	}
}

func TestAPluginNoReleaseOfWhichCarriesADescriptorIsRefusedByName(t *testing.T) {
	_, err := Of("metadata-sync", nil)
	r := refusal(t, err)

	if r.Reason != NoReleaseCarriesOne {
		t.Errorf("reason is %s, want %s", r.Reason, NoReleaseCarriesOne)
	}
	if r.Plugin != "metadata-sync" {
		t.Errorf("refusal names %q, want metadata-sync", r.Plugin)
	}
	if r.Release != "" {
		t.Errorf("refusal names the release %q, and this one is about the plugin", r.Release)
	}
}

// Entry is the one place the descriptor's fields become the schema's, so a field
// dropped on the way through is a wrong catalogue rather than a compile error.
// This reads the emitted keys rather than the struct, because the emitted file is
// what a server sees.
func TestEveryFieldReachesTheEmittedEntry(t *testing.T) {
	f := Fields{
		GUID:        fixtureGUID,
		Name:        "a name",
		Description: "a description",
		Overview:    "an overview",
		Owner:       "an owner",
		Category:    "a category",
		ImageURL:    "https://example.test/logo.png",
	}

	var buf bytes.Buffer
	if err := manifest.Encode(&buf, []manifest.Plugin{f.Entry([]manifest.Version{{Version: "1.0.0.0"}})}); err != nil {
		t.Fatalf("encoding: %v", err)
	}

	var back []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("reading the emitted entry: %v", err)
	}
	if len(back) != 1 {
		t.Fatalf("emitted %d entries, want 1", len(back))
	}

	for key, want := range map[string]string{
		"guid":        f.GUID,
		"name":        f.Name,
		"description": f.Description,
		"overview":    f.Overview,
		"owner":       f.Owner,
		"category":    f.Category,
		"imageUrl":    f.ImageURL,
	} {
		got, ok := back[0][key]
		if !ok {
			t.Errorf("the emitted entry carries no %q", key)
			continue
		}
		if got != want {
			t.Errorf("%s is %v, want %q", key, got, want)
		}
	}
}
