package pairing

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// The shape of a real release, from decisions/artifact-checksum-pairing.md,
// which measured it. The asset names are that release's; the digests here are
// invented, because a fixture carrying real ones would be a claim about a build
// rather than a fixture.
const (
	archiveName = "community-sso-for-jellyfin_4.3.0.28.zip"
	archiveMD5  = "be61dfe1e2b9101cd5c27169d4be8361"
	sbomSHA256  = "9375f49fd9f638a8dbf299ff2149216c07159b953e011aedbb020347f938413d"
	archiveSHA  = "1f0f5d4a4a2c4c6d8e0a2b4c6d8e0a2b4c6d8e0a2b4c6d8e0a2b4c6d8e0a2b4c"
)

// realShapedRelease returns the asset list in the order a name sort would NOT
// produce, so a selection that reaches for the last or the first thing a sort
// returns is visibly wrong rather than accidentally right.
//
// A sort by name puts sbom.cyclonedx.sha256 last. So does picking "the last
// asset whose name ends in a checksum suffix". Both would publish the SBOM's
// digest against the plugin archive, which is the failure the decision opens
// with, and every install on every server would then fail verification with
// nothing saying which of the two was wrong.
func realShapedRelease() ([]Asset, Fetch) {
	bodies := map[string]string{
		"build.yaml": "version: 4.3.0.28\nchecksum: not-a-checksum-line\n",
		"community-sso-for-jellyfin_4.3.0.28.md5":           archiveMD5 + "  " + archiveName + "\n",
		"community-sso-for-jellyfin_4.3.0.28.sha256":        archiveSHA + "  " + archiveName + "\n",
		"community-sso-for-jellyfin_4.3.0.28.zip.md5sum":    archiveMD5 + " *" + archiveName + "\n",
		"community-sso-for-jellyfin_4.3.0.28.zip.meta.json": `{"name":"` + archiveName + `","checksum":"` + archiveMD5 + `"}` + "\n",
		"sbom.cyclonedx.json":                               `{"bomFormat":"CycloneDX"}` + "\n",
		"sbom.cyclonedx.sha256":                             sbomSHA256 + "  sbom.cyclonedx.json\n",
	}

	// Deliberately not in sorted order, and the archive is not first.
	order := []string{
		"sbom.cyclonedx.sha256",
		"community-sso-for-jellyfin_4.3.0.28.zip.md5sum",
		"build.yaml",
		archiveName,
		"community-sso-for-jellyfin_4.3.0.28.zip.meta.json",
		"community-sso-for-jellyfin_4.3.0.28.md5",
		"sbom.cyclonedx.json",
		"community-sso-for-jellyfin_4.3.0.28.sha256",
	}

	assets := make([]Asset, 0, len(order))
	for _, name := range order {
		size := int64(len(bodies[name]))
		if name == archiveName {
			size = 2_400_000
		}
		assets = append(assets, Asset{
			Name: name,
			URL:  "https://example.com/download/" + name,
			Size: size,
		})
	}

	fetch := func(a Asset) ([]byte, error) {
		body, ok := bodies[a.Name]
		if !ok {
			return nil, fmt.Errorf("no fixture body for %s", a.Name)
		}
		return []byte(body), nil
	}
	return assets, fetch
}

// TestAnOrderingThatWouldFoolANameSortStillPairsCorrectly is the first half of
// the issue's done-condition.
func TestAnOrderingThatWouldFoolANameSortStillPairsCorrectly(t *testing.T) {
	assets, fetch := realShapedRelease()

	pair, err := Resolve("4.3.0-beta.28", assets, fetch)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if pair.Archive.Name != archiveName {
		t.Errorf("selected %s, want %s", pair.Archive.Name, archiveName)
	}
	if pair.Checksum != archiveMD5 {
		t.Errorf("paired checksum %s, want %s", pair.Checksum, archiveMD5)
	}
	if pair.Checksum == sbomSHA256 {
		t.Error("the SBOM's digest was published against the plugin archive, which is the failure this package exists against")
	}
	// Both MD5 spellings name the archive and both were read. The decision says
	// they agree in the release it measured and that the generator does not
	// assume it.
	if len(pair.Sidecars) != 2 {
		t.Errorf("read %v, want both MD5 spellings", pair.Sidecars)
	}
}

