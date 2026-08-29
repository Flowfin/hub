package carry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Every test here runs against a server on the loopback interface, which is
// what decisions/headless-and-unelevated.md requires of anything the gate
// compiles. It is also the only way the writing half can be exercised at all: a
// test that proved this against the real repository would prove it once and
// leave a branch and a pull request behind.
//
// The whole package is driven through Client rather than through a fake
// satisfying the two interfaces. The interfaces exist so that the decisions can
// be judged without the network, and the loopback server delivers that; what
// driving the real client adds is that the addresses, the payload shapes and the
// statuses are judged too, and those are exactly the parts a fake would agree
// with while the server did not.

const (
	// The names here are a fixture vocabulary and are nobody's account.
	// no-hardcoded-names refuses a declared account name in source, and a test
	// carrying the real one would be the thing that check exists against.
	repository = "an-owner/a-repository"
	served     = "main"
	filePath   = "docs/manifest.json"
	proposedOn = "place/docs/manifest.json"
)

// forge is the part of the API this package uses, standing up as a server.
type forge struct {
	// heads is every branch the repository holds, and what it points at.
	heads map[string]string

	// files is what each branch carries, by path.
	files map[string]map[string]string

	// pulls is what has been opened, in the order it was opened.
	pulls []map[string]any

	// commits is one entry per write that the server accepted, so a test can
	// say a run committed nothing rather than only that the content is right.
	commits []map[string]any

	// created is one entry per branch the run asked for, which is what a test
	// asserting the standing branch was not recreated reads.
	created []string

	// broken makes the read of one path answer with a server error, and
	// brokenBody is what that answer carries. The pair is how a read that did
	// not happen is told from a read that found no difference: the failing
	// answer is a well-formed file carrying exactly the bytes the run built, so
	// a client that read the body and not the status reports a current
	// catalogue and passes.
	broken     string
	brokenBody string

	// asked is every address the run sent, so that a test can say what the
	// client put in front of the server rather than only what the server made
	// of it.
	asked []string

	// encoding overrides what a file read answers with, for the one case where
	// the endpoint returns a blob and no bytes.
	encoding string

	next int
}

func newForge() *forge {
	return &forge{
		heads: map[string]string{served: "0000000000000000000000000000000000000000"},
		files: map[string]map[string]string{},
		next:  40,
	}
}

// put places content on a branch without going through the API, which is how a
// test says what the world looked like before the run.
func (f *forge) put(branch, path, content string) {
	if f.files[branch] == nil {
		f.files[branch] = map[string]string{}
	}
	f.files[branch][path] = content
}

func blobOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func (f *forge) commit(branch string) string {
	f.next++
	return fmt.Sprintf("%040d", f.next)
}

