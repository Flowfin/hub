# The account and organisation names are data, never constants in the source

Two names run through this project and they are not the same name. The plugins
are released under the account `iderex`. The catalogue and the site are published
under the organisation `Flowfin`. Either can move, and the move is somebody
else's decision made for somebody else's reason.

Every place in this repository that spells one of them out is a place that has to
be found and edited when that happens. The ones that get missed do not fail
loudly. They publish a manifest that points at a repository nobody owns any more,
and a server reading it reports an empty catalogue or a download that will not
verify, with nothing in the interface saying why.

The scale of the alternative is not a guess. #6 is where the count is recorded:
the same class of assumption, allowed to spread elsewhere, had to be removed
from twenty-nine separate places once somebody went looking. That number comes
from the issue rather than from any measurement made against this tree, and
nothing here can reproduce it.

## The rule

Both names come from the declaration file described in the source-set decision,
and from nowhere else.

Neither name appears in generator source. Neither appears in the logic of a
workflow step. Neither appears in a test's expected output, except where a
fixture is deliberately naming a fixture, which is a different thing from a test
asserting on the real account.

A name reaching the generator arrives as a value it read, so pointing the tool at
a second organisation is editing one file rather than auditing the tree.

## Where they come from instead

The declaration file is the single place either name is written. It names the
account that owns the source repositories and the organisation the catalogue is
published under, as separate fields, because they are separate facts and
collapsing them is how the wrong one ends up in the manifest's owner field.

Which of the two names the manifest's owner field carries is a different
question and is not settled here. That is entry 4 of #1.

## The check that refuses a violation

`no-hardcoded-names`, a named leg of the gate, built by #30.

It refuses a literal account or organisation name in the paths this rule covers.
It needs a declared evidence scope, or it fails on the declaration file that is
supposed to hold those names and on the fixtures that deliberately spell them
out. Naming that scope is part of #30 rather than an exception carved out of it.

This file owes the rule and the name. It does not owe the code, and until #30
lands there is nothing that refuses a violation of anything written above.
