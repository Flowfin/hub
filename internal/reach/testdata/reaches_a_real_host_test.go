// Fixture. A test that fetches from a host outside the runner, written with the
// package the check deliberately allows, so that the address rule is what has to
// catch it.
package fixture

import (
	"net/http"
	"testing"
)

func TestPlantedFetch(t *testing.T) {
	resp, err := http.Get("https://api.github.com/repos/Flowfin/hub/releases")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}
