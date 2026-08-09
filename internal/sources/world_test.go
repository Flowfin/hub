//go:build needs_network

// The declared source set, resolved against the world rather than against a
// fixture. This is the check the rest of the package cannot make: every
// classification in resolve_test.go is judged against a planted list, so a green
// there says the reader is right and says nothing about whether the repositories
// it names are still there.
//
// It is out of the gate and in the harness because it reaches the release API,
// which decisions/headless-and-unelevated.md puts outside the merge gate: a gate
// that reddens because somebody else's service is having an afternoon teaches
// everybody that red means nothing.
package sources_test

import (
	"context"
	"os"
	"testing"
	"time"

	"flowfin.dev/hub/internal/releases"
	"flowfin.dev/hub/internal/sources"
)

func TestTheDeclaredSetResolvesAgainstTheWorld(t *testing.T) {
	declarations, err := sources.Load(os.DirFS("../../" + sources.Dir))
	if err != nil {
		t.Fatalf("reading the declared set: %v", err)
	}
	if len(declarations) == 0 {
		t.Fatal("the declared set is empty, so this check would pass having read nothing")
	}

	client := releases.New()
	// Absent, the read still works against public repositories and meets the
	// rate limit sooner, which is a read that failed rather than a short list.
	client.Token = os.Getenv("GITHUB_TOKEN")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	resolutions := sources.Resolve(ctx, client, declarations)
	t.Log("\n" + sources.Report(resolutions))
	if err := sources.Judge(resolutions); err != nil {
		t.Fatalf("the declared set does not resolve: %v", err)
	}
}
