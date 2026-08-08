# Flowfin

Plugins and clients for [Jellyfin](https://jellyfin.org). This repository holds
the plugin manifest, the design system, and the numbers the clients are held to.

Flowfin is not affiliated with the Jellyfin project.

## Installing the plugins

Not yet possible. No manifest is published, so there is nothing to paste into a
Jellyfin server. `decisions/manifest-address.md` is the rule an address has to
meet before it may be printed in this tree, and it carries the request showing
what the intended one answers today.

The shape of that address is settled even though the name is not. It will be
served from a domain this organisation controls rather than from a `github.io`
address, so the organisation, the hosting and even the name can change later
without breaking a single installation. A manifest URL that moves breaks every
install silently: no error, the plugins simply never update again. Which name it
is, and who registers it, is entry 2 of #1.

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
arrives, no reflow. The speed budget is written as numbers that a build can miss —
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
"accessible" mode — because which hue works depends on which cone type is missing,
and the hue that serves the two common forms best is the wrong one for the rare
third. Removing colour entirely is correct only for achromatopsia; for everyone
else it discards a signal their eyes could have read.

No state depends on hue alone, whatever the preset.

## Licence and use

See [NOTICE.md](NOTICE.md).
