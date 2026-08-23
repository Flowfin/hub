//go:build needs_jellyfin

// The half of the pairing rule only a server can decide.
//
// Everything else about a manifest is judged here against this repository's own
// idea of what a Jellyfin server does. This one asks the server. The failure it
// catches is the one nobody sees coming: a manifest that is well formed, passes
// every leg of the gate, and renders as an empty repository or a refused install
// on a real server because of a field the server reads differently.
//
// It is also the only place decisions/artifact-checksum-pairing.md is proven
// against something other than a fixture. The rest of this package decides which
// asset and which checksum line belong together; the server is what actually
// downloads the archive, hashes it and compares. A pairing that came apart is
// invisible from this side and is a failed install on every server that polls
// the address.
//
// Two halves, in this order and not the other. The mismatched checksum runs
// first, against a catalogue this test serves itself, because
// decisions/manifest-address.md treats the published address as a promise that
// cannot be withdrawn and a knowingly broken manifest there is served to every
// operator polling it. The published address carries the half that is meant to
// succeed, and it runs second so that the refusal above cannot be read off a
// server that had already installed the plugin.
package pairing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"flowfin.dev/hub/internal/address"
)

// AddressVariable is where the server under test is named.
//
// The job that asks for this requirement brings the server up and hands the
// address in, and unset is a refusal here rather than a skip. A check that skips
// when its environment is missing reports green over nothing, which is the
// failure the whole harness exists against, and a job that started a server and
// forgot to pass its address would otherwise read as a broken test.
const AddressVariable = "JELLYFIN_ADDRESS"

// The bounds this check waits under. Each one is a separate sentence in a
// failure, because "the server never came up", "the catalogue never listed the
// plugin" and "the install never finished" send somebody to three different
// places.
const (
	readyWindow   = 3 * time.Minute
	catalogueWait = 2 * time.Minute
	installWindow = 3 * time.Minute
	pollEvery     = 2 * time.Second
)

// The account this check creates on the server it sets up. It is not a
// credential shared with anything: the server is brought up for the run and torn
// down after it, and a server that has already been set up is refused below
// rather than logged into.
const (
	adminUser     = "harness"
	adminPassword = "harness-password-for-one-run"
)

// mismatchedDigest is a well-formed checksum that belongs to no archive. It is
// the right length and it is hexadecimal, so the manifest parses and the server
// gets as far as downloading the archive and hashing it, which is the step this
// half is about. A malformed value would be refused earlier and would prove
// something else.
const mismatchedDigest = "00000000000000000000000000000000"

