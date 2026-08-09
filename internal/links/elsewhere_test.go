//go:build needs_network

// The half of the site's links that only a request can decide.
//
// It is here rather than in the gate because a link to somebody else is a claim
// about somebody else's server, and decisions/headless-and-unelevated.md keeps
// that out of a merge: a gate that reds because a host was having an afternoon
// is a gate people learn to ignore, and once they ignore it the internal half
// stops being read either.
//
// So this runs when a person asks for it, under needs-network, and nothing here
// blocks anything. The sorting it depends on is decided in the gate's own suite
// against fixtures, which leaves exactly the requests unproven until somebody
// asks.
package links_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"flowfin.dev/hub/internal/links"
)

func TestEveryAddressTheSitePointsAtAnswers(t *testing.T) {
	found, err := links.References(os.DirFS(filepath.Join("..", "..", links.Dir)))
	if err != nil {
		t.Fatalf("reading the site: %v", err)
	}

	// One request per address rather than one per appearance, and the pages an
	// address appears on are kept so a failure names them.
	where := map[string][]string{}
	for _, r := range found {
		address, outside := links.Elsewhere(r.Address)
		if !outside {
			continue
		}
		where[address] = append(where[address], r.File)
	}

	addresses := make([]string, 0, len(where))
	for address := range where {
		addresses = append(addresses, address)
	}
	sort.Strings(addresses)

	// What was examined, said out loud. The site carries no address off it
	// today, and a run over nothing prints the same green as a run over
	// everything unless it says which it was.
	t.Logf("%d address(es) off this site, across %d reference(s) under %s/", len(addresses), len(found), links.Dir)
	if len(addresses) == 0 {
		t.Logf("nothing was requested, so this run is evidence about the site's own files and about nothing else")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 30 * time.Second}

	for _, address := range addresses {
		pages := where[address]
		status, err := reachable(ctx, client, address)
		switch {
		case err != nil:
			t.Errorf("%s, linked from %s: %v", address, strings.Join(pages, ", "), err)
		case status >= 400:
			t.Errorf("%s, linked from %s: answered %d", address, strings.Join(pages, ", "), status)
		default:
			t.Logf("%s: %d", address, status)
		}
	}
}

// reachable asks for the address the cheap way first.
//
// A HEAD costs the far end nothing and is enough for most servers. Some answer
// it with 405 while serving the page perfectly well, so a status that says the
// method was the problem is retried as a GET rather than reported as a dead
// link.
func reachable(ctx context.Context, client *http.Client, address string) (int, error) {
	status, err := request(ctx, client, http.MethodHead, address)
	if err != nil {
		return 0, err
	}
	if status == http.StatusMethodNotAllowed || status == http.StatusNotImplemented {
		return request(ctx, client, http.MethodGet, address)
	}
	return status, nil
}

func request(ctx context.Context, client *http.Client, method, address string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, address, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