func (f *forge) handler(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	base := "/repos/" + repository

	mux.HandleFunc("GET "+base+"/{$}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"default_branch": %q}`, served)
	})

	mux.HandleFunc("GET "+base+"/git/ref/heads/{branch...}", func(w http.ResponseWriter, r *http.Request) {
		head, ok := f.heads[r.PathValue("branch")]
		if !ok {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		fmt.Fprintf(w, `{"object": {"sha": %q}}`, head)
	})

	mux.HandleFunc("POST "+base+"/git/refs", func(w http.ResponseWriter, r *http.Request) {
		payload := decode(t, r)
		name := strings.TrimPrefix(payload["ref"].(string), "refs/heads/")
		if _, taken := f.heads[name]; taken {
			http.Error(w, "Reference already exists", http.StatusUnprocessableEntity)
			return
		}
		f.heads[name] = payload["sha"].(string)
		f.created = append(f.created, name)
		// A branch created from another one carries that one's files.
		for path, content := range f.files[branchAt(f, payload["sha"].(string))] {
			f.put(name, path, content)
		}
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{}`)
	})

	mux.HandleFunc("GET "+base+"/contents/{path...}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		if path == f.broken {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"type": "file", "sha": %q, "size": %d, "encoding": "base64", "content": %q}`,
				blobOf(f.brokenBody), len(f.brokenBody),
				base64.StdEncoding.EncodeToString([]byte(f.brokenBody)))
			return
		}
		branch := r.URL.Query().Get("ref")
		content, ok := f.files[branch][path]
		if !ok {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		encoding := f.encoding
		if encoding == "" {
			encoding = "base64"
		}
		fmt.Fprintf(w, `{"type": "file", "sha": %q, "size": %d, "encoding": %q, "content": %q}`,
			blobOf(content), len(content), encoding, base64.StdEncoding.EncodeToString([]byte(content)))
	})

	mux.HandleFunc("PUT "+base+"/contents/{path...}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		payload := decode(t, r)
		branch, _ := payload["branch"].(string)
		if _, held := f.heads[branch]; !held {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		// The fast-forward the whole design rests on: the write lands only
		// where the blob it names is the one the branch holds. A test that
		// accepted any sha would let a rewrite pass unnoticed.
		current, present := f.files[branch][path]
		named, _ := payload["sha"].(string)
		if present != (named != "") || (present && named != blobOf(current)) {
			http.Error(w, "Conflict", http.StatusConflict)
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(payload["content"].(string))
		if err != nil {
			t.Errorf("the content sent is not base64: %v", err)
		}
		f.put(branch, path, string(decoded))
		made := f.commit(branch)
		f.heads[branch] = made
		f.commits = append(f.commits, payload)
		status := http.StatusOK
		if !present {
			status = http.StatusCreated
		}
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"commit": {"sha": %q}}`, made)
	})

	mux.HandleFunc("GET "+base+"/pulls", func(w http.ResponseWriter, r *http.Request) {
		head := strings.TrimPrefix(r.URL.Query().Get("head"), owner()+":")
		for i, p := range f.pulls {
			if p["head"] == head {
				fmt.Fprintf(w, `[{"number": %d}]`, i+1)
				return
			}
		}
		io.WriteString(w, `[]`)
	})

	mux.HandleFunc("POST "+base+"/pulls", func(w http.ResponseWriter, r *http.Request) {
		payload := decode(t, r)
		f.pulls = append(f.pulls, payload)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"number": %d}`, len(f.pulls))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the run asked for %s %s, which is not an address this package should use", r.Method, r.URL)
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	// The addresses are recorded before the server unescapes anything. A
	// handler reading only what the mux made of a path cannot tell a slash from
	// an escaped one, because it unescapes both to the same value, and the real
	// server does not.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.asked = append(f.asked, r.Method+" "+r.RequestURI)
		mux.ServeHTTP(w, r)
	})
}

func owner() string {
	cut := strings.IndexByte(repository, '/')
	return repository[:cut]
}

// branchAt is which branch a sha is the head of, so that a created branch can
// be given the files of the one it was created from.
func branchAt(f *forge, sha string) string {
	for name, head := range f.heads {
		if head == sha {
			return name
		}
	}
	return ""
}

func decode(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Fatalf("the payload sent does not decode: %v", err)
	}
	return payload
}

// run drives one carry against the server and returns what it printed.
func run(t *testing.T, f *forge, on string, content string) (string, error) {
	t.Helper()
	server := httptest.NewServer(f.handler(t))
	t.Cleanup(server.Close)

	client := New()
	client.API = server.URL
	client.Repository = repository
	client.Token = "a-token"

	var out bytes.Buffer
	err := Carry(context.Background(), &out, on, Change{
		Path:   filePath,
		Branch: proposedOn,
		Bytes:  []byte(content),
	}, client, client)
	return out.String(), err
}

func TestACatalogueTheServedBranchDoesNotCarryIsProposedOnOneRequest(t *testing.T) {
	f := newForge()
	f.put(served, filePath, "the old catalogue")

	out, err := run(t, f, served, "the new catalogue")
	if err != nil {
		t.Fatalf("carrying: %v", err)
	}

	if got := f.files[proposedOn][filePath]; got != "the new catalogue" {
		t.Errorf("the standing branch carries %q rather than the new catalogue", got)
	}
	if got := f.files[served][filePath]; got != "the old catalogue" {
		t.Errorf("the served branch was written to, and it carries %q", got)
	}
	if len(f.pulls) != 1 {
		t.Fatalf("%d request(s) were opened rather than one", len(f.pulls))
	}
	if got := f.pulls[0]["base"]; got != served {
		t.Errorf("the request is against %v rather than %s", got, served)
	}
	if got := f.pulls[0]["title"]; got != Title {
		t.Errorf("the request is titled %v", got)
	}

	body, _ := f.pulls[0]["body"].(string)
	// internal/hygiene refuses a request naming no issue, and this request is
	// judged by the same gate as one opened by hand.
	if !strings.Contains(body, fmt.Sprintf("#%d", ArguedIn)) {
		t.Errorf("the body names no issue, so the gate's own hygiene leg would refuse it:\n%s", body)
	}
	// The trap a reader has no other way to find out about.
	if !strings.Contains(body, "will not start on their own") {
		t.Errorf("the body does not say the checks do not start by themselves:\n%s", body)
	}
	if !strings.Contains(out, "pull request #1 is open against main") {
		t.Errorf("the run did not say which request it opened:\n%s", out)
	}
}