func TestAServerInstallsFromThePublishedCatalogueAndRefusesAMismatchedChecksum(t *testing.T) {
	base := strings.TrimSpace(os.Getenv(AddressVariable))
	if base == "" {
		t.Fatalf("%s is unset, so this run has no server to talk to. "+
			"The job that names this requirement brings one up and hands its address in; "+
			"a local run needs one provided the same way, and unset is refused here rather than skipped "+
			"because a check that skips over a missing environment reports green over nothing.",
			AddressVariable)
	}
	if len(address.Answered) == 0 {
		t.Fatal("no install address is recorded as answering, so there is no published catalogue for a server to add. " +
			"decisions/manifest-address.md is where an entry is added and what it costs.")
	}
	published := address.Answered[0]
	t.Logf("server under test: %s", base)
	t.Logf("published catalogue under test: %s", published)

	ctx, cancel := context.WithTimeout(context.Background(), readyWindow+2*catalogueWait+2*installWindow)
	defer cancel()

	s := &server{t: t, base: strings.TrimRight(base, "/"), client: &http.Client{Timeout: 60 * time.Second}}
	s.waitUntilReadyAndUnconfigured(ctx)
	s.completeSetup(ctx)
	s.authenticate(ctx)

	// The manifest this repository publishes, read from the tree rather than
	// fetched, so nothing in this test reaches off the machine. Every request
	// that leaves the runner is the server's, which is why this file carries one
	// requirement and not two.
	catalogue := readPublishedCatalogue(t)
	name, guid := onePlugin(t, catalogue)
	t.Logf("plugin under test: %s (%s)", name, guid)

	t.Run("a mismatched checksum refuses the install", func(t *testing.T) {
		s := s.with(t)

		damaged := withDigest(t, catalogue, mismatchedDigest)
		served := serveCatalogue(t, s.base, damaged)
		t.Logf("serving a catalogue whose checksum belongs to no archive at %s", served)

		s.setRepositories(ctx, repository{Name: "harness-damaged", URL: served, Enabled: true})
		s.waitForCatalogueEntry(ctx, name)
		s.install(ctx, name, served)

		if installed, waited := s.pluginAppears(ctx, guid, installWindow); installed {
			t.Fatalf("the server installed %s after %s from a manifest whose checksum belongs to no archive. "+
				"That is the pairing rule not being enforced where it is finally checked, and every published "+
				"mismatch would install silently.", name, waited)
		}
		t.Logf("after %s the server has not installed %s, which is the refusal this half is for", installWindow, name)
	})

	t.Run("the published catalogue installs", func(t *testing.T) {
		s := s.with(t)

		s.setRepositories(ctx, repository{Name: "harness-published", URL: published, Enabled: true})
		s.waitForCatalogueEntry(ctx, name)
		s.install(ctx, name, published)

		installed, waited := s.pluginAppears(ctx, guid, installWindow)
		if !installed {
			t.Fatalf("the server did not install %s from %s within %s. "+
				"An operator pasting that address sees the same thing: a repository that lists the plugin "+
				"and an install that never arrives, with no error in the interface.", name, published, installWindow)
		}
		t.Logf("the server installed %s from the published address after %s", name, waited)
	})
}

// server is the Jellyfin instance under test and the token this run holds on it.
type server struct {
	t      *testing.T
	base   string
	token  string
	client *http.Client
}

// with is this server read by a subtest, so a failure lands on the subtest that
// caused it. The token and the address are shared; only which test is being
// spoken to changes.
func (s *server) with(t *testing.T) *server {
	next := *s
	next.t = t
	return &next
}

// authorization is the header every request carries. Jellyfin reads the client
// fields whether or not a token is held, and refuses a request that carries none
// of them, so the header is built in one place rather than per call.
func (s *server) authorization() string {
	header := `MediaBrowser Client="hub-harness", Device="harness", DeviceId="hub-harness", Version="1"`
	if s.token != "" {
		header += fmt.Sprintf(`, Token=%q`, s.token)
	}
	return header
}

// call makes one request and returns the status and the body. It never fails the
// test itself: a caller that is polling wants the error, and a caller that is not
// says what the failure means in its own words.
func (s *server) call(ctx context.Context, method, path string, body any) (int, []byte, error) {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("encoding the body of %s %s: %w", method, path, err)
		}
		payload = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.base+path, payload)
	if err != nil {
		return 0, nil, fmt.Errorf("building %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization", s.authorization())
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	read, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("reading the body of %s %s: %w", method, path, err)
	}
	return resp.StatusCode, read, nil
}

// must makes one request and fails the test on anything but a 2xx, quoting what
// came back. A truncated body is enough to place the failure and short enough to
// read in a log.
func (s *server) must(ctx context.Context, method, path string, body any) []byte {
	s.t.Helper()
	status, read, err := s.call(ctx, method, path, body)
	if err != nil {
		s.t.Fatalf("%s %s: %v", method, path, err)
	}
	if status < 200 || status > 299 {
		s.t.Fatalf("%s %s answered %d: %s", method, path, status, excerpt(read))
	}
	return read
}

