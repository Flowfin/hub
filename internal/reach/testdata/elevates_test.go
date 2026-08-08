// Fixture. The refusal has to happen without the call being made: an elevated
// call on a machine with somebody at it is a consent prompt that takes the
// screen, which is the failure the rule exists to stop.
package fixture

import "testing"

func TestPlantedElevation(t *testing.T) {
	mustRun(t, "sudo", "setcap", "cap_net_bind_service=+ep", "./server")
}
