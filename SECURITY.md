# Security policy

This repository publishes a plugin catalogue. A Jellyfin server reads it and
takes download addresses and checksums from it, so a problem here can reach every
server that has the address configured. Reports are handled ahead of other work.

## Reporting a vulnerability

Report privately, through
[private vulnerability reporting](https://github.com/Flowfin/hub/security/advisories/new),
which is "Report a vulnerability" on the Security tab of this repository. It is
enabled and it reaches me and nobody else.

Please do not open a public issue for something exploitable. A public issue is
opened once a fix is out.

If you used an automated tool to find it, please confirm it by hand before
reporting.

## What to expect

An initial response within a few days. A fix released as soon as it is ready
rather than batched behind other work. Coordinated disclosure, meaning please
give the fix time to reach servers before publishing.

This is a volunteer project. The sentences above describe intent applied
consistently, and they are not a service level agreement.

## What is in scope

The manifest generator, once it exists, and anything it publishes. In particular
a manifest entry whose download address or checksum could be made to point at
something other than the release it names.

The published site under `docs/`, including anything that would make a visitor's
browser fetch from a third party.

The workflow files in `.github/workflows`, including anything that would let a
pull request from outside run with more permission than it should have.

## What is out of scope

Vulnerabilities in Jellyfin itself. Those go to the Jellyfin project.

Vulnerabilities in a plugin the catalogue lists. Those go to that plugin's own
repository, which the catalogue entry names. A defect in how this catalogue
describes a plugin is in scope here; a defect inside the plugin is not.

Reports that a checksum in the manifest is MD5. That is the digest the Jellyfin
server computes and compares, so it is the only value that field can carry, and
`decisions/artifact-checksum-pairing.md` records what it does and does not prove.

## What this project holds about you

Nothing. There is no analytics, no third-party asset on the site, and no
identifier in the manifest. `decisions/data-posture.md` is the rule and what it
rules out. A report sent through the route above is held by GitHub under
GitHub's own terms, which is outside anything this project controls and is worth
knowing before you write one.

The same is true of every other request, and the provider is named rather than
left as "the host". These pages and the manifest are served by GitHub Pages, so
GitHub receives each request for them; whatever it keeps is GitHub's under its
own terms, and this project neither reads it nor adds a record of its own beside
it. What a server sending one of those requests discloses, and what it does not,
is in the readme under `## What a server sends when it uses this catalogue`.

The plugins the catalogue lists are separate projects, and what each one does
with data is its own to describe. Listing one is not a statement about its
behaviour, so a question about what a plugin sends is that plugin's to answer
rather than this catalogue's. That is a different boundary from the one under
`## What is out of scope` above, which decides where a report goes rather than
who answers for a behaviour.

Personal data stays on the operator's own host unless the operator configures a
plugin to send it somewhere, which is something set up deliberately and never a
default. What that looks like, so that a reader can tell whether they have done
it: another server's address entered in a setting, an invitation issued to
somebody outside the household, a synchronisation target pointed at a machine
that is not yours. If none of that has been configured, nothing has left the
host, and whether a particular plugin offers any of it is that plugin's own
documentation.
