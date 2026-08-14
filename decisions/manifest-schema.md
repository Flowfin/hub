# The manifest schema, and the byte format of the generated file

Two halves that are not the same kind of decision. The field names and types are
transcription: a Jellyfin server reads this file and the shape is not ours to
choose. The bytes around those fields are ours, and fixing them deliberately is
what makes a diff on the published file mean something.

## Where the schema comes from

The server deserialises the file into two types, and their attributes are the
field names:

    curl -sSL -o PackageInfo.cs https://raw.githubusercontent.com/jellyfin/jellyfin/master/MediaBrowser.Model/Updates/PackageInfo.cs
    curl -sSL -o VersionInfo.cs https://raw.githubusercontent.com/jellyfin/jellyfin/master/MediaBrowser.Model/Updates/VersionInfo.cs
    grep -o 'JsonPropertyName("[a-zA-Z]*")' PackageInfo.cs VersionInfo.cs

Run 2026-08-08. `PackageInfo` carries `name`, `description`, `overview`, `owner`,
`category`, `guid`, `versions` and `imageUrl`. `VersionInfo` carries `version`,
`changelog`, `targetAbi`, `sourceUrl`, `checksum`, `timestamp`, `repositoryName`
and `repositoryUrl`.

Getting one of those wrong does not produce an error a person sees. It produces a
repository that renders empty, or a plugin with no installable version, which is
the same symptom as a network fault.

## The plugin object

Emitted in this order, which is the order the ecosystem's own published catalogue
uses. All thirty-four entries in it use one of exactly two key orders, differing
only in whether the optional key is present:

    curl -sSL -o jf-manifest.json https://repo.jellyfin.org/files/plugin/manifest.json
    python -c "
    import json
    from collections import Counter
    d=json.load(open('jf-manifest.json'))
    for k,v in Counter(tuple(p.keys()) for p in d).items(): print(v,k)"
    11 ('guid', 'name', 'description', 'overview', 'owner', 'category', 'versions')
    23 ('guid', 'name', 'description', 'overview', 'owner', 'category', 'imageUrl', 'versions')

Run 2026-08-08.

`guid` is a string, the plugin's identity, lowercase hexadecimal with hyphens. It
comes from the release's own build descriptor and never from a value typed here,
which is `decisions/plugin-identity.md`. It is the one field a server matches an
installed plugin against, so a changed guid is a different plugin rather than an
update.

`name` is a string, the display name, from the same descriptor.

`description` is a string, the long text a server shows on the plugin's page, from
the same descriptor.

`overview` is a string, the one-line text a server shows in the catalogue list,
from the same descriptor.

`owner` is a string, shown to the person installing the plugin. It carries the
publishing organisation, `Flowfin`, which entry 4 of #1 settled on 2026-08-11 for
the reason that it is the name on the site and the domain the operator pasted.
The cost is in that answer and is taken deliberately: the organisation is not the
account that signs a release, so what a server displays and what a signature says
are two different names. The answer puts the repair on the published page, which
is to say plainly under which account releases are signed. The page does not say
it yet, and no check reads for it.

`category` is a string, the group a server files the plugin under, from the same
descriptor.

`imageUrl` is a string and is optional. Where a plugin has no artwork the key is
absent, not present and null, which is what the eleven entries above show. It sits
between `category` and `versions`.

`versions` is an array of version objects, described below, and it is last.

`repositoryName` and `repositoryUrl` exist on the server's type and are not
emitted. No entry in the published catalogue carries either, and the server
supplies both itself from the repository the file was fetched from. Emitting a
value there would be this project asserting something the server already knows
from having made the request.

## The version object

Emitted in this order, which is the order every one of the two hundred and
seventy-seven version entries in the published catalogue uses:

    python -c "
    import json
    from collections import Counter
    d=json.load(open('jf-manifest.json'))
    for k,v in Counter(tuple(v.keys()) for p in d for v in p['versions']).items(): print(v,k)"
    277 ('version', 'changelog', 'targetAbi', 'sourceUrl', 'checksum', 'timestamp')

Run 2026-08-08.

`version` is a string holding a four-component number. It comes from the release's
build descriptor rather than from the tag, because the tag and the version are
different strings in the one sourced release that exists: the tag is
`4.3.0-beta.27` and the descriptor's version is `4.3.0.27`. The server parses this
field as a version and compares it numerically, so a string that does not parse is
an entry the server drops.

`changelog` is a string, free text, from the release.

`targetAbi` is a string holding a four-component number, the lowest server version
the build runs on. The server filters on it, and an entry above the server's own
version is invisible to that server. That filter is why the version cap is per
target rather than overall, which is `decisions/version-cap.md`.

`sourceUrl` is a string, the URL of the archive. It is the release asset's own
download URL, selected by the predicate in
`decisions/artifact-checksum-pairing.md`.

`checksum` is a string, thirty-two lowercase hexadecimal characters, the MD5 of
the archive at `sourceUrl`. Why MD5 and not something stronger, and how the value
is paired with the archive, are both in
`decisions/artifact-checksum-pairing.md`.

`timestamp` is a string in RFC 3339 with a `Z` suffix and second precision, twenty
characters. Every entry in the published catalogue is that length:

    python -c "
    import json
    d=json.load(open('jf-manifest.json'))
    print(sorted({len(v['timestamp']) for p in d for v in p['versions']}))"
    [20]