func TestASecondRunConvergesOnTheOneOpenRequest(t *testing.T) {
	f := newForge()
	f.put(served, filePath, "the old catalogue")

	if _, err := run(t, f, served, "one catalogue"); err != nil {
		t.Fatalf("the first carry: %v", err)
	}
	out, err := run(t, f, served, "a later catalogue")
	if err != nil {
		t.Fatalf("the second carry: %v", err)
	}

	if len(f.pulls) != 1 {
		t.Errorf("the second run opened a second request, and there are now %d", len(f.pulls))
	}
	if len(f.created) != 1 {
		t.Errorf("the standing branch was created %d time(s)", len(f.created))
	}
	if len(f.commits) != 2 {
		t.Errorf("%d commit(s) were made rather than one per catalogue", len(f.commits))
	}
	if got := f.files[proposedOn][filePath]; got != "a later catalogue" {
		t.Errorf("the standing branch carries %q rather than the later catalogue", got)
	}
	if !strings.Contains(out, "pull request #1 is the one already open") {
		t.Errorf("the run did not name the request that was already open:\n%s", out)
	}
}

func TestARunThatWouldRepeatTheStandingBranchCommitsNothing(t *testing.T) {
	f := newForge()
	f.put(served, filePath, "the old catalogue")

	if _, err := run(t, f, served, "the new catalogue"); err != nil {
		t.Fatalf("the first carry: %v", err)
	}
	before := len(f.commits)
	out, err := run(t, f, served, "the new catalogue")
	if err != nil {
		t.Fatalf("the second carry: %v", err)
	}

	if len(f.commits) != before {
		t.Errorf("%d commit(s) were made where the branch already carried these bytes", len(f.commits)-before)
	}
	if !strings.Contains(out, "already carries these bytes") {
		t.Errorf("the run did not say why it committed nothing:\n%s", out)
	}
	if len(f.pulls) != 1 {
		t.Errorf("%d request(s) are open rather than one", len(f.pulls))
	}
}

func TestACurrentCatalogueOpensNothingAndSaysSo(t *testing.T) {
	f := newForge()
	f.put(served, filePath, "the catalogue")

	out, err := run(t, f, served, "the catalogue")
	if err != nil {
		t.Fatalf("carrying: %v", err)
	}

	if len(f.pulls) != 0 || len(f.commits) != 0 || len(f.created) != 0 {
		t.Errorf("a current catalogue wrote something: %d request(s), %d commit(s), %d branch(es)",
			len(f.pulls), len(f.commits), len(f.created))
	}
	if !strings.Contains(out, "nothing to carry") || !strings.Contains(out, "no pull request was opened") {
		t.Errorf("the run did not say that it carried nothing:\n%s", out)
	}
}

func TestARunOffTheServedBranchCarriesNothingAndSaysSo(t *testing.T) {
	f := newForge()
	f.put(served, filePath, "the old catalogue")

	out, err := run(t, f, "a-branch", "the new catalogue")
	if err != nil {
		t.Fatalf("carrying: %v", err)
	}

	if len(f.pulls) != 0 || len(f.commits) != 0 || len(f.created) != 0 {
		t.Errorf("a run off the served branch wrote something: %d request(s), %d commit(s), %d branch(es)",
			len(f.pulls), len(f.commits), len(f.created))
	}
	if !strings.Contains(out, "not carried") || !strings.Contains(out, "a-branch") {
		t.Errorf("the run did not say why it carried nothing:\n%s", out)
	}
}

func TestTheStandingBranchIsCommittedOnRatherThanMoved(t *testing.T) {
	f := newForge()
	f.put(served, filePath, "the old catalogue")
	// A standing branch left behind by an earlier request, with the served
	// branch since moved on. Resetting it to the served head is what a
	// force-push would do, and the request would then read as one commit that
	// keeps changing underneath whoever is reviewing it.
	f.heads[proposedOn] = "1111111111111111111111111111111111111111"
	f.put(proposedOn, filePath, "a catalogue from an earlier run")
	f.heads[served] = "2222222222222222222222222222222222222222"

	if _, err := run(t, f, served, "the new catalogue"); err != nil {
		t.Fatalf("carrying: %v", err)
	}

	if len(f.created) != 0 {
		t.Errorf("the standing branch was created again: %v", f.created)
	}
	if len(f.commits) != 1 {
		t.Fatalf("%d commit(s) were made rather than one", len(f.commits))
	}
	// The commit replaced the blob the standing branch held, which the server
	// refuses unless it is the one named. A run that had reset the branch first
	// would have named the served branch's blob and been refused.
	if got := f.commits[0]["sha"]; got != blobOf("a catalogue from an earlier run") {
		t.Errorf("the commit named the blob %v, which is not the one the standing branch held", got)
	}
}

