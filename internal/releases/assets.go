package releases

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"flowfin.dev/hub/internal/pairing"
)

// MaxAssetBytes bounds one asset read.
//
// The files this reads are a checksum line and a descriptor, and neither is
// near it: internal/pairing already refuses to read anything larger than
// MaxSidecarBytes as a sidecar, and the descriptors in the declared set are a
// few hundred bytes. The bound is here for the file that is not one of those,
// because a release is a place somebody else can attach a gigabyte, and a run
// that reads it holds it in memory and stops being a run.
//
// Reaching it is a read that did not happen rather than a release that cannot
// be published. A defect in the release is a statement about the release, and
// this is a statement about the size of the response, which says nothing about
// whether the file is any good.
const MaxAssetBytes = 1 << 20

// Fetching returns the reader the layers that judge release files take.
//
// The context is bound here rather than passed to each call because
// pairing.Fetch carries no context and giving it one would reach into three
// packages to say what a deadline already says. A run whose context is done
// stops at the next asset instead of at the next plugin.
func (c *Client) Fetching(ctx context.Context) pairing.Fetch {
	return func(a pairing.Asset) ([]byte, error) {
		return c.fetch(ctx, a)
	}
}

func (c *Client) fetch(ctx context.Context, a pairing.Asset) ([]byte, error) {
	if a.URL == "" {
		return nil, fmt.Errorf("the asset %s carries no address to read it from", a.Name)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return nil, err
	}
	// The address is the browser download address, which answers with the file
	// rather than with a description of it, so this is the one Accept the API
	// requests do not use.
	req.Header.Set("Accept", "application/octet-stream")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Every status other than a plain success is a read that did not happen,
	// including the not-found. An asset listed by the release and answering with
	// a not-found is a release that changed under the run rather than one that
	// shipped nothing, and decisions/failure-posture.md puts a read it cannot
	// trust in the fatal column whatever the reason.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s for %s", a.URL, resp.Status, a.Name)
	}

	// One byte past the bound is read so that a file exactly at it is not
	// reported as too large, and so that a file past it is refused rather than
	// silently truncated to the bound. A truncated checksum sidecar parses as a
	// line that names nothing, which is a release skipped for a reason that did
	// not happen.
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxAssetBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", a.Name, err)
	}
	if len(body) > MaxAssetBytes {
		return nil, fmt.Errorf("%s is larger than the %d bytes one asset is read up to", a.Name, MaxAssetBytes)
	}
	return body, nil
}
