package releases

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
