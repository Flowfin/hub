# What the first release is

Defined from the operator's side, because a release defined as a list of finished
issues can be complete and still not do anything for anybody.

## What an operator can do

Open a Jellyfin server, go to the plugin repositories list, paste one address,
and see a plugin they can install. Install it. Have it work.

That sequence is the whole test. Everything else in this plan exists to make it
boring, and any part of it that still needs a person to explain something is not
finished.

Two things have to be true alongside it. The address the operator pasted keeps
working afterwards without anybody touching the server, which is what
`decisions/manifest-address.md` is about. And the operator can find out what the
plugin does and who published it from inside the server interface, without
opening a browser.

## Which plugins the catalogue carries

One entry, and it is `requests`. #65 settled that on 2026-08-21 against the
counts below rather than against the ones this section was first written with,
and `sources/sso.json` carries the answer as `"enabled": false` with the
condition that turns it back on.

    for r in discover invites metadata-sync requests server-pairing share-links \
             smart-collections sso stats watch-sync watchlist whisper-subtitles; do
      printf "%s %s\n" "$r" "$(gh api "repos/Flowfin/jellyfin-plugin-$r/releases?per_page=100" --jq 'length')"
    done

Run 2026-08-21: 74 for `sso`, 1 for `requests`, 0 for the other ten. Both counts
move, so re-run it rather than citing it.

Two repositories have published and only one of them can produce an entry, which
is the fact the choice turns on rather than the totals above. Every release `sso`
has published on the finished side of the channel split ships the archive, an
md5 and a sha256, and none of them ships the `.zip.meta.json` descriptor beside
it:

    gh api 'repos/Flowfin/jellyfin-plugin-sso/releases?per_page=100' \
      --jq '.[] | select(.tag_name|test("^v?[0-9]+[.][0-9]+[.][0-9]+([.][0-9]+)?(-stable)?$")) | {tag: .tag_name, assets: [.assets[].name]}'
    {"assets":["sso-authentication_4.2.1.0.md5","sso-authentication_4.2.1.0.sha256","sso-authentication_4.2.1.0.zip"],"tag":"4.2.1-stable"}
    {"assets":["sso-authentication_4.2.0.0.md5","sso-authentication_4.2.0.0.sha256","sso-authentication_4.2.0.0.zip"],"tag":"4.2.0-stable"}
    {"assets":["sso-authentication_4.1.1.0.md5","sso-authentication_4.1.1.0.sha256","sso-authentication_4.1.1.0.zip"],"tag":"4.1.1-stable"}
    {"assets":["sso-authentication_4.1.0.0.md5","sso-authentication_4.1.0.0.sha256","sso-authentication_4.1.0.0.zip"],"tag":"v4.1.0.0"}

    gh api 'repos/Flowfin/jellyfin-plugin-requests/releases?per_page=100' \
      --jq '.[] | {tag: .tag_name, prerelease, draft, assets: [.assets[].name]}'
    {"assets":["requests_0.1.0.0.md5","requests_0.1.0.0.sha256","requests_0.1.0.0.zip","requests_0.1.0.0.zip.meta.json"],"draft":false,"prerelease":false,"tag":"0.1.0.0-stable"}

Both run 2026-08-21. `decisions/plugin-identity.md` reads the guid, the name, the
description, the overview, the owner, the category and the version entry's four
fields out of that descriptor, so a release without one produces no entry, and
`decisions/failure-posture.md` stops the whole run on a defect in the newest
release of a channel. Left switched on, that repository does not shorten the
catalogue by one entry; it stops the catalogue existing.

So the choice is between a catalogue with one entry and no catalogue at all, and
one entry is the answer for two reasons that are worth separating. It makes
something installable that is not installable today, which is the gap this
repository was opened to fill. And it exercises the catalogue's machinery against
a real server while there is one plugin to debug rather than twelve, instead of
leaving the generator, the publication route and the manifest format unproven
until the day eleven things need them at once.

The cost is real and belongs here rather than in the argument for it. The plugin
the whole channel split was measured against is the one left out, so the first
thing this catalogue proves is proved without the case it was designed for. The
two alternatives cost more. Waiting for a finished `sso` tag carrying a
descriptor means no catalogue for a period nobody here controls. Widening
`stable_tags` until the finished channel picks up what that repository itself
calls unfinished would give two entries immediately and would make "finished"
mean one thing in the catalogue and another on the board it came from, at exactly
the point where the catalogue makes a promise to a stranger.

Nothing in this tree makes turning that declaration back on fall due, and the
`note` in the record is the whole of the reminder. So the run prints the note of
every disabled declaration on every run, which is what keeps a deliberate absence
from reading like the ordinary case.

The counterpart is honesty in the interface. A catalogue with one entry has to
look like a catalogue with one entry rather than like a catalogue that failed to
load, which is the same distinction #28 draws for the generator's own output and
the same one #11 draws for the run's verdict.

## What is deliberately not in it

The ten plugins that have published nothing. They enter the catalogue on the day
they publish a finished release, through the same pipeline and with no change
here, and that is the property the first release is proving.

`sso` is the eleventh and is not one of them. It is left out by a field somebody
set, so it comes back by a field somebody sets, which is the one place in this
list where a person has to act rather than a repository.

A test channel. Entry 5 of #1 answered on 2026-08-11 that pre-release builds get
no second address, so the first release ships the finished list and there is no
second list waiting behind it.

The clients. This repository holds the design system they are held to, not the
clients themselves, and nothing an operator does in the sequence above involves
one.

A conformance programme for the design system. Milestone 6 makes the numbers
checkable, which is worth doing and is not what makes the sequence above work.

Anything that changes because the catalogue grows. Search, categories, sorting
and pagination are all real once there are twelve entries and all of them are
noise at one.

## What blocks it

These are the open issues the sequence above cannot happen without. The list is
a judgement, and the current state of each one is:

    for n in 1 5 7 11 17 18 19 24 25 26 27 28 31 32 33 34 35 51 53 54 55; do
      gh issue view "$n" --repo Flowfin/hub --json number,state,title \
        --jq '"\(.number)\t\(.state)\t\(.title)"'
    done

Three answers out of #1 are load-bearing here and all three are now given. The
license, because a catalogue with no license is a catalogue nobody can package,
and `LICENSE` at the root carries one. The address, because it is the one thing
an operator types and it cannot be changed afterwards. And whose name the
catalogue carries, because it is rendered on every server that lists a plugin,
which entry 4 answered on 2026-08-11 with the publishing organisation. The
remaining two entries do not block either: the site's language does not touch
the sequence, and entry 5 settled that there is no second channel to build.

None of that is a plugin entry. What stands between this list and an install is
work rather than an answer, and the address is where it shows:

    curl -sS -o /dev/null -w "%{http_code}\n" https://flowfin.dev/
    200
    curl -sS -o /dev/null -w "%{http_code}\n" https://flowfin.dev/manifest.json
    404

Run 2026-08-11 against the tree at 864f747, and the same two codes on 2026-08-08.
The site has answered on both dates and the manifest has never existed, so the
404 is the generator and the publication route rather than anything the domain
decision left undone.

## What does not block it

The gate parity work in milestone 7, the design system token file and its checks
in milestone 6, the site's link checking in #36, and the site's language in #38.
Each is worth doing and none of them is between an operator and a working
install. Saying which is which now is the point of this file, because at the end
every one of them will look like it was nearly done.