// waitUntilReadyAndUnconfigured waits for the server to answer and refuses one
// that has already been set up.
//
// The refusal is deliberate and it is not a limitation worth working around.
// This check takes the administrator account on the server it talks to, so a
// server somebody is using is the wrong thing to point it at, and the harness
// says in its own cost line that the server is brought up for the run and torn
// down after it.
func (s *server) waitUntilReadyAndUnconfigured(ctx context.Context) {
	s.t.Helper()
	started := time.Now()
	deadline := started.Add(readyWindow)
	var last string
	for time.Now().Before(deadline) {
		status, body, err := s.call(ctx, http.MethodGet, "/System/Info/Public", nil)
		switch {
		case err != nil:
			last = err.Error()
		case status != http.StatusOK:
			last = fmt.Sprintf("answered %d: %s", status, excerpt(body))
		default:
			var info struct {
				Version                string `json:"Version"`
				StartupWizardCompleted bool   `json:"StartupWizardCompleted"`
			}
			if err := json.Unmarshal(body, &info); err != nil {
				s.t.Fatalf("%s answered 200 with something that is not a Jellyfin server: %v", s.base, err)
			}
			if info.StartupWizardCompleted {
				s.t.Fatalf("%s is a server that has already been set up. This check creates the administrator account "+
					"on the server it talks to, so it needs one brought up for the run rather than one somebody is using.", s.base)
			}
			s.t.Logf("the server answered after %s, version %s, not yet set up", time.Since(started).Round(time.Second), info.Version)
			return
		}
		sleep(ctx, pollEvery)
	}
	s.t.Fatalf("%s did not answer within %s. The last thing it said was: %s", s.base, readyWindow, last)
}

// completeSetup walks the startup wizard, which is the only way to get an
// administrator account on a fresh server.
func (s *server) completeSetup(ctx context.Context) {
	s.t.Helper()
	s.must(ctx, http.MethodPost, "/Startup/Configuration", map[string]string{
		"UICulture":                 "en-US",
		"MetadataCountryCode":       "US",
		"PreferredMetadataLanguage": "en",
	})
	s.must(ctx, http.MethodGet, "/Startup/User", nil)
	s.must(ctx, http.MethodPost, "/Startup/User", map[string]string{
		"Name":     adminUser,
		"Password": adminPassword,
	})
	s.must(ctx, http.MethodPost, "/Startup/RemoteAccess", map[string]bool{
		"EnableRemoteAccess":         true,
		"EnableAutomaticPortMapping": false,
	})
	s.must(ctx, http.MethodPost, "/Startup/Complete", nil)
	s.t.Log("the startup wizard is complete and this run holds the administrator account")
}

