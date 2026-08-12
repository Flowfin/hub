// Package releases reads release lists from the API.
//
// It is the only part of the generator that leaves the runner, which is why it
// is its own package: decisions/headless-and-unelevated.md refuses a gate test
// that reaches the network, and keeping the one thing that does behind an
// interface is what lets every classification in internal/sources be judged
// against a fixture instead.
package releases

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"flowfin.dev/hub/internal/pairing"
	"flowfin.dev/hub/internal/sources"
)

// DefaultAPI is where the release lists are read from. It is a field on Client
// rather than a constant used directly, so a test can point the same code at a
// server on the loopback interface.
const DefaultAPI = "https://api.github.com"

// PageSize is what each request asks for.
//
// The default page is thirty and it is not enough. Measured 2026-08-08:
//
//	gh api 'repos/Flowfin/jellyfin-plugin-sso/releases' --jq 'length'
//	30
//	gh api 'repos/Flowfin/jellyfin-plugin-sso/releases?per_page=100' --jq 'length'
//	54
//
// So a read that does not paginate drops 24 of the 54 releases of the one
// repository in the declared set that has anything to drop, and it drops them on
// the first run rather than eventually. A larger page is not the fix on its own,
// which is why Pages below follows the link header as well.
const PageSize = 100

// Client reads release lists.
type Client struct {
	HTTP    *http.Client
	API     string
	Token   string
	MaxPage int
}

// New returns a client with a timeout, because a request with no deadline is a
// run that hangs instead of failing.
func New() *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 30 * time.Second},
		API:     DefaultAPI,
		MaxPage: 50,
	}
}

type apiRelease struct {
	TagName     string     `json:"tag_name"`
	Prerelease  bool       `json:"prerelease"`
	PublishedAt time.Time  `json:"published_at"`
	Assets      []apiAsset `json:"assets"`
}

// apiAsset is one attached file, and the address it takes is the one that
// answers with the file.
//
// An asset carries two addresses. `url` is the API's own asset endpoint, which
// answers with a description of the asset unless the request asks for octets,
// so a fetch pointed at it succeeds and returns JSON where the archive's bytes
// were expected. `browser_download_url` answers with the file. The two differ in
// host as well as in path, so the mistake is not visible in a diff of one word.
//
// Size is read because it is the only thing that keeps a sidecar search off a
// release's whole payload: internal/pairing reads every asset under
// MaxSidecarBytes looking for a checksum line, and an asset whose size is
// unknown is an asset under any bound.
type apiAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// release converts one listed release into the model the layers above read.
func (r apiRelease) release() sources.Release {
	out := sources.Release{
		Tag:        r.TagName,
		Prerelease: r.Prerelease,
		Published:  r.PublishedAt,
	}
	for _, a := range r.Assets {
		out.Assets = append(out.Assets, pairing.Asset{Name: a.Name, URL: a.URL, Size: a.Size})
	}
	return out
}

// ListReleases reads every page of a repository's releases.
//
// It returns the path the response says it is for, so that a declaration
// answered through a rename can be told from one answered as written. The
// releases endpoint does not carry the repository's full name, so the name is
// read from the repository endpoint and both are requested.
func (c *Client) ListReleases(ctx context.Context, account, repository string) ([]sources.Release, string, error) {
	landedOn, err := c.fullName(ctx, account, repository)
	if err != nil {
		return nil, "", err
	}

	var out []sources.Release
	next := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=%d",
		strings.TrimRight(c.API, "/"), url.PathEscape(account), url.PathEscape(repository), PageSize)

	for page := 0; next != ""; page++ {
		if page >= c.MaxPage {
			// A cursor that never ends is a run that never ends. Refusing is the
			// only answer that does not silently publish a truncated list.
			return nil, "", fmt.Errorf("more than %d pages of releases for %s/%s", c.MaxPage, account, repository)
		}

		var body []apiRelease
		link, err := c.get(ctx, next, &body)
		if err != nil {
			return nil, "", err
		}
		for _, r := range body {
			out = append(out, r.release())
		}
		next = nextLink(link)
	}

	return out, landedOn, nil
}

func (c *Client) fullName(ctx context.Context, account, repository string) (string, error) {
	var body struct {
		FullName string `json:"full_name"`
	}
	address := fmt.Sprintf("%s/repos/%s/%s",
		strings.TrimRight(c.API, "/"), url.PathEscape(account), url.PathEscape(repository))
	if _, err := c.get(ctx, address, &body); err != nil {
		return "", err
	}
	return body.FullName, nil
}

func (c *Client) get(ctx context.Context, address string, into any) (link string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", sources.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		// Every other status is a read that did not happen, including the rate
		// limit. decisions/failure-posture.md puts this in the fatal column
		// because its symptom is a short list, which looks exactly like success.
		return "", fmt.Errorf("%s answered %s", address, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		return "", fmt.Errorf("reading %s: %w", address, err)
	}
	return resp.Header.Get("Link"), nil
}

// nextLink reads the address of the next page out of a Link header, and returns
// empty when there is not one.
//
// The header is followed rather than the page number incremented until a short
// page arrives, because a full last page is indistinguishable from a page with
// more behind it, and the difference is silently dropped releases.
func nextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		segments := strings.Split(strings.TrimSpace(part), ";")
		if len(segments) < 2 {
			continue
		}
		address := strings.TrimSpace(segments[0])
		if !strings.HasPrefix(address, "<") || !strings.HasSuffix(address, ">") {
			continue
		}
		for _, s := range segments[1:] {
			if rel := strings.TrimSpace(s); rel == `rel="next"` || rel == "rel=next" {
				return address[1 : len(address)-1]
			}
		}
	}
	return ""
}
