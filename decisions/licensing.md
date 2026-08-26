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

**Under CC-BY-4.0**, whose text is in
[LICENSES/CC-BY-4.0.txt](../LICENSES/CC-BY-4.0.txt), attribution required:

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

Not the CC-BY-4.0 text any more. Both licences this tree grants under are
reproduced here, the second one fetched from the licensor rather than typed:

    git ls-files | grep -i -E '^(LICENSE|COPYING|LICENSES)'
    LICENSE
    LICENSES/CC-BY-4.0.txt

Run 2026-08-26 on the change that added the second file, for #140.
`LICENSES/CC-BY-4.0.txt` is the plain-text legal code served at
`https://creativecommons.org/licenses/by/4.0/legalcode.txt`, and the commit that
added it names that address, because a licence text reproduced from recollection
is the one artefact a reader will not check against its source.

The AGPL text is not copied into that directory beside it. `LICENSE` stays at
the root, where the detector below reads it and where a reader looks first, and
one text held in two places is a text that can drift with nothing here to refuse
the drift.

What the tree still does not carry is one surface that reports both. The
repository detector reads the root file and only the root file, which is what a
single root `LICENSE` gets:

    gh api repos/Flowfin/hub --jq '.license.spdx_id'
    AGPL-3.0

That field holds one licence and cannot hold two, which is a property of the
field rather than a judgement about it:

    gh api repos/Flowfin/hub --jq '.license | type'
    object

Both run 2026-08-26. That field is derived from the root file, so a second text
under `LICENSES/` was never going to move it, and a reader who takes it for the
whole answer still misses this file. What the platform can be made to say
elsewhere, in a description or a page of its own, is not read here and no claim
is made about it: the reason this declaration is a file in the tree is that the
tree is where a licence question is answered, not that no other surface exists.

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
