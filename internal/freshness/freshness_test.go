package freshness

import (
	"fmt"
	"strings"
	"testing"
)

// published builds a manifest body the way a server would receive it. The
// entries carry only what this check reads; a real one carries more and the
// decoder ignores the rest, which is the point of decoding three fields.
func published(entries ...string) []byte {
	return []byte(`[{"guid":"0d2f6f0a-1111-4a11-9a11-aaaaaaaaaaaa","name":"Example Widget","versions":[` +
		strings.Join(entries, ",") + `]}]`)
}

func versionEntry(path, tag, target string) string {
	return fmt.Sprintf(`{"version":"%s","targetAbi":"%s","sourceUrl":"https://example.com/%s/releases/download/%s/plugin.zip"}`,
		strings.TrimPrefix(tag, "v"), target, path, tag)
}

const declared = "Example/jellyfin-plugin-widget"

func TestTheNewestReleaseUnderEveryTargetIsAPass(t *testing.T) {
	body := published(
		versionEntry(declared, "2.0.0.0-stable", "10.11.0.0"),
		versionEntry(declared, "1.9.0.0-stable", "10.11.0.0"),
		versionEntry(declared, "2.0.0.0-stable", "10.10.0.0"),
		versionEntry(declared, "1.8.0.0-stable", "10.10.0.0"),
	)
	if err := Judge(body, []Expected{{Slug: "widget", Path: declared, Tag: "2.0.0.0-stable"}}); err != nil {
		t.Fatalf("a current manifest was refused: %v", err)
	}
}

// TestOneTargetMissingTheNewestReleaseIsRefused is the property the issue names.
// The newest release is present, so a check asking only "is it there" passes,
// and every server on the other target line is looking at an old build.
func TestOneTargetMissingTheNewestReleaseIsRefused(t *testing.T) {
	body := published(
		versionEntry(declared, "2.0.0.0-stable", "10.11.0.0"),
		versionEntry(declared, "1.9.0.0-stable", "10.11.0.0"),
		versionEntry(declared, "1.8.0.0-stable", "10.10.0.0"),
	)
	err := Judge(body, []Expected{{Slug: "widget", Path: declared, Tag: "2.0.0.0-stable"}})
	if err == nil {
		t.Fatal("a target line missing the newest release was read as current")
	}
	for _, want := range []string{"widget", "2.0.0.0-stable", "10.10.0.0"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not carry %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "10.11.0.0 (") {
		t.Errorf("the refusal blames the target line that is current: %v", err)
	}
}

func TestAPluginTheCatalogueDoesNotListAtAllIsRefused(t *testing.T) {
	body := published(versionEntry("Example/jellyfin-plugin-other", "2.0.0.0-stable", "10.11.0.0"))
	err := Judge(body, []Expected{{Slug: "widget", Path: declared, Tag: "2.0.0.0-stable"}})
	if err == nil {
		t.Fatal("a declared plugin missing from the catalogue was read as current")
	}
	if !strings.Contains(err.Error(), "lists nothing from "+declared) {
		t.Errorf("the refusal does not say the plugin is absent rather than old: %v", err)
	}
}

func TestABodyThatDoesNotParseIsRefused(t *testing.T) {
	for _, body := range []string{
		"",
		"<!DOCTYPE html><title>404</title>",
		`{"plugins": []}`,
		"[{",
	} {
		if err := Judge([]byte(body), []Expected{{Slug: "widget", Path: declared, Tag: "v1"}}); err == nil {
			t.Errorf("a body that is not a manifest was read as one: %q", body)
		}
	}
}

func TestAnEmptyCatalogueIsRefusedRatherThanPassed(t *testing.T) {
	// A server shows an empty repository for this, an unreachable address and an
	// unparseable body alike. Passing here would make the loudest of the three
	// silent.
	err := Judge([]byte("[]"), []Expected{{Slug: "widget", Path: declared, Tag: "v1"}})
	if err == nil {
		t.Fatal("an empty catalogue was read as current")
	}
	if !strings.Contains(err.Error(), "no plugins") {
		t.Errorf("the refusal does not say the catalogue is empty: %v", err)
	}
}

func TestExpectingNothingIsRefusedRatherThanPassed(t *testing.T) {
	// The way this check quietly stops checking: the declared set fails to load,
	// or every declaration is off, and a loop over an empty list finds no
	// failure. A pass then means nothing was compared.
	body := published(versionEntry(declared, "2.0.0.0-stable", "10.11.0.0"))
	err := Judge(body, nil)
	if err == nil {
		t.Fatal("a run that expected nothing was read as a pass")
	}
	if !strings.Contains(err.Error(), "judged none") {
		t.Errorf("the refusal does not say nothing was compared: %v", err)
	}
}

func TestAReleasePageLinkIsNotAnAsset(t *testing.T) {
	// A sourceUrl pointing at a release page instead of at a download carries no
	// archive. Matching on the download segment rather than on the repository
	// keeps such an entry from standing in for a published build.
	body := []byte(`[{"guid":"g","name":"n","versions":[` +
		`{"version":"2.0.0.0","targetAbi":"10.11.0.0","sourceUrl":"https://example.com/` + declared + `/releases/tag/2.0.0.0-stable"}` +
		`]}]`)
	err := Judge(body, []Expected{{Slug: "widget", Path: declared, Tag: "2.0.0.0-stable"}})
	if err == nil {
		t.Fatal("a link to a release page was counted as a published archive")
	}
	if !strings.Contains(err.Error(), "lists nothing from") {
		t.Errorf("the refusal does not say the plugin is absent: %v", err)
	}
}

func TestAnEntryUnderTwoPluginObjectsIsStillFound(t *testing.T) {
	// The declaration is tied to the catalogue through the download address
	// rather than through a guid, so this check does not depend on how plugin
	// identity is built and does not break when it changes.
	body := []byte(`[` +
		`{"guid":"a","name":"A","versions":[` + versionEntry("Example/other", "1.0.0.0-stable", "10.11.0.0") + `]},` +
		`{"guid":"b","name":"B","versions":[` + versionEntry(declared, "2.0.0.0-stable", "10.11.0.0") + `]}` +
		`]`)
	if err := Judge(body, []Expected{{Slug: "widget", Path: declared, Tag: "2.0.0.0-stable"}}); err != nil {
		t.Fatalf("an entry in the second plugin object was not found: %v", err)
	}
}

func TestEveryFailureIsNamedRatherThanTheFirst(t *testing.T) {
	body := published(versionEntry(declared, "1.0.0.0-stable", "10.11.0.0"))
	err := Judge(body, []Expected{
		{Slug: "widget", Path: declared, Tag: "2.0.0.0-stable"},
		{Slug: "other", Path: "Example/jellyfin-plugin-other", Tag: "1.0.0.0-stable"},
	})
	if err == nil {
		t.Fatal("two failures were read as none")
	}
	for _, want := range []string{"widget", "other"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %s, so a repair would need a second run to find it: %v", want, err)
		}
	}
}
