# Where a plugin's identity comes from

Every entry in the manifest carries identity before it carries a single version:
a stable identifier, a name, a category, a one-line overview, a longer
description, the owner shown to whoever is installing, and sometimes an image.
`decisions/manifest-schema.md` says what each field is. This file says which
bytes each value is read out of, and what happens when they are not there.

None of it is typed into this repository. A copied name is wrong from the moment
the plugin renames itself, and a copied identifier is worse: a server keys an
installed plugin by `guid`, so a stale or mistyped one turns an update into a
second plugin sitting beside the first. `decisions/manifest-is-generated.md` is
the general form of that argument and this is the case it bites hardest in.

## What is decided

**The values come from the descriptor a release ships beside its archive**, the
asset whose name is the archive's name followed by `.meta.json`. The archive is
the one `decisions/artifact-checksum-pairing.md` selects, so the descriptor is
tied to the bytes it describes by its own name rather than by a second predicate
somebody has to keep in step.

**One release supplies all of them, and it is the newest release that carries a
readable descriptor**, ordered by the four-component number the descriptor itself
declares in `version`, compared the way `decisions/manifest-schema.md` compares
versions. Two descriptors declaring the same version are ordered by their release
tag, descending, byte by byte, so the order is total and two runs over one
history cannot disagree.

**Identity is one answer per plugin and does not depend on the channel being
generated.** The version list is per channel, which is `decisions/channel-model.md`.
Identity is not. Entry 5 of #1 puts the same plugin in two published files, and a
plugin whose `guid` appears twice under two different names is a defect a server
has no way to resolve.

**There is no fallback to an older release.** If the newest release carrying a
descriptor carries one that cannot be read, or one that is missing a value the
schema requires, the plugin is refused by name and contributes nothing. Reading
the one behind it would publish a name the plugin has stopped using, and it would
do it silently, which is the failure this whole file exists against.

## Which fields, and which of them may be absent

Required: `guid`, `name`, `description`, `overview`, `owner`, `category`. A
descriptor missing any of them, or carrying it as whitespace, refuses the plugin.
`decisions/manifest-schema.md` measured the two entry shapes the ecosystem's own
catalogue uses and those six are in both of them.

Optional: `imageUrl`. Absent in eleven of the thirty-four entries measured there,
so a plugin with no artwork is ordinary rather than defective, and the key is
left out where there is no value for it.

`guid` is also checked for shape, because it is the one field whose damage is
invisible. It must be eight, four, four, four and twelve hexadecimal characters
separated by hyphens, and it is emitted lowercase. A value of another shape is
refused rather than passed through, since a server that cannot parse it treats
the entry as a plugin it has never seen.

`owner` is read from the descriptor like the rest. Entry 4 of #1 decided that the
catalogue carries the account that publishes the releases, and the reason given
there was that the field should show the same name as the release it came from.
Reading it out of the release is that decision implemented, and it opens no
second place where the name is decided. Writing the answer into this generator would
be refused by `no-hardcoded-names`, which is `decisions/names-are-data.md`.

## Why the sidecar rather than the build descriptor

The releases in the declared set ship two files carrying the same identity set. A
per-release `build.yaml`, and the per-archive `.zip.meta.json` beside each
archive. Both were read before choosing. Measured 2026-08-10:

    gh api 'repos/Flowfin/jellyfin-plugin-sso/releases?per_page=1' --jq '.[0].assets[].name'
    build.yaml
    community-sso-for-jellyfin_5.0.0.43.md5
    community-sso-for-jellyfin_5.0.0.43.sha256
    community-sso-for-jellyfin_5.0.0.43.zip
    community-sso-for-jellyfin_5.0.0.43.zip.md5sum
    community-sso-for-jellyfin_5.0.0.43.zip.meta.json
    sbom.cyclonedx.json
    sbom.cyclonedx.sha256

Three reasons, in the order they weighed.

The sidecar is JSON and the build descriptor is YAML, and this tree has no
dependency at all:

    cat go.mod
    module flowfin.dev/hub

    go 1.25

Choosing the YAML file adds a parser to a module that requires nothing, for a
file the other one duplicates. `decisions/means.md` asks whether a means adds a
dependency the tree does not already carry and whether that cost is paid
knowingly; here the cost buys nothing, because `encoding/json` reads the other
file today.

The sidecar is named after the archive, so selecting it is the archive selection
that already exists plus a suffix. `build.yaml` is a fixed name in a release that
may one day ship two archives, and then it describes which of them.

The sidecar carries only what the manifest publishes plus `version`, `targetAbi`
and `timestamp`. `build.yaml` carries build-side fields the manifest has no use
for, and a reader has to know which of them to ignore.

The cost is stated: a release that ships a `build.yaml` and no sidecar carries
identity this generator will not read, and is refused rather than parsed by the
other route. That holds the tree to one file format, and two readers of one fact
disagree the first time one of them is corrected.

## What this costs, and the cost is live today

Identity is read from the newest release carrying a descriptor whichever channel
that release is in, so a plugin that renames itself in a test build shows the new
name in the finished catalogue before any finished release carries it. That is
the intended direction. The name follows the plugin, and the alternative is a
catalogue that shows an old name for as long as the plugin has not cut a finished
release.

It is not hypothetical here. In the one declared repository with a release
history, every descriptor is on the test side of the channel split. Measured
2026-08-10:

    gh api 'repos/Flowfin/jellyfin-plugin-sso/releases?per_page=100' --jq '
      [ .[] | {finished: (.tag_name|test("^v?[0-9]+[.][0-9]+[.][0-9]+([.][0-9]+)?(-stable)?$")),
               descriptor: ([.assets[].name | select(endswith(".zip.meta.json"))] | length)} ]
      | {releases: length,
         finished: [.[]|select(.finished)]|length,
         finished_with_descriptor: [.[]|select(.finished and .descriptor>0)]|length,
         test_with_descriptor: [.[]|select((.finished|not) and .descriptor>0)]|length}'
    {"finished":4,"finished_with_descriptor":0,"releases":56,"test_with_descriptor":52}

Four finished releases, none with a descriptor, and fifty-two test builds with
one. Under a channel-scoped identity rule that plugin has no name at all in the
file an operator would paste. Under this rule it has the name its newest build
declares, which is the name the plugin actually goes by.

That measurement says something else which is not this file's to answer. Those
same four releases are where the finished channel's version entries would have to
come from, and `version`, `targetAbi`, `changelog` and `timestamp` are all
descriptor values. A finished channel built from them today carries no version
entries at all, whatever identity says. `decisions/failure-posture.md` is where
that lands and #28 is where the run reports it.

## What refuses a departure

`internal/identity`, under the gate's `test` leg. It refuses a release with no
descriptor beside its archive, a descriptor that is not JSON, a descriptor
missing any required field or carrying it as whitespace, a `guid` that is not the
canonical shape, and a plugin no release of which carries a readable descriptor.
Every one of those refusals names the plugin, because a run that says identity
could not be read without saying whose sends a reader to twelve repositories in
turn.

What no check here can decide is whether the descriptor is telling the truth. A
release declaring a `guid` that belongs to another plugin is well-formed, and
nothing in this tree can tell it apart from a correct one. `PROSE, NOT
ENFORCEMENT` for that half.

## What this does not settle

Where `version`, `targetAbi`, `changelog` and `timestamp` are read from. They sit
in the same file, they belong to the version entry, and nothing here reads them.

What the run does with a refused plugin. That is
`decisions/failure-posture.md`, which already places a defect in the newest
release for a plugin and channel on the fatal side and an older one on the loud
skip side, and #28 is how the report is shaped.
