# What the design system binds, and what missing a number means

The design system is published as a promise that a set of clients is held to it. A
promise with nothing behind it is a preference. Before any client is measured, the
document has to be split into the part a machine compares and the part a person
judges, because the two fail differently and pretending a machine makes the second
kind of call is worse than admitting it does not.

## The values a machine compares

These are numbers and strings. A client either carries them or it does not, and the
comparison needs no judgement.

The surface colours, in both schemes: `ground`, `raise`, `raise2`, `line`, `ink`,
`ink2`, `ink3`, and the lift shadow. The accent, `sel`, and its soft form.

The focus colour for each colour-vision preset, in both schemes. There are ten
values, not five, and this is the first thing the split turns up. The published
page's table lists one colour per preset and the page's own stylesheet carries two:

    grep -n 'data-cvd="protan"' docs/design-system.html
    33::root[data-cvd="protan"]{ --sel:#46B8E8; --sel-soft:rgba(70,184,232,.16); }
    34::root[data-cvd="protan"][data-theme="light"]{ --sel:#0E7FA8; --sel-soft:rgba(14,127,168,.13); }
    42:  :root[data-cvd="protan"]:not([data-theme="dark"]){ --sel:#0E7FA8; --sel-soft:rgba(14,127,168,.13); }

Run 2026-08-08 against the tree at 3e59901. `#46B8E8` is the value the table
prints, and it is the dark one. The light one is `#0E7FA8`, written twice because
the page supports both an explicit theme and the one the system asks for. A client
implementing the table gets the light scheme wrong for every preset, and nothing in
the document says so. This is what extracting the values into one file is for, and
it is #39.

The focus ring width, and the one place it changes. `--ring` is `3px`, and the
achromatopsia preset raises it to `4px`. The prose says `dp` and the stylesheet
says `px`, which for a web page are the same number and for a native client are
not. The token file has to state the unit once rather than leave each client to
read it off a stylesheet.

The type scale, as size and weight per role, at both viewing distances. Television,
read from three metres: display 56/700, shelf 32/640, tile 24/540, meta 20/400.
Telephone, read from thirty-five centimetres: display 28/700, shelf 19/640, tile
14/540, meta 12/400.

The corner radii, `--r` and `--rs`, and the content width `--maxw`.

The font stacks, as ordered lists of family names. These are values even though the
rule behind them is a judgement: the rule is "no bundled typeface", and the list is
what the platform's own font is called on each platform.

The five numbers in the speed budget. Focus change under 80 ms. Zero dropped frames
at 60 fps. First usable content under 1.2 s from cold. No layout shift. Playback
start under 2 s over a local network.

## The rules a reviewer applies

These are judgements. A machine can catch some violations of some of them and none
of them can be decided by a machine, and a check that claimed to would be a check
that passes the interesting cases.

Colour marks focus and nothing else. No coloured buttons, no coloured headings, no
red for error and no green for success. A tool can find a hex value outside the
token set; it cannot tell a decorative use of an approved colour from a meaningful
one.

No state depends on hue alone. Every state that colour marks also carries shape or
text. This is the rule that makes the per-preset accent safe to change, and whether
a given state carries a second signal is a reading of the interface.

No bundled typeface. Exceptions need a reason stronger than looking better. A
build can be searched for font files; whether a given exception is justified is the
judgement.

Missing artwork is a design case and not an error case. Every view has to look
deliberate against a library with no images at all.

Nothing waits visibly. No spinner inside a tile, no reflow, no element arriving
late into a space that was not held for it. The speed budget measures the outcome
of this rule and does not decide whether a particular animation reads as waiting.

No decoration that costs frame rate. No blur over scrolling surfaces, no shadows on
moving elements, no gradients over artwork. Where a decoration and a budget number
conflict, the number wins.

Television and telephone share colour, type, motion and image handling, and do not
share a layout. Density, arrangement and navigation belong to the platform.

## Where each kind lives

The values live in one machine-readable file, and it becomes the authority for
them. The published page then renders from that file or is checked against it, so
the disagreement found above cannot be reintroduced by editing one of the two. The
file is `docs/design-tokens.json`, which #39 extracted, and the check that holds
the page to it is #40. Until that check exists, the two can still disagree; what
the file changes today is that the disagreement has a side that is right.

The rules stay prose, on the published page, and they are applied by a person
reviewing a client. They are not moved into the token file in a weakened form that
a machine can pass. A rule that reads "no more than three accent colours" would be
checkable and would not be this system's rule.

Neither kind lives in a client. A client that copies a number is a client that will
still carry it after the number changes.

## What missing a number means

Stated before the first measurement, because a consequence negotiated after a
failed measurement is not a consequence.

A client that misses a budget number does not ship a release. The number is a
release condition, not a target and not a score. Five numbers, all five held, or
the build is not published as a release of that client.

Two things are true alongside it, and both are there to keep the rule from being
quietly abandoned the first time it bites.

A missed number is recorded with the measurement that produced it, on the hardware
it was measured on. "Slow on a 2015 television" and "slow" are different findings,
and the second one gets argued about while the first one gets fixed. What the
method is for each of the five numbers is #41, and until that exists a measurement
is not comparable between two clients and this rule cannot be applied fairly.

A number may be changed, and it may not be waived. Changing it is a change to this
system, argued in the open, applying to every client from then on. Waiving it for
one release is how a budget becomes a preference, and it is the specific failure
this file exists to prevent. If 80 ms turns out to be the wrong number, the repair
is to move it and say why, for everybody.

A client that has not been measured has not met the budget. Not measured and met
are different states, and a release that reports the first as the second is worse
than one that reports a miss.

## What refuses a violation

Nothing today. Every sentence above is prose.

The parts that can become mechanisms are named: #39 extracts the values, #40 checks
the published page against them, #41 writes the method that makes a measurement
comparable, and #42 states how a client declares conformance. Until those land, a
client's conformance is a claim it makes about itself, and this file does not turn
that claim into evidence.

The rules in the second section stay unrefusable after all four land, and that is
their terminal state rather than a gap waiting on a check.
