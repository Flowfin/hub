# The manifest is generated from the releases, never edited by hand

The rule in one sentence: `manifest.json` is produced by a program that reads the
release artifacts, and a change to it is made by changing an input or the
generator, never by opening the file.

## The alternative that was refused

Writing the entries by hand and keeping them in step with the releases. Every
field in a version entry already exists somewhere else, so a hand-written entry is
a copy of a value that lives in the release. The copy is correct on the day it is
typed and stays correct only for as long as nobody ships anything.

It was refused because the copy that goes stale is the one nobody can see is
stale. A wrong name or a wrong description is visible to anyone who reads the
page. A wrong checksum is thirty-two hexadecimal characters that look exactly like
the right thirty-two.

The cost of generating instead is real and it is paid once: a job that runs, a
credential that lets it read releases, and somewhere to put the output. None of
that exists here yet. That is a cost, not an argument against the rule.

## The failure the rule prevents

A server does not display a bad checksum. It downloads the package, hashes what it
got, compares, and stops:

    curl -sSL -o im.cs https://raw.githubusercontent.com/jellyfin/jellyfin/master/Emby.Server.Implementations/Updates/InstallationManager.cs
    grep -n 'MD5.HashDataAsync\|package.Checksum, hash' im.cs
    558:                var hash = Convert.ToHexString(await MD5.HashDataAsync(stream, cancellationToken).ConfigureAwait(false));
    559:                if (!string.Equals(package.Checksum, hash, StringComparison.OrdinalIgnoreCase))

Run 2026-08-08. What the operator sees is an install that does not finish, and
nothing in the interface saying which of the two numbers was wrong.

The whole failure is reproducible without a server, because the only number that
matters is one anybody can compute. Take the archive of the one sourced release
that exists today and hash it:

    B=https://github.com/iderex/jellyfin-plugin-sso/releases/download/4.3.0-beta.27
    curl -sSL -o sso.zip "$B/community-sso-for-jellyfin_4.3.0.27.zip"
    md5sum sso.zip
    c2b9ab45ca368b55ecd88527df302302 *sso.zip

Run 2026-08-08. That is the value a version entry for `4.3.0.27` has to carry, and
a hand-written entry carries whatever was pasted into it. Change one character of
it and the comparison on line 559 fails for every operator on every server, with
the file still well-formed JSON and every other field still right.

There is a sharper version of the same failure, and it is not hypothetical. The
same release carries a checksum for a different file:

    curl -sSL "$B/sbom.cyclonedx.sha256"
    81f0e61a062f2d5fa05d7641f8d2dc7361e3eeb7074bdfa27f82eee5830c00cd  sbom.cyclonedx.json

Run 2026-08-08. Pasting that value against the archive produces an entry that
looks stronger than the right one and fails every install.
`decisions/artifact-checksum-pairing.md` is the rule that keeps the generator from
making the same mistake automatically, which is a separate rule from this one:
generating the file removes the transcription, and pairing correctly is what the
generator has to do once the transcription is gone.

## What the rule buys beyond correctness

A hand-editable file cannot be checked, because there is no second source to check
it against. A generated file can: regenerate from unchanged inputs and compare.
`decisions/manifest-schema.md` fixes the byte format so that comparison is exact,
and #29 is the test. Under that pair, an edit made to the published file outside
the generator shows up as a diff the next time the generator runs, rather than
surviving until somebody notices.

## What this does not settle

Where the generator runs, how often, and what it does when a release cannot be
read. Those are #31, #32 and `decisions/failure-posture.md`.
