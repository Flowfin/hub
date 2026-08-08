// Fixture. Not compiled: the toolchain skips testdata, which is the only reason
// a file that violates the rule can be committed beside the check that refuses
// it. The address here is loopback, so the import is the only thing wrong with
// this file and the import rule is what has to catch it.
package fixture

import (
	"net"
	"testing"
)

func TestPlantedDial(t *testing.T) {
	conn, err := net.Dial("tcp", "127.0.0.1:8096")
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}
