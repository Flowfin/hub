# How an artifact and its checksum are paired

A release carries several files. A plugin archive, checksum sidecars in more than
one digest, a build descriptor, an SBOM and the SBOM's own checksum. The manifest
publishes exactly one archive URL and exactly one checksum per version entry, and
the two have to belong to each other.

They come apart when the checksum is chosen by position instead of by name.
Picking the last asset whose name ends in a given suffix, or the first one a sort
returns, works until a rename moves a different file into that position. When
that happened in a neighbouring project the manifest published the SBOM's
checksum against the plugin archive, and every install on every server failed
verification with nothing in the interface explaining it.

## How the archive is selected

The archive is selected first, on its own, by a stated predicate over the
release's asset list. The predicate is a filename suffix and nothing else about
the asset's position in the list.

The suffix is `.zip`. In the release listed in the next section it selects one
asset, and the two assets whose names contain `.zip` in the middle are not
selected, which is the difference between a suffix and a substring.

Exactly one asset in a release may satisfy the predicate. Zero is a release the
generator cannot use. Two or more is the ambiguity this whole file exists
against, and the generator does not break the tie, because every tie-break
available to it is an ordering it did not choose.

## How the checksum is derived from it

The sidecar names its own subject inside the file, and that name is what the
pairing is made on. Not the sidecar's filename, and not its position in the asset
list.

A real release is what settles this, because the filename convention people
assume is there is not there. These are the assets of the newest release of the
one plugin that has any:

    gh api repos/iderex/jellyfin-plugin-sso/releases/latest --jq '.tag_name, ([.assets[].name] | sort | .[])'
    4.3.0-beta.27
    build.yaml
    community-sso-for-jellyfin_4.3.0.27.md5
    community-sso-for-jellyfin_4.3.0.27.sha256
    community-sso-for-jellyfin_4.3.0.27.zip
    community-sso-for-jellyfin_4.3.0.27.zip.md5sum
    community-sso-for-jellyfin_4.3.0.27.zip.meta.json
    sbom.cyclonedx.json
    sbom.cyclonedx.sha256

Run 2026-08-08. The archive is `community-sso-for-jellyfin_4.3.0.27.zip`, and it
has two MD5 sidecars under two different spellings. One drops the `.zip` before
the suffix and one keeps it and uses a different suffix. A rule saying "the
archive's name plus `.md5`" resolves neither of them, and a rule saying "the
asset ending in `.md5`" happens to work here and is the rule that published an
SBOM's checksum somewhere else.

Reading the contents removes the guesswork, because the sidecar format carries
the filename it is about:

    B=https://github.com/iderex/jellyfin-plugin-sso/releases/download/4.3.0-beta.27
    curl -sSL "$B/community-sso-for-jellyfin_4.3.0.27.md5"
    c2b9ab45ca368b55ecd88527df302302  community-sso-for-jellyfin_4.3.0.27.zip
    curl -sSL "$B/community-sso-for-jellyfin_4.3.0.27.zip.md5sum"
    c2b9ab45ca368b55ecd88527df302302 *community-sso-for-jellyfin_4.3.0.27.zip
    curl -sSL "$B/sbom.cyclonedx.sha256"
    81f0e61a062f2d5fa05d7641f8d2dc7361e3eeb7074bdfa27f82eee5830c00cd  sbom.cyclonedx.json

All run 2026-08-08. So the rule is:

A candidate sidecar is any asset that parses as a checksum line, meaning a hex
digest, whitespace, an optional `*`, and a filename. It is the archive's sidecar
only when that filename equals the selected archive's filename. The SBOM's
sidecar is refused on its contents rather than on a naming convention, which is
why the failure this file opens with cannot happen under this rule whatever
anybody renames.

The two MD5 spellings above agree, and the generator does not assume that.
Where more than one candidate names the archive, all of them are read and their
digests have to match. A disagreement is a release that is not published,
because two different answers about the same bytes is the one case where
publishing either would be a coin toss.

A release where that pairing does not resolve is not guessed at. It is skipped,
named in the run output, and counted, on the terms #11 sets for the run's
verdict.

## Which digest is published, and why that one

MD5, in hexadecimal.

Not because it is a good digest. It is the digest the server computes, and no
other value in the release is a candidate however much stronger it is. The
server downloads the package, hashes it, and compares the result against the
manifest's `checksum` field case-insensitively:

    curl -sSL -o im.cs https://raw.githubusercontent.com/jellyfin/jellyfin/master/Emby.Server.Implementations/Updates/InstallationManager.cs
    grep -n 'MD5.HashDataAsync\|package.Checksum, hash' im.cs
    558:                var hash = Convert.ToHexString(await MD5.HashDataAsync(stream, cancellationToken).ConfigureAwait(false));
    559:                if (!string.Equals(package.Checksum, hash, StringComparison.OrdinalIgnoreCase))

Run 2026-08-08. The same shape is visible in what the Jellyfin project publishes
today, where every checksum in the catalogue is 32 hexadecimal characters:

    curl -sSL -o jf-manifest.json https://repo.jellyfin.org/files/plugin/manifest.json
    python -c "import json;d=json.load(open('jf-manifest.json'));print(sorted({len(v['checksum']) for p in d for v in p['versions']}))"
    [32]

Run 2026-08-08.

Publishing a SHA-256 sidecar's value in that field would produce a manifest that
looks stronger and fails every install, because the comparison is a string
comparison against something the server computed with MD5. So a stronger sidecar
in the release is a good thing to ship and is not a candidate for this field.

What this costs is stated rather than argued away. MD5 is broken against
collision attacks, so this field is an integrity check against corruption and a
truncated download, and it is not evidence that the archive is the one the
project built. The property that would carry that weight is a signature over the
release, which is a different mechanism and is not this field.

## What happens when the pair cannot be resolved

Three cases and they are not the same case.

No asset matches the archive predicate. The release ships nothing this generator
recognises, which is a distinct thing from a repository with no releases at all,
and the output says which.

Two or more assets match the archive predicate. The generator refuses to choose
and names both, because choosing here is exactly the failure the file opens with.

An archive is selected and no sidecar names it. The version is skipped and named.
A version entry is never published with an empty checksum field or with a
checksum taken from a sidecar that names something else, because a server that
cannot verify a download will not install it, and an entry that cannot be
installed is worse than an entry that is not offered.

Two or more sidecars name the archive and their digests disagree. The version is
skipped and both values are named, for the reason in the section above.

None of these is guessed at, and none of them silently produces a shorter
versions list. Whether a skip ends the run or is carried is #11, and how the skip
is reported is #28.
