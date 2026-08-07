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

One. The single sourced repository that has releases.

    for r in discover invites metadata-sync requests server-pairing share-links \
             smart-collections sso stats watch-sync watchlist whisper-subtitles; do
      printf "%s %s\n" "$r" "$(gh api "repos/iderex/jellyfin-plugin-$r/releases?per_page=100" --jq 'length')"
    done

Run 2026-08-08: 52 for `sso`, 0 for the other eleven.

So the first release either waits for plugins that do not exist yet, or it ships
a catalogue with one entry. One entry is the answer, for two reasons that are
worth separating.

It makes something installable that is not installable today, which is the gap
this repository was opened to fill. And it exercises the catalogue's machinery
against a real server while there is one plugin to debug rather than twelve. The
alternative leaves the generator, the publication route and the manifest format
unproven until the day eleven things need them at once, which is the worst
possible day to find out that a field is wrong.

The counterpart is honesty in the interface. A catalogue with one entry has to
look like a catalogue with one entry rather than like a catalogue that failed to
load, which is the same distinction #28 draws for the generator's own output and
the same one #11 draws for the run's verdict.

## What is deliberately not in it

The other eleven plugins. They enter the catalogue when they publish a release,
through the same pipeline and with no change here, and that is the property the
first release is proving.

A test channel. Whether pre-release builds get a second address at all is entry 5
of #1 and is not settled. The first release ships the stable list only, which
does not foreclose the answer.

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
a judgement rather than a derivation, and the current state of each one is:

    for n in 1 5 7 11 17 18 19 24 25 26 27 28 31 32 33 34 35 51 53 54 55; do
      gh issue view "$n" --repo Flowfin/hub --json number,state,title \
        --jq '"\(.number)\t\(.state)\t\(.title)"'
    done

Three answers out of #1 are load-bearing here. The license, because a catalogue
with no license is a catalogue nobody can package. The address, because it is the
one thing an operator types and it cannot be changed afterwards. And whose name
the catalogue carries, because it is rendered on every server that lists a
plugin. The remaining two entries of #1 do not block: the site's language does
not touch the sequence, and the test channel is out of scope above.

On the address, the name now answers even though the manifest does not:

    curl -sS -o /dev/null -w "%{http_code}\n" https://flowfin.dev/
    200
    curl -sS -o /dev/null -w "%{http_code}\n" https://flowfin.dev/manifest.json
    404

Run 2026-08-08. That is a fact about DNS and hosting rather than a decision, and
entry 2 of #1 is what settles the decision.

## What does not block it

The gate parity work in milestone 7, the design system token file and its checks
in milestone 6, the site's link checking in #36, and the site's language in #38.
Each is worth doing and none of them is between an operator and a working
install. Saying which is which now is the point of this file, because at the end
every one of them will look like it was nearly done.