// authenticate exchanges the account for the token every later request carries.
func (s *server) authenticate(ctx context.Context) {
	s.t.Helper()
	body := s.must(ctx, http.MethodPost, "/Users/AuthenticateByName", map[string]string{
		"Username": adminUser,
		"Pw":       adminPassword,
	})
	var result struct {
		AccessToken string `json:"AccessToken"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		s.t.Fatalf("reading the authentication answer: %v: %s", err, excerpt(body))
	}
	if strings.TrimSpace(result.AccessToken) == "" {
		s.t.Fatalf("the server authenticated and returned no token: %s", excerpt(body))
	}
	s.token = result.AccessToken
}

// repository is one entry in the server's repositories list, which is the thing
// an operator pastes an address into.
type repository struct {
	Name    string `json:"Name"`
	URL     string `json:"Url"`
	Enabled bool   `json:"Enabled"`
}

// setRepositories replaces the whole list, which is what the endpoint takes.
// Replacing rather than appending is what keeps the two halves of this check
// apart: one catalogue is visible to the server at a time, so an entry that
// appeared cannot have come from the other one.
func (s *server) setRepositories(ctx context.Context, repos ...repository) {
	s.t.Helper()
	s.must(ctx, http.MethodPost, "/Repositories", repos)
	for _, r := range repos {
		s.t.Logf("the server now carries one repository: %s", r.URL)
	}
}

// pkg is the part of a catalogue entry the server reports back.
type pkg struct {
	Name     string `json:"name"`
	GUID     string `json:"guid"`
	Versions []struct {
		Version string `json:"version"`
	} `json:"versions"`
}

// waitForCatalogueEntry waits until the server's own package list carries the
// plugin, which is the step an operator sees as the repository filling in.
//
// It is a wait rather than a read because the server refreshes its list on its
// own schedule after a repository is added, and an empty list read too early is
// exactly what an unreachable address looks like.
func (s *server) waitForCatalogueEntry(ctx context.Context, name string) {
	s.t.Helper()
	deadline := time.Now().Add(catalogueWait)
	var seen []string
	for time.Now().Before(deadline) {
		status, body, err := s.call(ctx, http.MethodGet, "/Packages", nil)
		if err == nil && status == http.StatusOK {
			var packages []pkg
			if err := json.Unmarshal(body, &packages); err != nil {
				s.t.Fatalf("the server's package list is not readable: %v: %s", err, excerpt(body))
			}
			seen = seen[:0]
			for _, p := range packages {
				seen = append(seen, p.Name)
				if strings.EqualFold(p.Name, name) {
					s.t.Logf("the server lists %s with %d version(s)", p.Name, len(p.Versions))
					return
				}
			}
		}
		sleep(ctx, pollEvery)
	}
	s.t.Fatalf("the server did not list %s within %s. What it listed instead was %v, and an empty list here is "+
		"what an operator is shown for an address that answers with nothing at all.", name, catalogueWait, seen)
}

// install asks the server to install the plugin from one repository.
//
// The answer says the request was accepted and never that the install
// succeeded: the server downloads, verifies and installs afterwards, which is
// why both halves of this check read the plugin list rather than this status.
func (s *server) install(ctx context.Context, name, repositoryURL string) {
	s.t.Helper()
	path := "/Packages/Installed/" + url.PathEscape(name) + "?repositoryUrl=" + url.QueryEscape(repositoryURL)
	s.must(ctx, http.MethodPost, path, nil)
	s.t.Logf("the server accepted the install request for %s from %s", name, repositoryURL)
}

// pluginAppears polls the installed plugin list for the guid and says how long
// it waited, so a negative result carries the length of the wait behind it.
func (s *server) pluginAppears(ctx context.Context, guid string, within time.Duration) (bool, time.Duration) {
	s.t.Helper()
	started := time.Now()
	deadline := started.Add(within)
	want := normaliseGUID(guid)
	for time.Now().Before(deadline) {
		status, body, err := s.call(ctx, http.MethodGet, "/Plugins", nil)
		if err == nil && status == http.StatusOK {
			var plugins []struct {
				ID     string `json:"Id"`
				Name   string `json:"Name"`
				Status string `json:"Status"`
			}
			if err := json.Unmarshal(body, &plugins); err != nil {
				s.t.Fatalf("the server's plugin list is not readable: %v: %s", err, excerpt(body))
			}
			for _, p := range plugins {
				if normaliseGUID(p.ID) == want {
					s.t.Logf("the server reports %s installed, status %s", p.Name, p.Status)
					return true, time.Since(started).Round(time.Second)
				}
			}
		}
		sleep(ctx, pollEvery)
	}
	return false, time.Since(started).Round(time.Second)
}

// readPublishedCatalogue reads the manifest this repository publishes out of the
// tree.
func readPublishedCatalogue(t *testing.T) []map[string]any {
	t.Helper()
	const path = "../../docs/manifest.json"
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var catalogue []map[string]any
	if err := json.Unmarshal(body, &catalogue); err != nil {
		t.Fatalf("%s is not a catalogue a server could read: %v", path, err)
	}
	if len(catalogue) == 0 {
		t.Fatalf("%s carries no plugin, so there is nothing for a server to install and a green here would say nothing", path)
	}
	return catalogue
}

// onePlugin picks the entry both halves are about and refuses a catalogue that
// cannot supply one.
func onePlugin(t *testing.T, catalogue []map[string]any) (name, guid string) {
	t.Helper()
	for _, entry := range catalogue {
		versions, _ := entry["versions"].([]any)
		if len(versions) == 0 {
			continue
		}
		name, _ = entry["name"].(string)
		guid, _ = entry["guid"].(string)
		if name != "" && guid != "" {
			return name, guid
		}
	}
	t.Fatal("no entry in the published catalogue carries a name, a guid and a version, so there is nothing installable to ask a server about")
	return "", ""
}

// withDigest rewrites every version's checksum, leaving the archive URL alone.
//
// Leaving the URL is the whole point. The server downloads the real archive and
// hashes it, so what it refuses is the pairing rather than a broken link, and a
// URL pointing at nothing would prove a different thing.
func withDigest(t *testing.T, catalogue []map[string]any, digest string) []byte {
	t.Helper()
	if len(digest) != DigestLength {
		t.Fatalf("the planted digest is %d characters and the published field is %d", len(digest), DigestLength)
	}
	damaged := make([]map[string]any, 0, len(catalogue))
	rewritten := 0
	for _, entry := range catalogue {
		copied := map[string]any{}
		for k, v := range entry {
			copied[k] = v
		}
		versions, _ := entry["versions"].([]any)
		out := make([]any, 0, len(versions))
		for _, v := range versions {
			version, ok := v.(map[string]any)
			if !ok {
				out = append(out, v)
				continue
			}
			next := map[string]any{}
			for k, value := range version {
				next[k] = value
			}
			if was, ok := next["checksum"].(string); ok && was != digest {
				next["checksum"] = digest
				rewritten++
			}
			out = append(out, next)
		}
		copied["versions"] = out
		damaged = append(damaged, copied)
	}
	if rewritten == 0 {
		t.Fatal("no checksum was rewritten, so the catalogue served below is the published one and this half would prove nothing")
	}
	body, err := json.Marshal(damaged)
	if err != nil {
		t.Fatalf("encoding the damaged catalogue: %v", err)
	}
	t.Logf("%d checksum(s) rewritten, every archive url left alone", rewritten)
	return body
}

// serveCatalogue serves the bytes at an address the server under test can reach,
// and returns that address.
//
// The listener takes a port the operating system chooses, which is above 1024 by
// construction. decisions/headless-and-unelevated.md refuses a port below it,
// and a check that asked for one would need rights this harness may not take.
func serveCatalogue(t *testing.T, serverBase string, body []byte) string {
	t.Helper()
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listening for the catalogue this half serves: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	port := listener.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf("http://%s:%d/manifest.json", reachableHost(t, serverBase), port)
}

// reachableHost is the name the server under test would use for this machine.
//
// A server on this machine reaches it back on the loopback address. A server
// somewhere else reaches it on whatever name this run was given for the server,
// which is the best guess available and is wrong for a server that cannot route
// back at all. That case fails as the catalogue never being listed, and the
// failure says so.
func reachableHost(t *testing.T, serverBase string) string {
	t.Helper()
	parsed, err := url.Parse(serverBase)
	if err != nil {
		t.Fatalf("reading %s as an address: %v", AddressVariable, err)
	}
	host := parsed.Hostname()
	if host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return "127.0.0.1"
	}
	return host
}

// normaliseGUID compares identifiers the way two spellings of one guid should
// compare: case and hyphens are presentation.
func normaliseGUID(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, "-", ""))
}

// excerpt bounds what a failure quotes back.
func excerpt(body []byte) string {
	const most = 400
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "(an empty body)"
	}
	if len(text) > most {
		return text[:most] + "..."
	}
	return text
}

// sleep waits, and stops waiting when the run is over.
func sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