Run 2026-08-08.

## The byte format

Four choices, and the reason for fixing them at all is that a regeneration from
unchanged inputs has to produce unchanged bytes. Once it does, a diff on the
published file means an input changed, and a file that was edited outside the
generator becomes visible the next time the generator runs. Without it every
regeneration produces noise and nothing can be read out of a diff.

**Indentation is four spaces**, and arrays and objects are broken across lines.
That is what the published catalogue uses, so an operator comparing this file
against the one they already know is not reading two different shapes. The cost is
size, and it is small against a file this shape: the published catalogue is 169365
bytes at four-space indentation.

**Line endings are LF, and the file ends with one newline.** The published
catalogue carries no carriage returns and ends without a final newline; this file
keeps the first and departs on the second, because a file with no trailing newline
produces a diff on its last line every time that line changes and every tool in
the way has an opinion about adding one. `.gitattributes` pins the tracked golden
fixture so the comparison is against the same bytes on every platform, which is
the failure #23 is about and is not left to a checkout setting.

**Keys are emitted in the order above, not sorted.** Sorted keys would put
`category` before `changelog` and scatter a version entry's identity through it.
The order above is fixed by the order of the fields in the types the generator
marshals, so it is a property of the source, and changing it is a change to a
struct that a reader sees.

**Strings carry their own characters.** An ampersand is `&` and not `\u0026`, an
angle bracket is `<` and not `\u003c`, and a non-ASCII character is emitted as
UTF-8. The published catalogue does the same, and the bytes say so:

    python -c "
    raw=open('jf-manifest.json','rb').read()
    print('literal &', raw.count(b'&'), 'escaped', raw.count(rb'\u0026'))
    print('literal <', raw.count(b'<'), 'escaped', raw.count(rb'\u003c'))
    print('any backslash-u escape', raw.count(rb'\u'))"
    literal & 61 escaped 0
    literal < 1 escaped 0
    any backslash-u escape 0

Run 2026-08-08. This one is named because the obvious way to write JSON in Go gets
it wrong. `json.Marshal` escapes those three characters by default, so a generator
that does the obvious thing produces a file that is valid, loads correctly, and
differs from this format in every changelog containing an ampersand. Turning it off
is one call on the encoder, and a golden comparison is what catches its absence.

## Ordering

**Plugins are ordered by `guid`, ascending, comparing the strings byte by byte.**
Not by name. A plugin's guid is fixed for the life of the plugin and its name is
not, so ordering by name means a rename moves an entry and produces a diff whose
size has nothing to do with what changed. Byte-wise rather than by any collation,
because a locale-dependent order is one that differs between the machine that
generated the file and the machine that reviewed it.

**Versions are ordered newest first, by the four-component number in `version`,
compared component by component as integers.** Not as strings: `2.4.0.10` sorts
below `2.4.0.9` in a string comparison and above it in the comparison the server
makes. Where two entries carry the same version the tie is broken by `timestamp`
descending, and where that also ties, by `targetAbi` descending, so the order is
total and does not depend on the order the releases came back from the API.

One version array holds every target line, interleaved by that single rule. The
per-target cap is applied when entries are selected, before they are ordered, so
the cap decides which entries are present and this rule decides only where they
sit.

The server does not depend on any of this. It sorts what it finds:

    curl -sSL -o im.cs https://raw.githubusercontent.com/jellyfin/jellyfin/master/Emby.Server.Implementations/Updates/InstallationManager.cs
    grep -n 'OrderByDescending' im.cs
    280:            foreach (var v in availableVersions.OrderByDescending(x => x.VersionNumber))

Run 2026-08-08. So the ordering in this file is for the person reading the diff,
not for the server, and that is the whole reason it has to be stated rather than
left to whatever order the generator happened to build the list in.

## The golden fixture

`manifest/testdata/golden-manifest.json` is in the tree and fixes every decision
above as bytes. It carries two plugins so the plugin ordering is visible, one with
`imageUrl` and one without so the optional key's absence is fixed, two target lines
on one plugin with five entries on one and two on the other so the per-target cap
is visible as a shape, and a changelog containing an ampersand, angle brackets, a
quotation mark, a backslash and a non-ASCII letter so the escaping is fixed by an
example a reader can run.

It is a fixture and its contents are fixture contents. The names, guids, URLs and
digests in it are invented, the host is the documentation domain reserved for
exactly this, and nothing in it is a claim about any real release. A fixture built
out of the real catalogue would prove the state of the world on the day it was
built, and the format is what needs proving.

The test that compares generator output against it byte for byte is #29. This file
and the fixture land together because a format decision written without an example
is a format decision two people read differently.

## What this does not settle

Where the plugin-level values come from in detail, which is
`decisions/plugin-identity.md`. And what the generator does when a value cannot be
resolved, which is `decisions/failure-posture.md`.

Two things stood here until #1 answered them on 2026-08-11. Whose name `owner`
carries, which is entry 4 and is now in the field list above. And whether the file
is served from one address or two, which is entry 5 and is one;
`decisions/channel-model.md` holds what follows, and the split it describes is
unchanged by the answer because it was built to survive either one.
