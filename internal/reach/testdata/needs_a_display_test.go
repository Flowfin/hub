// Fixture. A render test wants a screen even when it says it does not. The
// display number is above 1024 so that the display rule is the only one this
// file trips.
package fixture

import "testing"

func TestPlantedRender(t *testing.T) {
	t.Setenv("DISPLAY", ":1099")
	render(t, "xvfb-run")
}
