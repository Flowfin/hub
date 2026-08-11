# Flowfin

Plugins and clients for [Jellyfin](https://jellyfin.org). This repository holds
the plugin manifest, the design system, and the numbers the clients are held to.

Flowfin is not affiliated with the Jellyfin project.

## Installing the plugins

Not yet possible. No manifest is published, so there is nothing to paste into a
Jellyfin server. `decisions/manifest-address.md` is the rule an address has to
meet before it may be printed in this tree, and it carries the request showing
what the intended one answers today.

The address is settled, name included, and it is served from a domain this
organisation holds rather than from a `github.io` address, so the hosting and
even the account behind it can change later without breaking a single
installation. A manifest URL that moves breaks every install silently: no error,
the plugins simply never update again. That was entry 2 of #1 and it is answered.

What is missing is the file, not the name. The domain answers and the manifest
does not, so there is still nothing worth pasting, and this section says so until
there is.

## What is here

| Path | What it is |
|---|---|
| `docs/` | The published site: manifest, plugin list, design system |
| `docs/design-system.html` | Colour, type, tiles, focus, and the speed budget |
| `manifest/` | The generator that builds `manifest.json` from the plugin releases |

The manifest is **generated**, never edited by hand. It is built from the release
artifacts of each plugin repository, and a check refuses a build that does not
list a current release for every Jellyfin version line it claims to support. A
manifest maintained by memory is a manifest that is wrong the first time somebody
is busy.

## The clients

Native clients, one per platform, sharing a core rather than a codebase. The
design system in `docs/` is what makes eleven clients look like one product.

Two things are settled there and worth repeating here, because they are the parts
that get quietly dropped first:

**Nothing waits visibly.** No spinner inside a tile, no layout shift when an image
arrives, no reflow. The speed budget is written as numbers that a build can miss -
focus change under 80 ms, no dropped frames at 60 fps across 200 tiles, first
usable tile under 1.2 s from a cold start.

**Missing artwork is a design case, not an error case.** Netflix curates its
imagery; a personal library has whatever the metadata provider happened to hold. A
tile wall with three grey holes looks worse than a list, so the empty tile has a
shape of its own.

## Colour and colour vision

The interface is neutral. Content is the only colour, and one accent marks one
thing: what has focus.

That accent is configurable, with presets per deficiency rather than a single
"accessible" mode - because which hue works depends on which cone type is missing,
and the hue that serves the two common forms best is the wrong one for the rare
third. Removing colour entirely is correct only for achromatopsia; for everyone
else it discards a signal their eyes could have read.

No state depends on hue alone, whatever the preset.

## What a server sends when it uses this catalogue

Nothing about the server, the people using it, or what is in their libraries.

Giving a Jellyfin server a plugin repository address means the server fetches a
file from that address, on a schedule the server decides. The request carries
what any request carries, the address it came from and whatever the network puts
on it, and the file that comes back is the same file for every server. There is
no value in it that differs between operators, so nothing in the exchange says
which server asked or what it holds. `decisions/data-posture.md` is the rule and
lists what it rules out, including the identifier per install that is the usual
shape this goes wrong in.

Somebody sees the requests, and it is worth saying who rather than implying
nobody does. The site and the file are served by GitHub Pages, from the `docs`
directory of this repository:

    gh api repos/Flowfin/hub/pages --jq '{cname, source: .source.path, https_enforced}'
    {"cname":"flowfin.dev","https_enforced":true,"source":"/docs"}

Run 2026-08-09. Serving a file means receiving a request, so GitHub receives
every one of them, and whatever it keeps is GitHub's under GitHub's own terms.
This project has no access to those records and adds nothing on top of them: no
analytics, no counter of its own, no log. An operator who has to answer for
their deployment can take that as the whole of it, because there is no second
party here to find out about later.

The plugins listed in the catalogue are separate projects in separate
repositories. What each one does with data is its own to describe, and listing
something is not a statement about its behaviour any more than it is about its
licence. The catalogue entry names the repository; read it before installing
from it.

Personal data stays on the operator's own host unless the operator sends it
somewhere on purpose, and nothing in this catalogue can start that. A plugin
installed from it can, and it looks like something configured rather than
something that happens: another server's address entered in a setting, an
invitation issued to somebody outside the household, a synchronisation target
pointed at a machine that is not yours. If none of that has been set up, nothing
has left the host. Whether a particular plugin offers any of it is that plugin's
own documentation.

## License

The GNU Affero General Public License, version 3 or any later version. The full
text is in [LICENSE](LICENSE), and the "or any later version" is the option the
licence's own application notice offers rather than an addition to it.

The detector reads it:

    gh api repos/Flowfin/hub --jq '.license.spdx_id'
    AGPL-3.0

Run 2026-08-09.

### What it covers

Every path in this repository, with no exception and no second licence anywhere
in the tree. That is worth saying out loud because this tree holds three kinds of
thing with three different audiences, and a reader who knows that will look for a
boundary:

| Path | What it is | Under |
|---|---|---|
| `manifest/`, `internal/`, `main.go` | The generator, which is a program | AGPL-3.0-or-later |
| `docs/` | The published site, including the design system | AGPL-3.0-or-later |
| `decisions/`, `*.md` | The documents | AGPL-3.0-or-later |
| `sources/` | The declared source set, which is data | AGPL-3.0-or-later |

The design system is prose and numbers other projects are invited to adopt, and
adopting them means following what they say rather than copying the file, which
the licence does not reach. Copying the file, or a client built from it, is a
derivative and does.

Whether the published pages and the design system should carry different terms
from the generator is entry 1 of #1 and is open. A single `LICENSE` at the root
reads as covering the whole tree, which is what the table above states, so a split
would be a change to this rather than a reading of it.

The plugin repositories this catalogue lists are separate repositories under
their own terms. A catalogue listing something is not a statement about its
licence.

See [NOTICE.md](NOTICE.md) for the intended-use notice.
