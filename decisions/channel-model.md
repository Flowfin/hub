# The channel model, and what keeps a test build out of the finished list

The manifest format has no concept of a channel. A server reads one address and
shows every version it finds there, filtered only by target ABI. So an operator
who wants finished builds and an operator who wants to try the next one cannot be
served by one list unless something outside the format separates them.

This has to be settled before an address is committed to, because a second address
can be added later for the price of adding it, and the first one cannot be changed
at all. That asymmetry is `decisions/manifest-address.md`.

## How many addresses exist

The generator produces one file per channel, and each channel that is switched on
has its own address. Today one channel is switched on, the finished one, so one
address exists.

Whether a test channel is ever offered to the public is entry 5 of #1 and is not
decided here. What is decided here is that answering it is a configuration change
rather than a redesign: the classification below runs on every release whatever
the answer, so a second channel is a second output file from a split that has
already been made, and the generator does not learn a new concept on the day
somebody says yes.

The cost of that is one line of machinery carried for something nobody may ever
ask for. It is small, and the alternative is discovering on the day of the request
that every release ever published has to be reclassified.

## What enters each

A release enters the finished list when its tag matches the pattern its
declaration carries for that plugin, in the `stable_tags` field described in
`decisions/source-set-declaration.md`.

Every other release is a test build. Not "everything not matched is discarded":
the test channel is where they go if it is switched on, and where they would go if
it were, and either way a run says how many landed on each side.

Nothing else moves a release between the lists. Not the release's own flag, not a
word anywhere in its body, not its position in the list the API returns.

## The separator, and why the tag rather than the flag

The obvious separator is the pre-release flag on the release itself, and it is
wrong here for a reason that is measurable rather than theoretical. The flag and
the tag disagree in this repository, on ten of fifty-two releases:

    gh api 'repos/iderex/jellyfin-plugin-sso/releases?per_page=100' \
      --jq '{total: length, flagged_prerelease: [.[]|select(.prerelease)]|length,
             tag_says_beta: [.[]|select(.tag_name|test("beta"))]|length,
             beta_tag_flagged_stable: [.[]|select((.tag_name|test("beta")) and (.prerelease|not))]|length}'
    {"beta_tag_flagged_stable":10,"flagged_prerelease":38,"tag_says_beta":48,"total":52}

Run 2026-08-08. Ten releases whose tag says beta are not flagged as pre-releases.
The reason a project does that is ordinary and has nothing to do with the
catalogue: a release that is not flagged is the one the repository page shows as
the latest, and here that is a beta:

    gh api repos/iderex/jellyfin-plugin-sso/releases/latest --jq '{tag: .tag_name, prerelease}'
    {"prerelease":false,"tag":"4.3.0-beta.27"}

Run 2026-08-08. Under a flag-based separator those ten builds are in the finished
list, and the four-component version of a test build can sort above the finished
build it was testing against, so an operator who asked for finished builds is
offered a beta as the newest thing available.

The one sentence for why the tag is trusted instead: the tag is chosen at the same
moment as the intent and never changes afterwards, while the flag is a display
setting on the release page that somebody can move later for a reason that has
nothing to do with what the build is.

## Why the pattern is declared rather than derived

The tempting rule is to treat a tag as a test build when it carries a semantic
version pre-release part, meaning anything after the first hyphen. It is wrong
here, and it is worth writing down which tag breaks it, because the tag looks like
the safest one in the set:

    gh api 'repos/iderex/jellyfin-plugin-sso/releases?per_page=100' \
      --jq '[.[].tag_name | select(test("beta")|not)]'
    ["4.2.1-stable","4.2.0-stable","4.1.1-stable","v4.1.0.0"]

Run 2026-08-08. `4.2.1-stable` has a pre-release part that says the opposite of
what a pre-release part means. A rule reading hyphens puts the three finished
releases in the test channel and leaves the finished list with one entry.

So the separator is per plugin and it is declared. For the one sourced repository
with releases today the pattern is

    ^v?[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?(-stable)?$

and it selects exactly those four tags out of fifty-two:

    gh api 'repos/iderex/jellyfin-plugin-sso/releases?per_page=100' --jq '.[].tag_name' > tags.txt
    python -c "
    import re
    p=re.compile(r'^v?[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?(-stable)?\$')
    t=[x.strip() for x in open('tags.txt') if x.strip()]
    m=[x for x in t if p.match(x)]
    print('total',len(t),'matched',len(m),m)"
    total 52 matched 4 ['4.2.1-stable', '4.2.0-stable', '4.1.1-stable', 'v4.1.0.0']

Run 2026-08-08.

A declared pattern is itself something that can be wrong, and the answer to that
is not a better pattern. It is that a pattern which selects nothing, or everything,
is visible: the loader refuses a record whose pattern does not compile, and a run
names how many releases landed on each side of the split for every declared
plugin. `decisions/failure-posture.md` is what a run does about the counts and #28
is how it reports them. A pattern that quietly emptied a channel would otherwise
be indistinguishable from a plugin that stopped releasing.

## What this costs

A plugin that changes its tagging convention needs its declaration changed, and
until somebody does, its new releases sit on the wrong side. That is a visible
failure rather than a silent one, for the reason in the paragraph above, and it is
the price of not guessing.

The classification is made from the tag alone, so a release that was mistagged is
classified by the mistake. Retagging is not something this project can do to
somebody else's release, and the repair is a new release. Publishing a correction
by hand is not available, because `decisions/manifest-is-generated.md` says the
file is not edited.
