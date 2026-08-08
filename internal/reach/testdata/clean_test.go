// Fixture. A gate test doing ordinary things, including a documentation URL and
// a loopback address, neither of which is a reach.
package fixture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlantedClean(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("[]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const source = "https://example.com/plugins/thing_1.0.0.0.zip"
	const local = "127.0.0.1:8096"
	if source == "" || local == "" {
		t.Fatal("unreachable")
	}
}
