# The published site is in English, and every page says so

The site and the design system were written in German. The tracker, the source,
the decision files and the documents an operator reads are English. That was an
open question rather than a defect, and it is entry 3 of #1.

## What was decided

The published pages are in English. Decided by the maintainer on 2026-08-09 in
entry 3 of #1, together with the reasoning: a Jellyfin plugin catalogue is read
mostly by people who arrive in English, and everything else on the path to an
operator was already planned in that language, so a German page in front of it
would have been the only exception in the whole route.

The price is named there and is paid once: pages that were good as they were get
rewritten.

Two languages with one named as the source was the fuller answer and the only one
in which two versions can drift. On a page that states a speed budget, a number
that has drifted is worse than a language fewer.

## What follows from it

`docs/index.html` and `docs/design-system.html` are English. The German text is
not carried anywhere alongside them: keeping it would be the two-version case the
decision refused, and its history is in git either way.

Every served page declares the language it is written in, on the `html` element.
That is a separate obligation from the one above and it is not a formatting
detail. A screen reader chooses a voice and a pronunciation dictionary from that
attribute, and with nothing to read it guesses, usually from the reader's own
locale. Neither page declared one before this landed, and both were written in a
language the guess would rarely have matched.

    grep -c '<html' docs/index.html docs/design-system.html
    docs/index.html:0
    docs/design-system.html:0

Run 2026-08-09 against `main` at f965a7d, before the change. Both pages let the
parser supply the element, which is valid HTML and carries no attribute anybody
wrote.

## What refuses a departure

`Gate: site-declares-its-language`, over `docs/`. It refuses a served page with
no `html` element, one whose `html` element declares no `lang`, and one whose
`lang` names a language the site does not publish in. The third is the one a
presence check alone would miss: a page left behind in German would announce
itself correctly and still be the wrong page.

A tag that is more specific about the same language passes, so `en-GB` is not
refused. Pushing whoever writes the next page towards the least specific tag
available would be the wrong direction for a rule that exists to help a reader.

The language the check compares against is a constant in `internal/lang`. This
file is where the reasoning lives and that constant is what a machine reads;
changing one without the other is a departure this document cannot catch and the
review is where it would be.

## What this does not settle

Whether the design system is ever offered in a second language. If it is, the
decision above is the one that has to be reopened, because it refused the
two-version shape rather than merely not choosing it, and the reason it gave was
drift on a page carrying numbers.
