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
instruction. Turning the condition above into a check that refuses the next file
to print one is #34. This file states the rule and does not enforce it.

## What this costs

Publishing nothing until the address answers means the project is not
installable in the meantime, which is the state it is in anyway. The cost lands
later instead: once the address is published it cannot be tidied, consolidated
or moved to a shorter name, and a lapsed renewal is a silent outage for every
installation rather than an error anybody sees.
