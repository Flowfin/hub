// Fixture. The name appears here, in a comment quoting the command that produced
// a number:
//
//	gh api repos/an-account/jellyfin-plugin-sso/releases --jq 'length'
//	54
//
// A check refusing this would be a check against citing evidence, so it is not
// refused.
package fixture

func nothing() string { return "no name here" }
