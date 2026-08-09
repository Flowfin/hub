//go:build needs_network

// The half of the rule only a request can decide.
//
// The gate leg asks whether a tracked file prints an address that has been
// recorded as answering. That record is a claim about the world made on the day
// somebody wrote it, and an address that answered once can stop answering: a
// lapsed renewal, an expired certificate, a publication run that stopped
// writing. Every one of those is silent on a Jellyfin server, which shows an
// empty repository and reports nothing.
//
// So this re-reads every recorded address and refuses one that has stopped
// answering with a manifest. It needs the network, which is why it is the
// harness's and never the gate's.
package address_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"flowfin.dev/hub/internal/address"
)

func TestEveryRecordedInstallAddressStillAnswers(t *testing.T) {
	// What was examined, said out loud. The list is empty today, and a run over
	// an empty list prints the same green as a run over a working address
	// unless it says which it was.
	t.Logf("%d recorded install address(es) to read", len(address.Answered))
	if len(address.Answered) == 0 {
		t.Log("nothing was requested; this run is evidence about no address at all, and the tracked tree may print none")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 60 * time.Second}

	for _, recorded := range address.Answered {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, recorded, nil)
		if err != nil {
			t.Errorf("%s: %v", recorded, err)
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("%s: the address could not be read, which a server shows as an empty repository: %v", recorded, err)
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s answered %s, and it is printed in this tree as an install address", recorded, resp.Status)
			continue
		}
		if readErr != nil {
			t.Errorf("%s: reading the body: %v", recorded, readErr)
			continue
		}

		// Answering is not enough. A holding page served at the address with a
		// 200 is the failure this exists for, because the server treats it the
		// same as an empty repository and says nothing.
		var catalogue []map[string]any
		if err := json.Unmarshal(body, &catalogue); err != nil {
			t.Errorf("%s answered 200 with something a server cannot read as a catalogue: %v", recorded, err)
			continue
		}
		t.Logf("%s: 200, %d plugin(s)", recorded, len(catalogue))
	}
}
