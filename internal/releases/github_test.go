package releases

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"flowfin.dev/hub/internal/pairing"
	"flowfin.dev/hub/internal/sources"
)

// Every test here runs against a server on the loopback interface. Nothing in
// this package's suite leaves the runner, which is what
// decisions/headless-and-unelevated.md requires of anything in the gate, and it
// is the reason the one part of the generator that does leave sits behind an
// interface everything else is tested through.

func clientFor(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Client{HTTP: server.Client(), API: server.URL, MaxPage: 10}
}

// pages serves a release list in pages of size per, linking each to the next the
// way the API does.
func pages(t *testing.T, fullName string, tags []string, per int) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/releases") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"full_name": %q}`, fullName)
			return
		}

		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			fmt.Sscanf(p, "%d", &page)
		}
		start := (page - 1) * per
		end := min(start+per, len(tags))
		if start > len(tags) {
			start = len(tags)
		}

		if end < len(tags) {
			w.Header().Set("Link", fmt.Sprintf(`<%s://%s%s?per_page=%d&page=%d>; rel="next", <x>; rel="last"`,
				"http", r.Host, r.URL.Path, per, page+1))
		}
		w.Header().Set("Content-Type", "application/json")
		var b strings.Builder
		b.WriteString("[")
		for i, tag := range tags[start:end] {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `{"tag_name": %q, "prerelease": false}`, tag)
		}
		b.WriteString("]")
		fmt.Fprint(w, b.String())
	})
	return mux
}

// oneRelease serves a single release verbatim, so a test can state the response
// shape it is reading rather than build it out of a helper's idea of one.
func oneRelease(t *testing.T, fullName, release string) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.HasSuffix(r.URL.Path, "/releases") {
			fmt.Fprintf(w, `{"full_name": %q}`, fullName)
			return
		}
		fmt.Fprintf(w, "[%s]", release)
	})
	return mux
}

func TestAReleaseCarriesItsAssetsAndItsPublicationTime(t *testing.T) {
	// Neither is decoration. The archive and the checksum that names it are
	// selected out of the asset list, and the publication time is the order
	// decisions/failure-posture.md is written in terms of, which decides whether
	// a defect stops the run or is skipped by name.
	c := clientFor(t, oneRelease(t, "an-account/plugin", `{
		"tag_name": "1.2.3.4-stable",
		"prerelease": false,
		"published_at": "2026-08-12T05:47:37Z",
		"assets": [
			{"name": "a-plugin_1.2.3.4.zip", "size": 594668,
			 "browser_download_url": "https://example.com/download/a-plugin_1.2.3.4.zip"},
			{"name": "a-plugin_1.2.3.4.zip.md5sum", "size": 74,
			 "browser_download_url": "https://example.com/download/a-plugin_1.2.3.4.zip.md5sum"}
		]
	}`))

	got, _, err := c.ListReleases(context.Background(), "an-account", "plugin")
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("read %d releases", len(got))
	}

	want := []pairing.Asset{
		{Name: "a-plugin_1.2.3.4.zip", URL: "https://example.com/download/a-plugin_1.2.3.4.zip", Size: 594668},
		{Name: "a-plugin_1.2.3.4.zip.md5sum", URL: "https://example.com/download/a-plugin_1.2.3.4.zip.md5sum", Size: 74},
	}
	if !slices.Equal(got[0].Assets, want) {
		t.Errorf("the release's assets came back as %+v, want %+v", got[0].Assets, want)
	}
	if !got[0].Published.Equal(time.Date(2026, 8, 12, 5, 47, 37, 0, time.UTC)) {
		t.Errorf("the publication time came back as %v", got[0].Published)
	}
}

func TestTheAssetAddressIsTheOneThatAnswersWithTheFile(t *testing.T) {
	// An asset carries two addresses and only one of them answers with the file.
	// A fetch pointed at the other one succeeds, which is why this is a test
	// rather than something a run would notice.
	c := clientFor(t, oneRelease(t, "an-account/plugin", `{
		"tag_name": "1.0.0.0-stable",
		"assets": [
			{"name": "a-plugin_1.0.0.0.zip", "size": 12,
			 "url": "https://example.net/repos/an-account/plugin/releases/assets/1",
			 "browser_download_url": "https://example.com/download/a-plugin_1.0.0.0.zip"}
		]
	}`))

	got, _, err := c.ListReleases(context.Background(), "an-account", "plugin")
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(got) != 1 || len(got[0].Assets) != 1 {
		t.Fatalf("read %d releases", len(got))
	}
	if address := got[0].Assets[0].URL; address != "https://example.com/download/a-plugin_1.0.0.0.zip" {
		t.Errorf("the asset's address is %q, which is the endpoint that describes it rather than the one that serves it", address)
	}
}

