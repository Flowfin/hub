# Flowfin

Plugins and clients for [Jellyfin](https://jellyfin.org). This repository holds
the plugin manifest, the design system, and the numbers the clients are held to.

Flowfin is not affiliated with the Jellyfin project.

## Installing the plugins

Three steps.

1. In your server's **Dashboard**, open **Plugins** and then the
   **Repositories** tab.
2. Add a repository. **Repository Name** is yours to choose and is only a label.
   **Repository URL** is this, exactly:

       https://flowfin.dev/manifest.json

   Jellyfin warns before it accepts a repository that is not its own, and the
   warning is worth reading rather than clicking past: a third-party repository
   may hold unstable or malicious code and may change at any time. That is true
   of this one as much as of any other. What can be said for it is that the file
   is [generated from the plugin releases](decisions/manifest-is-generated.md)
   rather than written by hand, and that every entry names the repository its
   archive was downloaded from, so anything offered here can be read at its
   source before it is installed.
3. Go back to the plugin list. What this repository offers appears there beside
   whatever the server already had.

What it offers today is one plugin, `Requests`, and that plugin's own entry says
it is under development and adds nothing a user can see yet. One entry is what
this address serves rather than a catalogue that failed to load, and telling
those two apart is the next section.

The address itself is served from a domain this project holds rather than from a
`github.io` name, so the hosting and even the account behind it can change later
without breaking a single installation. An address that moves breaks every
install silently: no error, the plugins simply never update again.

### When the list is empty

A Jellyfin server says nothing when a plugin repository does not work. It shows
an empty list, and it shows the same empty list for three different problems.
None of the three produces an error message anywhere, so the list on its own says
nothing about which one it is.

Run these on the machine the server runs on rather than on the machine you are
reading this from. A server behind a firewall, or on a network with a resolver of
its own, can fail all three while your laptop succeeds, and nothing in the
interface shows that difference.

**Whether the address answers at all.**

    curl -sS -o /dev/null -w "%{http_code}\n" https://flowfin.dev/manifest.json
    200

`200` is the answer to want. Any other number, or an error from curl instead of a
number, means that machine cannot reach the address, and the problem is between
it and the network rather than in Jellyfin or in the file.

**Whether what comes back is a catalogue.**

    curl -sS https://flowfin.dev/manifest.json | python3 -m json.tool > /dev/null ; echo "exit=$?"
    exit=0

Silence and a zero exit mean it parsed. An error means something answered with
bytes no server can read as a catalogue, which is what a captive portal, a
filtering proxy and a cached error page each produce with a `200` in front of
them.

**Whether anything in it fits your server.**

    curl -sS https://flowfin.dev/manifest.json \
      | python3 -c "import json,sys; [print(p['name'], v['version'], 'needs', v['targetAbi'], 'or newer') for p in json.load(sys.stdin) for v in p['versions']]"
    Requests 0.1.0.0 needs 10.11.0.0 or newer

Every version entry names the oldest Jellyfin it will install into, and your
server's own version is on its dashboard. A server older than every line that
command prints sees an empty list, and that is the catalogue working rather than
failing: the entries are there and none of them fits.

Check them in that order. The first is the one that is wrong most often and the
cheapest to answer, and a failure at any of the three makes the two after it
unreadable.

### How long a new version takes to appear

The catalogue is rebuilt from the plugin releases once a day, and a rebuild can
also be asked for on the day a release lands. That is the half of the answer with
a number on it.

The other half has none, and inventing one would be promising something this
project does not control: the rebuilt file still has to reach the address, and
the host that serves these pages publishes on its own timing. So the honest
answer is a day for a release to be picked up, plus a publication that is usually
short and is not guaranteed. A version you know exists and that has not appeared
a day later is worth asking about rather than waiting out.

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

Two licences over one tree.

The code is under the GNU Affero General Public License, version 3 or any later
version. The full text is in [LICENSE](LICENSE), and the "or any later version"
is the option the licence's own application notice offers rather than an addition
to it.

The design system and the words on the published pages are under CC-BY-4.0,
attribution required, so that a project adopting the design system does not
inherit a code licence with it.

### What it covers

The boundary between the two does not run between files. Both served pages hold
the page's own stylesheet, its markup and its script in the same bytes as the
words a reader reads, so a table of paths would be wrong about part of every file
it named. [`decisions/licensing.md`](decisions/licensing.md) is the declaration,
part by part, and it is the authority rather than this section: it says which
bytes are under which licence, what a reader who copies a whole file takes, and
what the tree does not carry.

The short of it. The generator, the site's own markup, stylesheets and script,
the documents and the declared source set are AGPL-3.0-or-later. The words on the
published pages, and the design system including `docs/design-tokens.json`, are
CC-BY-4.0.

Adopting the design system means following what it says rather than copying the
file, which no licence reaches. Copying the file, or a client built from it, is a
derivative and does, and CC-BY-4.0 is the licence that reaches it.

The repository detector reads the root file and only the root file, so it reports
one licence for a tree that grants under two:

    gh api repos/Flowfin/hub --jq '.license.spdx_id'
    AGPL-3.0

Run 2026-08-25. Nothing on the platform side can be made to state both, which is
why the declaration is a file in the tree.

The plugin repositories this catalogue lists are separate repositories under
their own terms. A catalogue listing something is not a statement about its
licence.

See [NOTICE.md](NOTICE.md) for the intended-use notice.
