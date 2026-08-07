# hub

The hub holds the generated plugin manifest, the published site and the design system that eleven native clients are held to. It is what makes the plugins installable at all, since no manifest exists today. The manifest is generated from the release artifacts of each plugin repository rather than maintained by hand, and its URL is treated as a permanent commitment because a manifest that moves breaks every installation silently. This is also the first board in a second GitHub organisation and therefore the test of whether the fleet tooling really derives the owner from the roster instead of assuming it.

Planning happens on the issue tracker first. Every decision that shapes
the architecture is written down there with its reasons before the code
that depends on it exists.

See [NOTICE.md](NOTICE.md) for the intended-use notice.
