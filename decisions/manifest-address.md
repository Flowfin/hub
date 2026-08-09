# The manifest address is a commitment that cannot be withdrawn

An operator adds this project to a Jellyfin server by pasting one address into
the repositories list. From that moment the server polls that address and nothing
else. Changing it raises no error on the server and produces no warning in the
interface. The server keeps asking the old address, the plugins stop updating,
and the operator finds out only if somebody notices a version standing still.

So the address is treated as permanent from the day it is first published, and
the two rules below follow from that.

## Which address shapes are refused

A `github.io` address is refused. It contains an account name and a repository
name, and both can be renamed by somebody who does not know what depends on
them. A rename produces a working site at the new name and a dead address at the
old one, which is exactly the silent case above.

A `raw.githubusercontent.com` address on a branch is refused. It contains an
account name, a repository name and a branch name, so it carries all of the
above plus one more thing that a routine cleanup can move, and it puts a third
party's caching behaviour between the file and every server that fetches it.

What is left is an address under a name this project holds and renews. Which
name, at which registrar, and who pays for it, is not settled by this file. That
is entry 2 of #1.

## When an address may be printed

An install address may appear in a tracked file only once the address answers
with the file it promises. Not once the name is registered, and not once the
host resolves. The test is a request for the address itself, and the evidence is
that request's output recorded where the address is argued.

The reason for the stricter form is in the current state of the tree. The name
resolves and the site answers, and the printed install address still does not:

    curl -sS -o /dev/null -w "%{http_code}\n" https://flowfin.dev/
    200
    curl -sS -o /dev/null -w "%{http_code}\n" https://flowfin.dev/manifest.json
    404

Both run 2026-08-08 against the tree at 6a98de6. A rule written against DNS
alone would read that pair as satisfied.

Two tracked files printed the address, and #35 removed both:

    grep -rno 'https://[a-zA-Z0-9./_-]*manifest.json' -- README.md docs/ ; echo "exit=$?"
    exit=1

Run 2026-08-08. Nothing is printed and the grep exits 1. What the operator-facing
files say instead is that installation is not available yet, which is true and is
what an address that answers 404 leaves them able to say.

The pair of requests above stays here because this is where the address is
argued, and recording a 404 is not the same act as printing an install
instruction.

`Gate: install-address-is-answered` is what refuses the next file to print one.
It reads every tracked file, recognises an address on the host `docs/CNAME`
declares whose last path segment ends in `manifest.json`, and refuses it unless
that exact address is recorded in `internal/address.Answered` as having been read
and found to answer. That list is empty, so the leg refuses every printed install
address today, which is the sentence above with a machine behind it.

Two things sit outside it, each for its own reason. `decisions/` is not read,
because this directory is where an address is argued and therefore where the
request showing that it does NOT answer is recorded; the paragraph above is that
case and three decision files hold one. An address on somebody else's host is not
read either: this tree quotes the Jellyfin project's own catalogue as the
measurement two decisions were derived from, and refusing that would be refusing
evidence for being evidence. Where a third party's address rots, that is a link,
and `internal/links` holds it under the harness.

Adding an entry to that list is the act this rule turns on, and it costs the
request, its output written here, and the list changed in the same commit. What
keeps the entry honest afterwards is the harness check
`TestEveryRecordedInstallAddressStillAnswers`, which re-reads every recorded
address and refuses one that has stopped answering, or that answers with
something a server cannot read as a catalogue. An address that answered once and
stopped is silent on every server that already has it.

The operator instruction that prints the address is the other half of #34 and
cannot be written until that list is not empty.

## What this costs

Publishing nothing until the address answers means the project is not
installable in the meantime, which is the state it is in anyway. The cost lands
later instead: once the address is published it cannot be tidied, consolidated
or moved to a shorter name, and a lapsed renewal is a silent outage for every
installation rather than an error anybody sees.
