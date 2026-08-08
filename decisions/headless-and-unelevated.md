# Every test in the gate runs headless and unelevated

The rule: everything the merge gate runs completes on a clean runner with no
display server, no administrator rights, no privileged port, no network reachable
outside the runner, and nothing installed beyond the toolchain
`decisions/means.md` names.

It is a birth requirement rather than something to arrive at, because the cost of
holding it is nearly nothing before the first test exists and is a rewrite
afterwards. A suite that has assumed a desktop for six months does not get talked
out of it.

## What the rule refuses, concretely

A test that opens a window, needs a compositor, or renders a page in a real
browser.

A test that asks for administrator or root, registers a service or a scheduled
task, binds a port below 1024, writes outside its own temporary directory, or
changes a machine-wide setting such as a certificate trust store. Elevation is
singled out because its failure mode is not a red test. On a machine with a person
sitting at it, it is a consent prompt that takes the screen away from whoever was
working, and a test that does that once is a test people learn to skip.

A test that reaches the public network. That includes fetching the published
manifest, resolving a name, and calling the release API of a repository this
project reads. Those are the interesting checks and they are not gate checks: a
gate that goes red because somebody else's service is having an afternoon teaches
everybody that red means nothing.

A test that needs a Jellyfin server, a container runtime, or any second program to
be running.

A test that depends on the clock beyond its own control, on a locale, or on a
line-ending setting. The last one is the reason `.gitattributes` pins what it
pins, and #23 widens it.

## The harness for everything else

Those checks are worth running. They are the only ones that answer whether the
published address actually serves the file, whether the site renders, and whether
a plugin installs into a real server. They do not belong in the gate, and they are
not quietly dropped either. They go into a separate harness, and the harness is
named for what it needs rather than for what it tests, so that somebody reading a
list of jobs can tell which ones would go red because the internet was down
without opening a file.

The names are requirements:

`needs-network` for anything that makes a request off the runner. Fetching the
manifest from its published address, checking that a redirect or a certificate is
what it should be, reading the release API against the world rather than against a
fixture.

`needs-browser` for anything that renders the site. Measuring the numbers in the
speed budget, checking the focus behaviour, running an accessibility pass against
a real render.

`needs-jellyfin` for anything that talks to a server. Adding the repository
address, listing the catalogue, installing a plugin and watching it load.

One requirement per name, and a check that needs two carries both, because a name
that covers a set of requirements stops answering the question it exists for.
Building the harness is #21, and the exact spelling of the tags and job names is
its to fix; what this file fixes is that the name states the requirement.

## The gate never depends on it

A red run in any of those harnesses does not block a merge and is not a required
check. They are triggered deliberately rather than on every pull request.

The gate produces a complete verdict with the harness entirely absent. Not a
partial verdict, not a verdict with a hole in it: the gate's legs are the ones that
run everywhere, and none of them is waiting on a harness that may not have run.

The counterpart, and it is the half that gets dropped: a harness that did not run
says so, in the same place a person reads the gate's result, along with what asking
for it would cost. An absent result is never rendered as a clean one. A green gate
next to a silent harness reads as everything passing, and that reading is wrong in
exactly the cases the harness exists for.

Where a check cannot run at all on a given machine, for example one that would
raise an elevation prompt, it is skipped and the skip is disclosed with its reason.
It is not worked around, and the disclosure does not get rewritten later into a
statement that it passed.

## The check that refuses a gate test reaching outside the rule

`gate-tests-reach-nothing`, a named leg of the gate, in `internal/reach`.

It refuses a test compiled into the gate that reaches for a display, an elevated
operation, the network, a second program, or a port below 1024.

What it judges is the reach rather than the result. A test that attempts one of
those things fails on a bare runner because the thing is absent and passes on a
desktop that has it, so a check reading outcomes is loudest where the rule is
already being kept and silent where it is being broken. Reading the source gives
the same verdict on both machines, and it is the only shape under which elevation
can be judged at all: attempting an elevated call to see what happens is the
consent prompt this rule exists to prevent.

The scope is drawn by the build constraint. A test file carrying a tag that names
a requirement is the harness's and is spared, because the harness contains exactly
the tests this check refuses and does so by design. `internal/reach` requires the
prefix of those tags and leaves their full spelling to #21.

Two limits, because a floor stated as a proof is worse than no floor. An address
assembled from parts at run time is not visible to a reader of the source and is
not refused. And of the things this file lists, three are outside what the check
reads: a write outside the test's own temporary directory, a change to a
machine-wide setting, and a dependence on the clock or the locale. Those are held
by review today and by nothing else.

## What this costs

The most valuable checks this project will have are the ones outside the gate, and
this rule guarantees that none of them blocks a bad merge. That is the trade, and
it is taken deliberately: a gate that blocks reliably on a smaller set is worth
more than one that blocks unreliably on a larger set, because the second one gets
overridden and then ignored.

What keeps the trade honest is the disclosure above. The moment a skipped harness
reads as a pass, this rule has bought nothing and cost the coverage.
