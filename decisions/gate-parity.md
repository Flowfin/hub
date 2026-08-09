# Which legs of the plugin board's gate this board carries

The gate on `iderex/jellyfin-plugin-sso` is the target this board is measured
against. It is public, so it is read rather than described:

    gh api repos/iderex/jellyfin-plugin-sso/rules/branches/main \
      --jq '.[] | select(.type=="required_status_checks")
            | .parameters.required_status_checks[].context'
    build
    ABI floor build
    Package (JPRM) / Build package
    Package (JPRM) / Generate SBOM
    CodeQL
    Analyze (csharp)
    DCO sign-off
    Deterministic PR-hygiene checks
    Enforce greppable invariants
    Reject Trojan Source Unicode
    Audit workflows (zizmor)
    prettier
    dependency-review

Run 2026-08-09. Thirteen names, and they are read at the time of writing rather
than copied from where this question was raised, because a required set moves.

Parity does not mean copying. That board is a compiled plugin with a test
project, a fuzz harness and a packaging step, and several of its legs have
nothing here to look at. What parity means is that every leg is accounted for:
adopted, adapted, or dropped, and a dropped leg says what it was protecting and
why that thing does not exist here. A leg dropped in silence is
indistinguishable from a leg forgotten.

## What this board requires today

The state this document was written against, and it has moved since the question
was raised: at that point this board required nothing at all.

    gh api repos/Flowfin/hub/rules/branches/main --jq '[.[].type]'
    ["deletion","non_fast_forward","pull_request","required_status_checks"]
    gh api repos/Flowfin/hub/rules/branches/main \
      --jq '.[] | select(.type=="required_status_checks")
            | .parameters.required_status_checks[].context'
    Gate: build
    Gate: test
    Gate: format
    Gate: tests-reach-nothing
    Gate: no-hardcoded-names
    Gate: site-fetches-nothing-outside
    DCO sign-off
    dependency-review
    Reject Trojan Source Unicode
    Audit workflows (zizmor)

Both run 2026-08-09. Ten required names against the target's thirteen.

One of this board's own legs is missing from that list. `Gate: editorconfig`
landed after the ruleset was written and nothing adds a name to a ruleset on its
own, so the newest leg runs on every pull request and blocks nothing. Widening
the required set is #48 and is not done here; recording that the gap exists is.

## The legs, one line each

**build.** ADAPTED. The target compiles a plugin against a pinned SDK. Here the
same word means `go build ./...` behind `go run . gate build`, which is a
different compiler and the same obligation: a tree that does not compile does not
merge.

**ABI floor build.** DROPPED. It builds the plugin against the oldest Jellyfin
version the manifest claims to support, so a source file using a newer API is
caught before an operator's server refuses to load the assembly. Nothing here is
loaded by a Jellyfin server; the only thing this board sends one is a JSON
document, whose compatibility question is the `targetAbi` field and is
`decisions/manifest-schema.md`'s.

**Package (JPRM) / Build package.** DROPPED. It runs the ecosystem's packaging
tool over the built plugin to produce the archive an operator installs. This
board produces no archive: it publishes a file that points at archives other
repositories built.

**Package (JPRM) / Generate SBOM.** DROPPED, and it is the one drop worth
arguing rather than stating. An SBOM lists what a build depends on, and this
tree's dependency set is empty and is held empty by the absence of a `go.sum`
that `CONTRIBUTING.md` spends a section on. An SBOM of nothing is a file nobody
reads. The condition that reverses this is the first `require` line in `go.mod`,
not a date.

**CodeQL** and **Analyze (csharp).** ADAPTED, and this is #44's to land rather
than this document's. The target's two names are one analysis over one language.
Go is a language the same scanner supports, so the leg has something to look at
here and is adopted with its language changed; what carries over unchanged is
that a finding blocks rather than annotates.

**DCO sign-off.** ADOPTED, unchanged, and already required. Same workflow shape,
same certificate, same refusal of a non-merge commit without the line.

**Deterministic PR-hygiene checks.** ADAPTED, and it is #45. What carries over is
the tiering, so high-confidence rules block and soft conventions annotate, and
the explicit skip for an author from outside the repository that announces itself
rather than going quiet. What is dropped from it is every rule about a solution
file, a changelog format or a test project, none of which exist here.

**Enforce greppable invariants.** ADAPTED, and it has already landed under other
names. The target runs a pattern linter over its source; this board's equivalent
is `Gate: no-hardcoded-names`, which refuses an account or organisation name
written into source or into a workflow step, and `Gate:
site-fetches-nothing-outside`, which refuses a served page that would load from
another host. Both are Go tests over the tree rather than a pattern language,
which is what `decisions/means.md` asks for: a rule that can be run against a
planted input rather than one that can only be shown to bite by breaking the tree.

**Reject Trojan Source Unicode.** ADOPTED, unchanged, and already required.

**Audit workflows (zizmor).** ADOPTED, unchanged, and already required.

**prettier.** ADAPTED. The target formats its TypeScript and its documents with
a Node tool. Adding one here would add a runtime `decisions/means.md` rules out
for a tree of six workflow files, two HTML pages and some prose, so the same
obligation is carried by `Gate: format` for Go and `Gate: editorconfig` for
everything else, which are the toolchain's own formatter and three whitespace
properties in Go.

