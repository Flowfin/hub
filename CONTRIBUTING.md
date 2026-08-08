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

Nothing. There are no test files, so `go test ./...` prints that and exits zero:

    go test ./...
    ?       flowfin.dev/hub/manifest        [no test files]

A green from it is not evidence about anything in the tree, and reading it as one
is the failure #19 exists to repair.

The single entry point that runs build, test and format as named legs lands in
#18. When it does, this section names its command.

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

They are supply-chain and hygiene checks. None of them builds or tests anything,
because there is nothing yet to build or test. That is worth knowing before you
read a green check as the tree being verified.

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

Some things are deliberately not settled, including the license this repository
carries. Those are in #1 and are not decided in a pull request.

## Style

English in tracked files, apart from the published pages under `docs/`, whose
language is an open question in #1.

Commit messages say what changed and what failure it prevents. Where a correction
is being made, they say what was wrong and how it was found.
