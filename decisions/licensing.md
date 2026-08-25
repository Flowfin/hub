# Two licences over one tree, declared by parts

This tree holds a program, a published site and a design system. The program is
under the AGPL. The design system and the words on the published pages are under
CC-BY-4.0. The boundary between them does not run between files, so it is
declared here, by part, and nowhere else.

That is entry 1 of #1, and it is the last entry of that issue to be answered.

## What was decided

**The code is AGPL-3.0-or-later.** The licence was taken by the maintainer on
2026-08-08 in entry 1 of #1 and applied to every board that carried none. The
`or-later` half is the option the licence's own application notice offers rather
than an addition to it, and it was made precise fleet-wide on 2026-08-09.

Entry 1 carried two answers naming different variants of the same licence for a
while: `AGPL-3.0-or-later` on 2026-08-09 and `AGPL-3.0-only` on 2026-08-11,
neither referring to the other. That was recorded in the issue on 2026-08-11 and
settled by the maintainer on 2026-08-24, who read the `-only` answer as a slip
and kept the variant the tree already stated. So this file states one variant and
does not carry a live disagreement forward.

**The design system and the words on the published pages are CC-BY-4.0.**
Decided by the maintainer on 2026-08-11, with the reason: a design system other
projects are invited to adopt must not raise code-licence questions for the
adopter. Attribution stays required.

**The form is one declaration by parts, not a header in each file.** Decided by
the maintainer on 2026-08-24, because the boundary does not run between files and
a per-file header would have to claim it does.

## Why a path table cannot carry it

Both served pages hold the page's own stylesheet, its markup and its script in
the same bytes as the words a reader reads. At `2a8f88a`:

    grep -n '<style>\|</style>\|<script>\|</script>' docs/index.html docs/design-system.html
    docs/index.html:5:<style>
    docs/index.html:22:</style>
    docs/design-system.html:4:<style>
    docs/design-system.html:149:</style>
    docs/design-system.html:411:<script>
    docs/design-system.html:447:</script>

    grep -c "" docs/index.html docs/design-system.html docs/design-tokens.json
    docs/index.html:71
    docs/design-system.html:447
    docs/design-tokens.json:276

Run 2026-08-25. A row saying `docs/` is under one licence is wrong about part of
every file it names, and a header at the top of either page would be a statement
about the whole file made in the one place a reader trusts it least.

Three shapes were open when the entry was recorded on 2026-08-11: a per-file
header, a declaration naming parts, and moving the prose into files of its own.
The third was not chosen, so a page's words stay in the page they are read from.

## The declaration

**Under AGPL-3.0-or-later**, whose text is in [LICENSE](../LICENSE):

- The generator, meaning `main.go`, `internal/` and `manifest/`, together with
  the suites and the specimen data that go with them.
- The site's own code: in `docs/index.html` and `docs/design-system.html`, the
  `<style>` block, the `<script>` block, and the elements, attributes and class
  names that arrange the page. That is a program in the sense that matters here,
  because it is what runs in a browser, and it is not part of what an adopter is
  asked to take.
- The documents: `README.md`, `CONTRIBUTING.md`, `decisions/`, and the other
  documents at the root of the tree. The answer of 2026-08-11 moved the design
  system and the page prose and named nothing else, so these stay where the tree
  already had them rather than following the page prose out.
- The declared source set under `sources/`, which is this project's data about
  which repositories it reads.

**Under CC-BY-4.0**, `https://creativecommons.org/licenses/by/4.0/`, attribution
required:

- The words a reader reads on `docs/index.html` and `docs/design-system.html`:
  the running text, the headings, the table contents and the captions. Not the
  markup they sit in.
- The design system as a whole, which is those words on `docs/design-system.html`
  plus the values in `docs/design-tokens.json`. The token file is on this side
  because it is the design system's values in the form an adopter is asked to
  take, and putting it on the other side would raise for the adopter exactly the
  code-licence question the answer exists to prevent.

**Under neither, because it is not this project's to license:**
`docs/manifest.json` is generated, and the names, descriptions and overviews in
it are read out of descriptors the listed repositories publish.
`decisions/manifest-is-generated.md` is how it is produced and
`decisions/names-are-data.md` is why no such value is typed here. The plugin
repositories the catalogue lists are separate repositories under their own terms,
and listing something is not a statement about its licence.

## What a reader who takes a whole file takes

Both. A served page is bytes under two licences, and copying the file copies both
parts of it. That is the cost of the boundary running inside a file rather than
between files, and it is stated here rather than argued away: an adopter who
wants only the design system takes the words and the token file and leaves the
page, and an adopter who wants the page takes it under both sets of terms.

## What the tree does not carry

The CC-BY-4.0 text. `LICENSE` holds the AGPL in full and there is no second
licence file:

    git ls-files | grep -i -E '^(LICENSE|COPYING|LICENSES)'
    LICENSE

Run 2026-08-25 at `2a8f88a`. So the second licence is named by identifier and
address above and is not reproduced in this repository, which is weaker than what
the first one gets. That is a gap rather than a decision, and #140 is where it is
held.

The repository detector reads the first licence and only the first one, which is
what a single root `LICENSE` gets:

    gh api repos/Flowfin/hub --jq '.license.spdx_id'
    AGPL-3.0

Run 2026-08-25. A reader who takes that field for the whole answer will miss this
file, and nothing on the platform side can be made to state two.

## What refuses a departure

Nothing. No leg of the gate reads a licence, and a file added tomorrow under
neither part of this declaration is refused by no route here. This is prose, and
the review is where a departure is caught.

The one thing that is mechanical is negative: `no-hardcoded-names` and the other
legs are printed by `go run .` and none of them has a licence as its subject.

## The listed plugins carry a different licence, on purpose

Entry 1 of #1 raised this in its own body: whether the plugin repositories the
catalogue lists should carry the same licence as the catalogue. They do not, and
that is an answer rather than an omission. The maintainer decided it on
2026-08-08 in the same comment that took AGPL-3.0 for this tree: the twelve
Jellyfin plugins stay GPL-3.0, because Jellyfin itself is under GPL-2.0 and
AGPL-3.0 is incompatible with it.

All twelve read that way today:

    gh api "orgs/Flowfin/repos?per_page=100" --jq '.[] | select(.name|startswith("jellyfin-plugin-")) | .name + " " + .license.spdx_id' | sort
    jellyfin-plugin-discover GPL-3.0
    jellyfin-plugin-invites GPL-3.0
    jellyfin-plugin-metadata-sync GPL-3.0
    jellyfin-plugin-requests GPL-3.0
    jellyfin-plugin-server-pairing GPL-3.0
    jellyfin-plugin-share-links GPL-3.0
    jellyfin-plugin-smart-collections GPL-3.0
    jellyfin-plugin-sso GPL-3.0
    jellyfin-plugin-stats GPL-3.0
    jellyfin-plugin-watchlist GPL-3.0
    jellyfin-plugin-watch-sync GPL-3.0
    jellyfin-plugin-whisper-subtitles GPL-3.0

Run 2026-08-25. So the question a reader of the catalogue would ask has an
answer, and the answer is that a catalogue under one licence lists plugins under
another because the plugin host's own licence decides the plugins and not this
tree.

This changes nothing about which licence reaches which byte here. The plugin
repositories are separate repositories under their own terms, listing something
is not a statement about its licence, and the twelve are named above as a
reading of what they carry rather than as a claim this file makes on their
behalf.