func TestAServedBranchCarryingNoCatalogueYetIsADifference(t *testing.T) {
	f := newForge()

	if _, err := run(t, f, served, "the first catalogue"); err != nil {
		t.Fatalf("carrying: %v", err)
	}
	if len(f.pulls) != 1 {
		t.Fatalf("%d request(s) were opened rather than one", len(f.pulls))
	}
	if len(f.commits) != 1 {
		t.Fatalf("%d commit(s) were made rather than one", len(f.commits))
	}
	// A file that is not there is created rather than replaced, and a run that
	// named a blob for it would be refused by the server.
	if _, named := f.commits[0]["sha"]; named {
		t.Errorf("the commit named a blob to replace where the branch carried no file")
	}
}

func TestAReadThatDidNotHappenIsNotACurrentCatalogue(t *testing.T) {
	f := newForge()
	f.put(served, filePath, "the old catalogue")
	f.broken, f.brokenBody = filePath, "the new catalogue"

	out, err := run(t, f, served, "the new catalogue")
	if err == nil {
		t.Fatalf("a failed read of the served branch passed as a current catalogue:\n%s", out)
	}
	if !strings.Contains(err.Error(), filePath) {
		t.Errorf("the failure does not name what could not be read: %v", err)
	}
	if len(f.pulls) != 0 || len(f.commits) != 0 {
		t.Errorf("a failed read still wrote something")
	}
}

func TestAFileTooLargeToBeReadBackIsRefusedRatherThanGuessedAt(t *testing.T) {
	f := newForge()
	f.put(served, filePath, "the old catalogue")
	// What the endpoint answers with past its own limit: the blob, the size,
	// and no bytes. Read as a difference, that would reopen the same request
	// every night for as long as the file stayed large.
	f.encoding = "none"

	if _, err := run(t, f, served, "the new catalogue"); err == nil {
		t.Fatal("a file the endpoint would not return the bytes of was read as a difference")
	} else if !strings.Contains(err.Error(), fmt.Sprint(ContentLimit)) {
		t.Errorf("the failure does not name the limit it hit: %v", err)
	}
}

func TestABranchTheRepositoryDoesNotHoldReadsAsAState(t *testing.T) {
	f := newForge()
	server := httptest.NewServer(f.handler(t))
	t.Cleanup(server.Close)

	client := New()
	client.API = server.URL
	client.Repository = repository

	if _, err := client.Head(context.Background(), "no-such-branch"); err == nil {
		t.Fatal("a branch the repository does not hold read as one that it does")
	} else if !strings.Contains(err.Error(), ErrNoBranch.Error()) {
		t.Errorf("an absent branch answered %v rather than the state", err)
	}

	head, err := client.Head(context.Background(), served)
	if err != nil {
		t.Fatalf("reading the served branch: %v", err)
	}
	if head != f.heads[served] {
		t.Errorf("the served branch reads as %s rather than %s", head, f.heads[served])
	}
}

func TestTheDerivedBranchNameSurvivesTheAddressItIsSentTo(t *testing.T) {
	// The standing branch and the placed path both carry slashes, which the
	// endpoints take as part of the address rather than inside one segment. A
	// run that escaped them asks the real server for a branch nobody holds and
	// reports every night that it created one.
	//
	// It is asserted against the address rather than against the server's
	// answer, because a server unescapes %2F back to a slash and answers
	// identically either way. That is exactly the shape a fixture agrees with
	// while the real service does not.
	f := newForge()
	f.put(served, filePath, "the old catalogue")

	if _, err := run(t, f, served, "the new catalogue"); err != nil {
		t.Fatalf("carrying: %v", err)
	}
	if _, held := f.heads[proposedOn]; !held {
		t.Errorf("the branch the run created is not %s: %v", proposedOn, f.heads)
	}
	for _, address := range f.asked {
		// The path only. Inside a query value an escaped slash is correct and
		// is what the pull request endpoint is handed.
		path, _, _ := strings.Cut(address, "?")
		if strings.Contains(strings.ToUpper(path), "%2F") {
			t.Errorf("the run escaped a slash inside a path: %s", address)
		}
	}
	if len(f.asked) == 0 {
		t.Error("no address was recorded, so this test asserted nothing")
	}
}
