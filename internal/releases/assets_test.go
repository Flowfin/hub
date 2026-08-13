package releases

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"flowfin.dev/hub/internal/pairing"
)

// serving answers every address with body, and reports how many times it was
// asked.
func serving(t *testing.T, status int, body []byte) (*Client, *int) {
	t.Helper()
	asked := 0
	return clientFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked++
		w.WriteHeader(status)
		w.Write(body)
	})), &asked
}

func TestAnAssetIsReadAsTheBytesItAnsweredWith(t *testing.T) {
	client, _ := serving(t, http.StatusOK, []byte("d41d8cd98f00b204e9800998ecf8427e  a-plugin_1.0.0.0.zip\n"))

	body, err := client.Fetching(context.Background())(pairing.Asset{
		Name: "a-plugin_1.0.0.0.zip.md5sum",
		URL:  client.API + "/download",
		Size: 74,
	})
	if err != nil {
		t.Fatalf("reading an asset that answered: %v", err)
	}
	if !strings.HasPrefix(string(body), "d41d8cd98f00b204e9800998ecf8427e") {
		t.Fatalf("the bytes read are not the ones served: %q", body)
	}
}

// TestAnAssetThatDoesNotAnswerIsAReadThatDidNotHappen holds the difference
// decisions/failure-posture.md turns on. A release that shipped nothing is a
// fact about the release; a status this cannot read is a fact about the
// response, and reporting the second as the first publishes a short catalogue
// that looks exactly like a correct one.
func TestAnAssetThatDoesNotAnswerIsAReadThatDidNotHappen(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			client, _ := serving(t, status, nil)

			_, err := client.Fetching(context.Background())(pairing.Asset{Name: "a.zip.md5sum", URL: client.API + "/download"})
			if err == nil {
				t.Fatalf("a %d answer was read as an asset", status)
			}
			// Not a *pairing.Refusal, which is the type the layers above read as
			// a release that cannot be published rather than as a read that did
			// not happen.
			var refusal *pairing.Refusal
			if errors.As(err, &refusal) {
				t.Fatalf("a %d answer was reported as a release that cannot be paired: %v", status, err)
			}
			if !strings.Contains(err.Error(), "a.zip.md5sum") {
				t.Errorf("the error does not name the asset: %v", err)
			}
		})
	}
}

// TestAnAssetLargerThanTheBoundIsRefusedRatherThanTruncated is the near miss for
// the bound: a truncated sidecar parses as a line naming nothing, which is a
// release skipped for a reason that did not happen.
func TestAnAssetLargerThanTheBoundIsRefusedRatherThanTruncated(t *testing.T) {
	client, _ := serving(t, http.StatusOK, make([]byte, MaxAssetBytes+1))

	_, err := client.Fetching(context.Background())(pairing.Asset{Name: "big.zip.meta.json", URL: client.API + "/download"})
	if err == nil {
		t.Fatal("an asset past the bound was read")
	}
	if !strings.Contains(err.Error(), "big.zip.meta.json") || !strings.Contains(err.Error(), strconv.Itoa(MaxAssetBytes)) {
		t.Errorf("the refusal names neither the asset nor the bound: %v", err)
	}
}

func TestAnAssetExactlyAtTheBoundIsRead(t *testing.T) {
	client, _ := serving(t, http.StatusOK, make([]byte, MaxAssetBytes))

	body, err := client.Fetching(context.Background())(pairing.Asset{Name: "at-the-bound", URL: client.API + "/download"})
	if err != nil {
		t.Fatalf("an asset at the bound was refused: %v", err)
	}
	if len(body) != MaxAssetBytes {
		t.Fatalf("read %d of the %d bytes served", len(body), MaxAssetBytes)
	}
}

// TestAnAssetWithNoAddressIsRefusedWithoutARequest guards the case a release
// list can produce: an asset entry the API answered with no download address.
// Asking for it would be a request to the empty string, which fails with a
// message naming no plugin and no release.
func TestAnAssetWithNoAddressIsRefusedWithoutARequest(t *testing.T) {
	client, asked := serving(t, http.StatusOK, []byte("never reached"))

	_, err := client.Fetching(context.Background())(pairing.Asset{Name: "addressless.zip"})
	if err == nil {
		t.Fatal("an asset with no address was read")
	}
	if !strings.Contains(err.Error(), "addressless.zip") {
		t.Errorf("the refusal does not name the asset: %v", err)
	}
	if *asked != 0 {
		t.Errorf("a request was made for an asset with no address")
	}
}

func TestAContextThatIsDoneStopsTheRead(t *testing.T) {
	client, asked := serving(t, http.StatusOK, []byte("never reached"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.Fetching(ctx)(pairing.Asset{Name: "a.zip", URL: client.API + "/download"}); err == nil {
		t.Fatal("a cancelled run read an asset")
	}
	if *asked != 0 {
		t.Errorf("a cancelled run made a request")
	}
}
