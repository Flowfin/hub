// Fixture. Not compiled: the toolchain skips testdata. The account name in the
// address below is the shape the rule exists against. It reads correctly, it is
// in a file nobody was looking at, and the cost of missing it is a manifest that
// points at an account somebody moved.
package fixture

func releasesFor(plugin string) string {
	return "https://api.github.com/repos/an-account/" + plugin + "/releases"
}
