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

What is left is an address under a name this project holds and renews. Which name
that is was settled in entry 2 of #1 on 2026-08-09 and registered the same day,
and it is the name `docs/CNAME` already carries. The registrar and the contact
details behind it are part of that answer and are not written down here.

The registration is where the obligation starts rather than where it ends. A
renewal that lapses raises no error anywhere: it is the silent case at the top of
this file arriving by a different route, with the name afterwards available to
whoever registers it next. The answer says it belongs under a watch rather than
in somebody's memory, and nothing in this tree is that watch:

    grep -rni 'renew\|expir' --include=*.go --include=*.yml .
    ./internal/address/answered_test.go:8:// lapsed renewal, an expired certificate, a publication run that stopped
    ./internal/sources/resolve.go:225:// that resolved nothing because a credential expired produces exactly the file a
    ./internal/sources/resolve_test.go:177:  // credentials expired produces exactly the file a correct run over an empty

Run 2026-08-11 at 864f747. Three comments, none of them a reader of a renewal
date. The nearest thing is the first of them, which asks whether the address
answers when somebody runs the harness, and that catches a lapse after it has
happened rather than before.

## When an address may be printed

An install address may appear in a tracked file only once the address answers
with the file it promises. Not once the name is registered, and not once the
host resolves. The test is a request for the address itself, and the evidence is
that request's output recorded where the address is argued.

The reason for the stricter form was in the state of the tree the day the rule
was written. The name resolved and the site answered, and the printed install
address did not:

    curl -sS -o /dev/null -w "%{http_code}\n" https://flowfin.dev/
    200
    curl -sS -o /dev/null -w "%{http_code}\n" https://flowfin.dev/manifest.json
    404

Both run 2026-08-08 against the tree at 6a98de6. A rule written against DNS
alone would have read that pair as satisfied, which is why this one is not.

## The reading that fills the list

The address answers now, and this is the request the entry rests on:

    curl -sS -o /dev/null -w "%{http_code}\n" https://flowfin.dev/manifest.json
    200

    curl -sS https://flowfin.dev/manifest.json \
      | python -c "import json,sys; d=json.load(sys.stdin); print(len(d),'entry')"
    1 entry

    curl -sS -o served.json https://flowfin.dev/manifest.json
    git show origin/main:docs/manifest.json | cmp - served.json && echo identical
    identical

All three run 2026-08-23 against the tree at 514c771, which is the commit that
carried the generated catalogue into `docs/`.

The third request is the one that decides it rather than the first. A holding
page, a redirect and a rewritten error page all answer 200, and a Jellyfin server
renders each of them the same way it renders an empty repository, so a status
code alone records that something is at the address rather than that the
catalogue is. What is written down here is that the bytes served are the bytes
this tree holds.

What was read is one entry and one version, which is the scope #65 settled, and
the address is now recorded in `internal/address.Answered`. That does not make
the catalogue complete, and nothing here says it is: the ten plugins that have
published nothing and the declaration switched off for the first publication are
`decisions/first-release.md`, and they are absent by decision rather than by
failure.

Two tracked files printed the address, and #35 removed both. What follows is the
reading taken that day rather than the state of the tree:

    grep -rno 'https://[a-zA-Z0-9./_-]*manifest.json' -- README.md docs/ ; echo "exit=$?"
    exit=1

Run 2026-08-08. Nothing was printed and the grep exited 1. What the
operator-facing files said instead was that installation is not available yet,
which was what an address answering 404 left them able to say.

The same command answers with five sites now, and those five are the operator
instruction:

    grep -rno 'https://[a-zA-Z0-9./_-]*manifest.json' -- README.md docs/ ; echo "exit=$?"
    README.md:17:https://flowfin.dev/manifest.json
    README.md:54:https://flowfin.dev/manifest.json
    README.md:63:https://flowfin.dev/manifest.json
    README.md:73:https://flowfin.dev/manifest.json
    docs/index.html:31:https://flowfin.dev/manifest.json
    exit=0

Run 2026-08-23 against the tree at 1f88020. The first is the address in the
three-step instruction, the three below it are inside the commands an operator
runs when the repository looks empty, and the fifth is the same address on the
served page. Printing them is what the entry in `internal/address.Answered` made
permissible, and it is written rather than still to be decided.

The 404 pair stays here because this is where the address is argued, and a
superseded reading is worth more in place than deleted: it is the pair that
decided the shape of the rule, and a reader who meets only the reading that
succeeded cannot see why a request rather than a DNS lookup is what the rule
asks for. Recording a 404 was never the same act as printing an install
instruction, which is why holding it here refused nothing.

`Gate: install-address-is-answered` is what refuses the next file to print one.
It reads every tracked file, recognises an address on the host `docs/CNAME`
declares whose last path segment ends in `manifest.json`, and refuses it unless
that exact address is recorded in `internal/address.Answered` as having been read
and found to answer. That list holds one address, so the leg admits that one and
goes on refusing every other, which is the sentence above with a machine behind
it. It matched nothing for as long as the list was empty, and what it refuses is
narrower now rather than gone.

Two things sit outside it, each for its own reason. `decisions/` is not read,
because this directory is where an address is argued and therefore where the
request showing that it does NOT answer is recorded; the paragraph above is that
case and three decision files hold one. An address on somebody else's host is not
read either: this tree quotes the Jellyfin project's own catalogue as the
measurement two decisions were derived from, and refusing that would be refusing
evidence for being evidence. Where a third party's address rots, that is a link,
and `internal/links` holds it under the harness.

Adding an entry to that list is the act this rule turns on, and it costs the
request, its output written here, and the list changed in the same commit. That
price was paid on 2026-08-23 in the section above. What keeps the entry honest
afterwards is the harness check
`TestEveryRecordedInstallAddressStillAnswers`, which re-reads every recorded
address and refuses one that has stopped answering, or that answers with
something a server cannot read as a catalogue. An address that answered once and
stopped is silent on every server that already has it.

That check returned early for as long as the list was empty, and it says so on
its own output rather than passing quietly. With an entry it makes the request,
so the day the entry landed is the day the check began reading anything at all.

The operator instruction that prints the address was the other half of #34. It is
written, the five sites above are it, and the list stopped standing in front of
it on the day the entry landed.

## What this costs

Publishing nothing until the address answered meant the project was not
installable in the meantime, which was the state it was in anyway. That half of
the cost is spent. What is left is the half this rule was always going to hand
forward: the address is published, so it cannot be tidied, consolidated or moved
to a shorter name, and a lapsed renewal is a silent outage for every installation
rather than an error anybody sees. Both obligations start on the day the entry
lands rather than on the day somebody first pastes the address.
