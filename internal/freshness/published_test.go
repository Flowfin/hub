//go:build needs_network

// The published address, read the way a server reads it.
//
// This is the only check in the tree that says anything about the world. Every
// other test here judges a planted body, so a green there says the reader is
// right and says nothing about whether the file an operator would fetch is
// current. It reaches the network twice, for the published file and for the
// release lists it is compared against, which is why it is the harness's and
// never the gate's.
package freshness_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"flowfin.dev/hub/internal/freshness"
	"flowfin.dev/hub/internal/releases"
	"flowfin.dev/hub/internal/sources"
)

// AddressVariable names the environment variable holding the published address.
//
// The address is not written into this tree, and that is
// decisions/manifest-address.md rather than an inconvenience: an install address
// may appear in a tracked file only once it answers with the file it promises,
// and today it answers 404. An unset variable is a refusal here rather than a
// skip, so this check cannot pass by not knowing where to look.
const AddressVariable = "MANIFEST_ADDRESS"

func TestThePublishedManifestIsCurrent(t *testing.T) {
	address := os.Getenv(AddressVariable)
	if address == "" {
		t.Fatalf("%s is unset, so there is no address to read; this is a refusal and not a skip, because a check that does not know where to look must not report the catalogue current",
			AddressVariable)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	expected := expectations(ctx, t)
	body := fetch(ctx, t, address)

	if err := freshness.Judge(body, expected); err != nil {
		t.Fatalf("%s: %v", address, err)
	}
	t.Logf("%s: %d declared plugin(s) current", address, len(expected))
}

// expectations reads the declared set and the newest finished release of each
// enabled declaration, from the release API rather than from anything this
// repository wrote.
func expectations(ctx context.Context, t *testing.T) []freshness.Expected {
	t.Helper()

	declarations, err := sources.Load(os.DirFS("../../" + sources.Dir))
	if err != nil {
		t.Fatalf("reading the declared set: %v", err)
	}

	client := releases.New()
	client.Token = os.Getenv("GITHUB_TOKEN")

	resolutions := sources.Resolve(ctx, client, declarations)
	if err := sources.Judge(resolutions); err != nil {
		t.Fatalf("the declared set does not resolve, so what the catalogue should hold is unknown: %v", err)
	}

	var out []freshness.Expected
	for _, r := range resolutions {
		if !r.Declaration.On() || len(r.Releases) == 0 {
			continue
		}
		out = append(out, freshness.Expected{
			Slug: r.Declaration.Slug,
			Path: r.Declaration.Path(),
			Tag:  r.Releases[0].Tag,
		})
	}
	return out
}

func fetch(ctx context.Context, t *testing.T, address string) []byte {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		t.Fatalf("%s: %v", address, err)
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s: the address could not be read, which a server shows as an empty repository: %v", address, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s answered %s; nothing installable is published there", address, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("%s: reading the body: %v", address, err)
	}
	return body
}
