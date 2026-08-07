# Contributing

Thank you for looking. This repository holds the plugin manifest generator, the
published site, and the design system the clients are held to.

## Building and testing

There is no build command and no test command in this tree today. The tree is
documents, workflow files and two static pages, and `git ls-files` is the
authority for that rather than a list here.

Saying so is more useful than inventing a command, because a contributor who runs
an invented one and gets an error cannot tell whether the repository is broken or
the instruction is. The site under `docs/` opens in a browser with no build step.

The toolchain lands in #17 and the single entry point that runs build, test and
format lands in #18. `decisions/means.md` says which toolchain and why. When #18
lands, this section names its command and this paragraph goes.

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
