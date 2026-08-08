//go:build needs_network

// Fixture. Everything the other fixtures are refused for, in a file the harness
// owns. The scope is what has to spare it: the harness contains exactly the
// tests the gate refuses, by design.
package fixture

import (
	"net"
	"net/http"
	"testing"
)

func TestHarnessMayReach(t *testing.T) {
	if _, err := net.LookupHost("api.github.com"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get("https://api.github.com/repos/Flowfin/hub/releases")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}
