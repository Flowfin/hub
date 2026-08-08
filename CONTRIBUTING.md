# Contributing

Thank you for looking. This repository holds the plugin manifest generator, the
published site, and the design system the clients are held to.

## Building and testing

    go build ./...
    go test ./...

Both need the Go toolchain and nothing else installed. `go.mod` names the
language floor and is the authority for it rather than this sentence.
`decisions/means.md` is why Go, and why the test runner and the formatter are the
ones that arrive with it.

The site under `docs/` still opens in a browser with no build step.

### Formatting

    gofmt -w .

That is the whole format command, and on a tree that is already formatted it
changes nothing. gofmt decides Go. Everything else is decided by three
properties `.editorconfig` states and `internal/format` refuses: a final
newline, no trailing whitespace, and no tab indent outside Go.

    go test ./internal/format -count=1

is what refuses them, and the Formatting leg of the gate runs both. There is no
formatter for the HTML, the YAML or the prose, because one would be a runtime
this tree does not carry, and `.editorconfig` is what an editor reads instead.

Line endings do not decide the verdict, and the two halves get there
differently. `internal/format` normalises before judging, so it answers the same
on either checkout, and a test holds that property rather than a sentence
claiming it. gofmt does not normalise: it lists a Go file whose working copy has
CRLF, on any platform. What keeps that from being a Windows-only red is
`.gitattributes`, which pins every tracked file to LF in the working copy as
well as in the object store, so a fresh clone is LF whatever `core.autocrlf`
says locally.

If gofmt lists a file you have not touched, that is the symptom: the working
copy predates `.gitattributes`. Deleting the file and checking it out again
fixes it, and nothing about the file's content was wrong.

### The dependency set is empty, and empty is not unlocked

There is no `go.sum` in the tree, because nothing is required yet. A build does
not quietly add one. The toolchain resolves imports in read-only mode by default,
so an import the module does not already require is a build failure rather than a
new line in the lock:

    printf 'package manifest\n\nimport _ "golang.org/x/text/language"\n' > manifest/x.go
    go build ./... ; echo "exit=$?"
    manifest\x.go:3:8: no required module provides package golang.org/x/text/language; to add it:
            go get golang.org/x/text/language
    exit=1

Run on Windows, which is why the compiler prints the path with a backslash;
the separator is the platform's and not part of the message. `go.mod` is
byte-identical after that build, which is the half worth checking:

    git hash-object go.mod

prints the same object id before and after.

A dependency therefore arrives as a commit somebody made and reviewed, or it does
not arrive. `go.sum` lands in the same commit as the first requirement.

### What the test command proves today

The manifest's byte format, and nothing beyond it:

    go test ./... ; echo "exit=$?"
    ok      flowfin.dev/hub/manifest        0.482s
    exit=0

The suite decodes `manifest/testdata/golden-manifest.json`, encodes it again and
compares the bytes, checks that an ampersand and angle brackets survive encoding
as themselves, and checks that no plugins are written as an empty array rather
than as null. Each of those has been watched failing: the escaping test and the
specimen test both red when one call is deleted from `Encode`, and the specimen
test reds on one space removed from the fixture.

What a green does not say is that a generated entry is right. The specimen round
trips, so an edit to a checksum or a URL inside it passes. Refusing a wrong entry
is #25 and #27, against the release it came from.

### The gate

    go run . gate

That is the whole gate, and it is one command rather than a list to remember.
Its legs run in order, they stop at the first failure, and the run ends by saying
how many of them it examined, so a run that covered two of three cannot be read
as one that covered three and found nothing.

The legs are build, test and format, which is what `decisions/means.md` settles
them as. Which legs exist is printed rather than restated here, because a list in
this file drifts against the one the command runs:

    go run .

names them. One leg at a time is `go run . gate format`, and that run says which
legs it was not asked for, so a single-leg run cannot be mistaken for the gate.

`.github/workflows/gate.yml` is one job per leg, and each job runs the same
command with the leg's name after it. Nothing is decided in the workflow file
that the command would not decide in your shell. The job names carry a prefix,
because `build`, `deploy` and `report-build-status` are already taken on `main`
by the Pages deployment, which has no file in this tree; `internal/gate` holds
the prefix, and the suite refuses a leg with no job and a job with no leg.

On a clone where the working copy has CRLF line endings, the format leg lists Go
files you have not touched. The content is not wrong: gofmt does not normalise
before judging, and only `*.json` is pinned to LF today. #23 is where that pin
widens to the rest of the tree.

## Where things live

`manifest/` is the generator: the manifest document, the encoder that fixes its
byte format, and the specimen of that format under `manifest/testdata/`.

`docs/` is everything published at the site's address, which is why the design
system lives there too even though it has a different audience and a different
cadence from the landing page. Moving it out would change a URL that is already
public. `docs/design-system.html` is the design system and `docs/index.html` is
the site.

`decisions/` is what has been settled and why, and it is not published.

The separation that matters most is between the generator and everything
published, because only the first one is a program. Whether the design system
ends up under a different licence from the generator is entry 1 of #1, and the
boundary that answer would fall on is `docs/design-system.html` plus the token
file #39 extracts from it.

## What runs today

The gate is the workflows in `.github/workflows`, and what each one does is in
the comment at the top of its own file:

    ls .github/workflows

Most of them are supply-chain and hygiene checks that build and test nothing.
The Formatting leg is the exception: it installs the toolchain and runs part of
the suite. Nothing else in the gate compiles this repository yet, which is worth
knowing before you read a green check as the tree being verified. #18 is where
one entry point runs build, test and format as named legs.

## Sign your commits

Every commit needs a `Signed-off-by` line whose name and address match the
commit's author. The DCO check reads every non-merge commit in a pull request and
reds if one is missing.

    git commit -s

That adds the line for you. To add it to commits you have already made:

    git rebase --signoff <base>

What you are certifying by adding it is in [DCO](DCO), which is the Developer
Certificate of Origin 1.1, unmodified. It is short and it is worth reading once,
because the sign-off is a statement about your right to contribute the code, not
a formality.

## Opening a change

Every change starts as an issue and lands as a pull request. Direct pushes to
`main` are refused.

An issue says what is wrong, what the evidence is, and what "done" means. Where
the evidence is a number, it carries the command that produced it, run against
the reference a reader will have rather than against your working copy. That is
the single largest source of wrong claims in a tracker, and it is cheap to avoid.

Where a change makes an assertion about the tree, the pull request body carries
the command and the output that backs it. Where a claim cannot be backed by a
command, write it as a claim rather than as a measurement, and say which it is.

One topic per pull request.

## Guards

A check that has never refused anything has not been shown to work. If your
change adds one, the pull request shows it refusing something: a planted fixture,
the command that runs it, and the red output. A guard that is silently matching
nothing looks exactly like a guard that is passing.

## Decisions

`decisions/` holds what has been settled and why, one file per decision, present
tense. Read the relevant one before arguing with a rule, because most
disagreements about a rule are disagreements with the decision behind it and
those are easier to have directly.

Some things are deliberately not settled. Those are in #1 and are not decided in
a pull request. The license this repository carries is no longer one of them: it
is AGPL-3.0, in [LICENSE](LICENSE), and what remains open in entry 1 of #1 is
whether the published pages and the design system carry different terms.

## Style

English in tracked files, apart from the published pages under `docs/`, whose
language is an open question in #1.

Commit messages say what changed and what failure it prevents. Where a correction
is being made, they say what was wrong and how it was found.