**dependency-review.** ADOPTED, unchanged, and already required. It reviews
nothing today, because the dependency set is empty, and that is the state it
exists to notice changing.

## Coverage and mutation

Neither is a required name on the target's list above, and both are legs it runs.
They are recorded here because a parity list that quietly drops what was too much
trouble is the failure this document exists against, and #46 is where that was
raised.

**Coverage. ADOPTED, with a floor, as `Gate: coverage`.** It costs no dependency:
`decisions/means.md` fixes the toolchain as Go, coverage instrumentation arrives
with it, and the leg is the suite plus one flag.

The floor is 40% of statements per package, and it is deliberately below every
value the tree held when it was set:

    go test ./... -cover -count=1
    ok  	flowfin.dev/hub	1.472s	coverage: 48.1% of statements
    ok  	flowfin.dev/hub/internal/coverage	0.481s	coverage: 94.7% of statements
    ok  	flowfin.dev/hub/internal/format	1.864s	coverage: 84.6% of statements
    ok  	flowfin.dev/hub/internal/freshness	0.745s	coverage: 97.7% of statements
    ok  	flowfin.dev/hub/internal/gate	0.745s	coverage: 83.5% of statements
    ok  	flowfin.dev/hub/internal/harness	0.746s	coverage: 73.9% of statements
    ok  	flowfin.dev/hub/internal/links	0.789s	coverage: 85.7% of statements
    ok  	flowfin.dev/hub/internal/names	1.117s	coverage: 90.4% of statements
    ok  	flowfin.dev/hub/internal/pairing	0.749s	coverage: 93.1% of statements
    ok  	flowfin.dev/hub/internal/reach	0.744s	coverage: 87.2% of statements
    ok  	flowfin.dev/hub/internal/releases	1.796s	coverage: 84.9% of statements
    ok  	flowfin.dev/hub/internal/site	0.744s	coverage: 85.9% of statements
    ok  	flowfin.dev/hub/internal/sources	0.782s	coverage: 91.5% of statements
    ok  	flowfin.dev/hub/manifest	0.744s	coverage: 90.3% of statements

Run 2026-08-09 on the branch that landed the leg. #46 names the cost of a floor
on a small tree exactly: it produces pressure to write tests that raise a number.
A floor set under every current value cannot produce that pressure, and it still
refuses the thing a percentage is genuinely good at catching - a package that
arrives with no test at all, which prints no number and fails nothing. Raising it
is a decision recorded here, and a test refuses a floor raised to the numbers
above without that decision.

What the leg does NOT do is worth naming, because it is what a reader will assume
it does. It reads the per-package summary and never a coverage profile, so it
cannot say which statements were missed, and it cannot say that a particular
refusal branch was reached by a test. The obligation that does carry that is the
one in `CONTRIBUTING.md`: a guard ships with a planted input it refuses.

**Mutation. DROPPED.** The Go toolchain carries no mutation runner, so adopting
it means adding the first third-party tool to a tree whose empty dependency set
is a stated property rather than an accident: there is no `go.sum`, the toolchain
refuses an unrequired import at build time, and `dependency-review` exists to
notice that changing. `decisions/means.md` asks for that cost to be paid
knowingly, and the thing bought does not pay it here.

The question mutation asks is a good one and #46 is right about where it bites:
the generator's value is in its refusals, and a test that calls a refusing
function and asserts it returned is green whether the refusal fired or not. This
board answers that question per guard instead of per run. Every pull request
adding a guard shows it refusing a planted input, and the fixtures live in the
tree afterwards, so the answer is in the suite rather than in a nightly report
nobody opens.

The bound on that is real and is stated rather than hidden: a planted fixture
proves the guard bites for the case somebody thought of, and a mutation run looks
for the case nobody did. Nothing here replaces that.

What reverses this drop: a mutation runner arriving in the Go toolchain itself,
or the first `require` line landing in `go.mod` for some other reason, at which
point the property this drop protects is already spent and the cost is only the
run time. A date is not the condition.

## What this board has that the target does not

Named here because parity is a comparison and a comparison in one direction is
half of one.

`Gate: tests-reach-nothing`, from `decisions/headless-and-unelevated.md`. It
refuses a gate test that reaches for the network, a display, elevation or a
privileged port.

`Gate: editorconfig`, the whitespace half of formatting, over the file types no
formatter here reaches.

`Gate: site-links-resolve`, which refuses a link in a served page pointing at a
file the site does not carry. The target has no published site to rot.

`Gate: coverage` is not in this list, because the target runs a coverage leg too.
It is in the section above, where the decision on it is recorded.

The harness, in `.github/workflows/harness.yml`, which is not a gate leg and is
not required: it is the environment-bound checks, asked for deliberately, and the
gate's own report says on every run that none of them ran.

## What is not settled here

Which of these becomes a required name on `main` is #48. This document says what
each leg is; a ruleset is what makes one block, and nothing in this tree can read
a ruleset. The list above was derived by running the commands at the top on
2026-08-09, and it is derived again rather than trusted the next time somebody
needs it.
