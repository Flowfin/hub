// Fixture. Binding below 1024 needs a capability the rule refuses to require.
package fixture

import "testing"

func TestPlantedBind(t *testing.T) {
	listenOn(t, ":443")
}
