# How a client declares conformance, and what a shortfall costs

`decisions/design-system-binding.md` splits the design system into the values a
machine compares and the rules a person judges, and it says what a missed budget
number costs. What it does not say is how a client states where it stands. Until
that has a shape, every client spells conformance its own way, and a reader
comparing two of them is comparing two sentences rather than two measurements.

This file settles the shape. It does not settle the numbers, the method that
produces them, or the file the values are read from, which are #41 and #39.

## A published report, not a badge

A client declares conformance by publishing a report in its own repository and
linking it from wherever it says it follows this system.

A badge carries one bit, and the bit is written by the same hand it describes.
That is the failure this milestone exists against: a claim with nothing behind
it, in the shortest form a claim can take. A report carries the values, the
measurements, the method each one was produced with and the hardware it ran on,
so a reader who doubts it can run it again and find out.

The report lives in the client's repository rather than here. It describes one
build of one client at one moment, so a copy kept in this tree is stale the next
time that client releases, and a stale copy of somebody else's evidence is worse
than no copy. This repository holds the system. Each client holds its own
evidence for its own build.

## What the report carries

The release it describes, by tag and by commit, so it is attached to bytes
somebody can fetch rather than to a version name that moves.

The revision of this system it was measured against, by commit. Both halves of
the yardstick move: the values file is #39 and the measurement method is #41. A
report that does not say which revision it read cannot be compared with the next
one, and two reports on different revisions that disagree are not evidence of
anything.

One entry per value in the machine-compared set, and one entry per budget number.
Which values and which numbers those are is not listed here, because a list in
this file drifts against the file that decides it. The binding file says which
two sets exist, and #39 is where the values themselves land.

Each budget entry carries its figure, the method it was produced with, the
hardware it ran on and the date. A figure on its own is not a measurement. "Slow
on a television bought in 2015" and "slow" are different findings, and only the
first one can be acted on or disputed.

One entry per rule in the judged set, naming who reviewed it and when. These are
the rules a person applies, so the report records that a person applied them and
does not pretend a tool did.

## Three states per entry, and no fourth

Every entry in the report is in exactly one of three states.

Met, with the value or the measurement beside it.

Missed, with the measurement that produced the miss, on the hardware named.

Not measured.

The third state is why the report is per entry rather than a verdict at the top.
A client that has measured all five budget numbers and misses one has said where
it stands. A client that has measured none of them has said nothing. Those two
are not the same position, and a single line at the head of a report collapses
them into one word. So there is no summary verdict in this shape. A reader who
wants one reads the entries, which is the work the collapsing was avoiding.

A client states conformance without qualification only when every entry is met.
Anything else is stated as what it is, in the same words the entries use.
"Conformant apart from" is a sentence this shape refuses to shorten.

## What a shortfall costs

A missed budget number is already answered, in
`decisions/design-system-binding.md`: it is a release condition, and the number
may be changed in the open for everybody but not waived for one release. Nothing
here reopens that.

Two things it does not answer, which belong to the report rather than to the
budget.

A value that does not match blocks nothing on its own. A corner radius two pixels
off harms no one, and a client that cannot publish until every value matches is a
client that stops publishing the report. It is recorded as missed, it is a defect
in that client, and it is fixed in the ordinary way. What it costs is the
unqualified statement, which is the paragraph above.

A rule in the judged set that has not been reviewed is not measured, and it is
carried in the report as such rather than left out. Leaving it out is the same
move as a summary verdict: it turns an absence into a silence, and the reader
cannot tell the difference between a rule that passed and a rule nobody looked
at.

## What this repository may claim meanwhile

The sentence that the clients are held to this system is a statement of intent
until a report exists that a reader can open, and it is worth writing that way
rather than in the present tense.

Whether any client has published a report is not a fact this tree holds. No
client repository is named or linked anywhere in it, so nothing here can be read
as an answer in either direction, and this file does not assert one.

## What refuses a violation

Nothing, today, and this one cannot be fixed here in the way the neighbouring
files can.

Every check this repository owns reads this repository. A client's report lives
in a tree this repository does not hold and does not gate, so no leg of this
gate can refuse a badge, a missing entry or a summary verdict that collapses the
three states. What is available is narrower and worth naming rather than
implying: this repository can hold its own published pages to the wording in the
section above, and it can publish the values and the method in forms a client's
own gate can read, which is #39 and #41.

A report is a claim by the client that carries evidence a reader can check. It is
not a verdict issued from here, and nothing in this file turns it into one.