// TestTheSuffixIsASuffixAndNotASubstring. Two assets in that release carry
// ".zip" in the middle of their names and neither is the archive.
func TestTheSuffixIsASuffixAndNotASubstring(t *testing.T) {
	assets, _ := realShapedRelease()

	archive, err := SelectArchive("4.3.0-beta.28", assets)
	if err != nil {
		t.Fatalf("SelectArchive: %v", err)
	}
	if archive.Name != archiveName {
		t.Fatalf("selected %s, want %s", archive.Name, archiveName)
	}

	containing := 0
	for _, a := range assets {
		if strings.Contains(a.Name, ArchiveSuffix) {
			containing++
		}
	}
	if containing < 3 {
		t.Fatalf("%d assets contain %q; this fixture is meant to hold the archive and two others, so it is not testing the difference",
			containing, ArchiveSuffix)
	}
}

// TestAnArchiveWithNoSidecarIsRefusedAndTheRefusalNamesTheRelease is the second
// half of the done-condition. A neighbouring checksum is what a positional rule
// would reach for, and there are two of them in this fixture.
func TestAnArchiveWithNoSidecarIsRefusedAndTheRefusalNamesTheRelease(t *testing.T) {
	assets, fetch := realShapedRelease()

	// Every sidecar that names the archive, removed. What is left is the SBOM's
	// checksum, which is exactly the value the failure this package exists
	// against would publish.
	var stripped []Asset
	for _, a := range assets {
		if strings.HasPrefix(a.Name, "community-sso-for-jellyfin_4.3.0.28.md5") ||
			strings.HasSuffix(a.Name, ".md5sum") ||
			strings.HasSuffix(a.Name, ".sha256") && !strings.HasPrefix(a.Name, "sbom") {
			continue
		}
		stripped = append(stripped, a)
	}

	_, err := Resolve("4.3.0-beta.28", stripped, fetch)
	if err == nil {
		t.Fatal("a release whose archive has no sidecar was paired")
	}
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("the failure is not a refusal: %v", err)
	}
	if refusal.Release != "4.3.0-beta.28" {
		t.Errorf("the refusal names release %q", refusal.Release)
	}
	if refusal.Reason != NoUsableSidecar {
		t.Errorf("the refusal is %s, want %s", refusal.Reason, NoUsableSidecar)
	}
	if !strings.Contains(err.Error(), "4.3.0-beta.28") || !strings.Contains(err.Error(), archiveName) {
		t.Errorf("the refusal does not name the release and the archive: %v", err)
	}
	if strings.Contains(err.Error(), sbomSHA256) {
		t.Errorf("the refusal carries the SBOM's digest, which is the value that must never reach an entry: %v", err)
	}
}

func TestASidecarNamingTheArchiveInAnotherDigestIsNotPublished(t *testing.T) {
	// The SHA-256 sidecar does name the archive. Publishing its value produces a
	// manifest that looks stronger and fails every install, because the server
	// compares against what it computed with MD5.
	assets, fetch := realShapedRelease()

	var onlyStrong []Asset
	for _, a := range assets {
		if strings.HasSuffix(a.Name, ".md5") || strings.HasSuffix(a.Name, ".md5sum") {
			continue
		}
		onlyStrong = append(onlyStrong, a)
	}

	_, err := Resolve("4.3.0-beta.28", onlyStrong, fetch)
	if err == nil {
		t.Fatal("a release whose only sidecar carries a stronger digest was paired")
	}
	if !strings.Contains(err.Error(), ".sha256") || !strings.Contains(err.Error(), "64 hex characters") {
		t.Errorf("the refusal does not say what it found and why it could not use it: %v", err)
	}
	if strings.Contains(err.Error(), archiveSHA) {
		t.Errorf("the refusal quotes the digest that must not reach the field: %v", err)
	}
}

func TestTwoSidecarsThatDisagreeAreRefusedAndBothValuesAreNamed(t *testing.T) {
	assets, fetch := realShapedRelease()
	const other = "ffffffffffffffffffffffffffffffff"

	wrapped := func(a Asset) ([]byte, error) {
		if a.Name == "community-sso-for-jellyfin_4.3.0.28.zip.md5sum" {
			return []byte(other + " *" + archiveName + "\n"), nil
		}
		return fetch(a)
	}

	_, err := Resolve("4.3.0-beta.28", assets, wrapped)
	if err == nil {
		t.Fatal("two sidecars disagreeing about one archive were paired anyway")
	}
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Reason != DisagreeingSidecars {
		t.Fatalf("the refusal is %v, want %s", err, DisagreeingSidecars)
	}
	for _, want := range []string{archiveMD5, other, "4.3.0-beta.28"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not carry %q: %v", want, err)
		}
	}
}