func TestAReleaseWithNoPublicationTimeIsReadRatherThanRefused(t *testing.T) {
	// A run that fails its whole read because one release in a history carries a
	// null in one field learns nothing about the other fifty. The zero time is
	// the honest answer, and what a classification does with a release it cannot
	// place in time is that layer's question.
	c := clientFor(t, oneRelease(t, "an-account/plugin", `{
		"tag_name": "1.0.0.0-stable",
		"published_at": null,
		"assets": []
	}`))

	got, _, err := c.ListReleases(context.Background(), "an-account", "plugin")
	if err != nil {
		t.Fatalf("a null publication time refused the read: %v", err)
	}
	if len(got) != 1 || got[0].Tag != "1.0.0.0-stable" {
		t.Fatalf("read %d releases", len(got))
	}
	if !got[0].Published.IsZero() {
		t.Errorf("a release with no publication time came back as %v", got[0].Published)
	}
}

func TestEveryPageIsRead(t *testing.T) {
	// The number this is about: one declared repository has 54 releases against
	// a default page of 30, so a read that stops at the first page drops 24 of
	// them on the first run rather than eventually.
	var tags []string
	for i := range 54 {
		tags = append(tags, fmt.Sprintf("v1.0.%d", i))
	}
	c := clientFor(t, pages(t, "an-account/plugin", tags, 30))

	got, _, err := c.ListReleases(context.Background(), "an-account", "plugin")
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(got) != len(tags) {
		t.Fatalf("read %d releases of %d; a one-page read would have returned 30", len(got), len(tags))
	}
	if got[0].Tag != tags[0] || got[len(got)-1].Tag != tags[len(tags)-1] {
		t.Fatalf("the pages were not joined in order: first %q last %q", got[0].Tag, got[len(got)-1].Tag)
	}
}

func TestALastPageThatIsFullIsNotMistakenForTheEnd(t *testing.T) {
	// The reason the link header is followed rather than the page number
	// incremented until a short page arrives: a full last page looks exactly
	// like a page with more behind it.
	var tags []string
	for i := range 60 {
		tags = append(tags, fmt.Sprintf("v1.0.%d", i))
	}
	c := clientFor(t, pages(t, "an-account/plugin", tags, 30))

	got, _, err := c.ListReleases(context.Background(), "an-account", "plugin")
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(got) != 60 {
		t.Fatalf("read %d releases of 60", len(got))
	}
}

func TestANotFoundIsTheSentinelRatherThanAnEmptyList(t *testing.T) {
	// The whole point of the classification above it: a repository that answers
	// not-found and one with no releases mean opposite things and would be the
	// same value if this returned an empty list.
	c := clientFor(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))

	_, _, err := c.ListReleases(context.Background(), "an-account", "plugin")
	if !errors.Is(err, sources.ErrNotFound) {
		t.Fatalf("a not-found came back as %v", err)
	}
}

func TestAnyOtherStatusIsAnErrorRatherThanAnEmptyList(t *testing.T) {
	// A rate limit answers 403 with a body. Reading that as a repository with no
	// releases is how a run reports success on a catalogue it never read.
	c := clientFor(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limit exceeded", http.StatusForbidden)
	}))

	_, _, err := c.ListReleases(context.Background(), "an-account", "plugin")
	if err == nil {
		t.Fatal("a rate-limited read came back clean")
	}
	if errors.Is(err, sources.ErrNotFound) {
		t.Fatal("a rate limit was read as a repository that does not exist")
	}
}

func TestThePathThatAnsweredIsReported(t *testing.T) {
	// So that a declaration answered through a rename can be told from one
	// answered as written. This layer is the last place the difference exists.
	c := clientFor(t, pages(t, "another-account/plugin", []string{"v1.0"}, 30))

	_, landedOn, err := c.ListReleases(context.Background(), "an-account", "plugin")
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if landedOn != "another-account/plugin" {
		t.Fatalf("landed on %q", landedOn)
	}
}

func TestACursorThatNeverEndsIsRefused(t *testing.T) {
	// A run that never ends is worse than one that fails, and a truncated list
	// published quietly is worse than both.
	c := clientFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.HasSuffix(r.URL.Path, "/releases") {
			fmt.Fprint(w, `{"full_name": "an-account/plugin"}`)
			return
		}
		w.Header().Set("Link", fmt.Sprintf(`<http://%s%s>; rel="next"`, r.Host, r.URL.Path))
		fmt.Fprint(w, `[{"tag_name": "v1.0", "prerelease": false}]`)
	}))

	_, _, err := c.ListReleases(context.Background(), "an-account", "plugin")
	if err == nil {
		t.Fatal("an endless cursor was followed to a clean result")
	}
	if !strings.Contains(err.Error(), "pages") {
		t.Fatalf("the refusal does not say what happened: %v", err)
	}
}

func TestNextLinkReadsOnlyTheNextRelation(t *testing.T) {
	const header = `<http://pages.example/1>; rel="prev", <http://pages.example/3>; rel="next", <http://pages.example/9>; rel="last"`
	if got := nextLink(header); got != "http://pages.example/3" {
		t.Fatalf("nextLink = %q", got)
	}
	if got := nextLink(`<http://pages.example/9>; rel="last"`); got != "" {
		t.Fatalf("a header with no next returned %q", got)
	}
	if got := nextLink(""); got != "" {
		t.Fatalf("an empty header returned %q", got)
	}
}
