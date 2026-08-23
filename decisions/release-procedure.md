# Cutting a release, and the states a release may not be cut in

A release of this repository is not a binary anybody downloads. It is the moment
the catalogue an operator pastes into a Jellyfin server starts answering, or
starts answering with something new, and the thing that ships is a file at an
address rather than an artefact attached to a tag.

That shape decides everything below. There is nothing to install here, so a
release that goes out wrong is not a bad download somebody deletes. It is a
server polling an address on its own schedule, showing what it finds, and
reporting nothing when it finds nothing.

## The order

The catalogue is generated, then placed, then landed, then served, then read
back, and only then written down. Each step is worth naming because the failure
of each one looks like the success of the one before it.

Generated. `go run . publish` reads the declared set in `sources/`, resolves
every declaration against its repository, builds a version entry per publishable
release, and refuses rather than guessing wherever
`decisions/failure-posture.md` says it must.

Placed. The same run writes the bytes to a temporary file beside the target and
renames it over the published one, so a server polling the address in the middle
of a run reads one run's whole output or the previous one's and never a mixture.
`internal/publish` is where that is argued.

Landed. The placed file is a tracked file, so it reaches the address the way
every other change does: a branch, a pull request, the gate, a merge. There is
no route from a generator run to the served directory that skips the merge, and
that gap is deliberate, and it is recorded here so it reads that way.

Served. The site is served from the `docs` directory of the default branch by a
host this repository does not run and cannot make deploy. A merge is not a
publication, and the gap between the two is real time.

Read back. The address is read the way a server reads it, and what comes back is
compared against the release lists it was built from. A publication run that
reported success and wrote nothing is a known failure shape rather than a
hypothetical: the run is green, the release exists, and the file an operator
fetches is the old one.

Written down. An install address may appear in a tracked file only once it has
been read and found to answer with the file it promises. That rule is
`decisions/manifest-address.md`, `internal/address.Answered` is the list that
holds the reading, and the merge gate refuses a printed address that is not on
it.

## What a person does by hand

Naming these is most of the value, because the hand steps are the ones done
differently the second time.

Deciding that a run's verdict calls for a merge, and asking for a run when a
release lands between the scheduled ones. Starting the publication is not the
hand step here. `.github/workflows/publish.yml` declares a daily schedule, a
request trigger, and a concurrency group naming the destination so that two runs
never write one file. What such a run leaves is a verdict and nothing else: its
permissions are read-only, none of its three steps pushes, and its checkout goes
with the runner. So `wrote new bytes` on the default branch is the sentence
saying the published catalogue no longer matches what the declarations come to
and a merge is owed, and reading that, then producing the bytes the request
below carries, is the part that stayed a person's.

The sentence that stood at this entry until this correction is superseded rather
than deleted. It said the publication had no schedule, and it sent a reader to
#32 as the place where a schedule and a run that could not race itself were
still being decided. #32 was answered on 2026-08-20 and the workflow file above
carries both, so what that sentence pointed forward to has arrived and what is
left by hand here is the paragraph above rather than starting the run.

Opening the pull request that carries the placed file, and merging it. This is
the same route as any other change and it has the same gate in front of it.

Waiting for the deployment, and deciding it happened. There is no workflow file
here that deploys the site, so nothing in this tree reports the deployment's
verdict, and the only way to know is to read the address.

Reading the address, and writing the reading into `internal/address.Answered` in
the same commit as the request output that backs it. This is the step that
converts a working address into an address anybody is allowed to print.

Tagging the tree the catalogue was published from. This repository carries no
tags today, so the first release chooses the shape as well as the number.

## What a release is refused for

Four states. Each of them makes a release actively harmful rather than merely
early, which is the line: waiting is cheap, and every one of these hands an
operator something worse than nothing.

No terms in the tree. A catalogue nobody may redistribute, package or fork,
found out after it is public and after somebody has already built on it.

No address recorded as answering. The instruction would tell an operator to
paste something that answers with nothing, and a Jellyfin server shows that as
an empty repository and no error at all. It is indistinguishable from a typo,
from a server that cannot reach the network, and from a project that is not
ready, which is why the quickstart in #55 cannot be written before this clears.

A published catalogue that is not current. The catalogue parses, lists the
plugin, and does not carry the version somebody is waiting for. Nothing reports
this anywhere: the plugins simply stop moving, and it surfaces when a person
asks why a version is old.

Nothing to publish. A run that resolved zero plugins produces exactly the file a
correct run over an empty world would produce, so publishing it replaces a
working catalogue with an empty one and reports success.
`decisions/failure-posture.md` spends its longest section on that case.

## What decides them

    go run . release

It reads each condition, prints what it read, and exits non-zero while any of
them holds. The conditions are `internal/readiness`'s and every one of them is
tripped in that package's own suite against a planted reading, so a condition
that has never refused anything is not among them.

A condition that does not clear has two answers, and they are not the same
sentence. A condition that HOLDS is a fact about this tree, and the
repair is to change the tree. A condition NOT EVALUATED is a fact about this
run: the address nobody has recorded, the read that failed, the release lists
that could not be fetched. Both refuse. An unread condition is never a clear
one, because the reading that would be most convenient to treat as a pass is
exactly the one taken on the night somebody wants to release.

The verb reaches the network for two of the four, which is why it is a verb
somebody runs and never a leg of the merge gate.
`decisions/headless-and-unelevated.md` keeps a merge from depending on somebody
else's service being up, and a leg deciding what an address answers with would
be precisely that.

## What refuses a violation

Nothing refuses a person who does not run the verb. It is not a leg, no ruleset
requires it, and this repository has no release event for anything to hang off,
so a release cut without ever asking leaves the same trace as one that asked and
was cleared. That is the same gap between a rule and a reminder that this file
exists to close, standing one step further back, and naming it is worth more
than a sentence claiming the procedure is enforced.

What is enforced is narrower and is worth stating exactly. The merge gate
refuses a tracked file printing an install address that is not recorded as
answering, so the instruction cannot be written ahead of the address whatever
anybody does with the verb. A publication run refuses a set that resolved
nothing and refuses a defect in a release it is about to publish. Neither of
those is the release path; both are in front of parts of it.

The shape that would close the rest is a release event this repository does not
have yet, with the verb between the tag and anything the tag causes. Adding one
before there is a first release to hang it on would be building the enforcement
for a procedure nobody has followed once.

## What this costs

A release that could have gone out tonight waits for an address to be read and
written down, and reading it is a person's step rather than a run's. That is the
price of the four conditions being asked at all, and it is paid every time.

The alternative is what the conditions are written against: a release cut on a
tree where one of them held, published to an address that answers on its own
schedule to servers nobody can reach, with no error raised anywhere and no way
to withdraw it.