func TestNoArchiveAndTwoArchivesAreDifferentRefusals(t *testing.T) {
	none := []Asset{{Name: "sbom.cyclonedx.json"}, {Name: "build.yaml"}}
	_, err := SelectArchive("v1", none)
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Reason != NoArchive {
		t.Fatalf("a release with no archive gave %v, want %s", err, NoArchive)
	}
	if !strings.Contains(err.Error(), "v1") {
		t.Errorf("the refusal does not name the release: %v", err)
	}

	two := []Asset{{Name: "a_1.0.0.zip"}, {Name: "b_1.0.0.zip"}, {Name: "build.yaml"}}
	_, err = SelectArchive("v2", two)
	if !errors.As(err, &refusal) || refusal.Reason != AmbiguousArchive {
		t.Fatalf("a release with two archives gave %v, want %s", err, AmbiguousArchive)
	}
	for _, want := range []string{"a_1.0.0.zip", "b_1.0.0.zip"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %s: %v", want, err)
		}
	}
}

func TestAFetchThatFailedIsNotAReleaseThatCannotBePaired(t *testing.T) {
	// A read that did not happen looks exactly like a release with no sidecar,
	// and the two must not collapse: one is skipped and the other stops the run.
	assets, _ := realShapedRelease()
	broken := func(a Asset) ([]byte, error) { return nil, errors.New("503 Service Unavailable") }

	_, err := Resolve("4.3.0-beta.28", assets, broken)
	if err == nil {
		t.Fatal("a release whose sidecars could not be read was paired")
	}
	var refusal *Refusal
	if errors.As(err, &refusal) {
		t.Fatalf("a failed read was reported as a release that cannot be paired: %v", err)
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("the error does not carry what the read said: %v", err)
	}
}

func TestParseChecksumLineReadsTheTwoRealSpellingsAndRefusesTheRest(t *testing.T) {
	for _, c := range []struct{ name, body, digest, subject string }{
		{"two spaces", archiveMD5 + "  " + archiveName + "\n", archiveMD5, archiveName},
		{"binary mode", archiveMD5 + " *" + archiveName + "\n", archiveMD5, archiveName},
		{"a tab", archiveMD5 + "\t" + archiveName + "\n", archiveMD5, archiveName},
		{"no trailing newline", archiveMD5 + "  " + archiveName, archiveMD5, archiveName},
		{"upper case digest", strings.ToUpper(archiveMD5) + "  " + archiveName + "\n", archiveMD5, archiveName},
		{"a stronger digest", sbomSHA256 + "  sbom.cyclonedx.json\n", sbomSHA256, "sbom.cyclonedx.json"},
	} {
		line, ok := ParseChecksumLine([]byte(c.body))
		if !ok {
			t.Errorf("%s: refused a checksum line", c.name)
			continue
		}
		if line.Digest != c.digest || line.Subject != c.subject {
			t.Errorf("%s: read %q about %q, want %q about %q", c.name, line.Digest, line.Subject, c.digest, c.subject)
		}
	}

	for _, c := range []struct{ name, body string }{
		{"empty", ""},
		{"a JSON document", `{"bomFormat":"CycloneDX","specVersion":"1.5"}` + "\n"},
		{"a YAML document", "version: 4.3.0.28\nchecksum: none\n"},
		{"a digest with no subject", archiveMD5 + "\n"},
		{"a subject with no digest", "  " + archiveName + "\n"},
		{"a digest that is not hexadecimal", "zzzzdfe1e2b9101cd5c27169d4be8361  " + archiveName + "\n"},
		{"two lines", archiveMD5 + "  a.zip\n" + archiveMD5 + "  b.zip\n"},
	} {
		if line, ok := ParseChecksumLine([]byte(c.body)); ok {
			t.Errorf("%s: read as %q about %q; a body that is not a checksum line is how the SBOM is refused on its contents",
				c.name, line.Digest, line.Subject)
		}
	}
}

func TestAnAssetTooLargeToBeASidecarIsNotRead(t *testing.T) {
	// The bound is stated in the package and this is what it does. An SBOM that
	// happens to begin with something parseable is not downloaded in full to
	// find out.
	read := map[string]bool{}
	assets := []Asset{
		{Name: archiveName, Size: 2_400_000},
		{Name: "huge.md5", Size: MaxSidecarBytes + 1},
		{Name: "small.md5", Size: 64},
	}
	fetch := func(a Asset) ([]byte, error) {
		read[a.Name] = true
		return []byte(archiveMD5 + "  " + archiveName + "\n"), nil
	}

	pair, err := Resolve("v1", assets, fetch)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if read["huge.md5"] {
		t.Errorf("an asset of %d bytes was read, and the bound is %d", MaxSidecarBytes+1, MaxSidecarBytes)
	}
	if !read["small.md5"] {
		t.Error("the sidecar inside the bound was not read")
	}
	if pair.Checksum != archiveMD5 {
		t.Errorf("paired %s, want %s", pair.Checksum, archiveMD5)
	}
}
