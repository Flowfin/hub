# The means: Go, gofmt, and the toolchain's own test runner

The generator and the checks are written in Go, at a version floor of 1.25. The
test runner is `go test`, which ships with the toolchain. The formatter is
`gofmt`, which ships with it too, and the formatting leg is `gofmt -l` over the
tree with a non-empty result failing the run.

The floor is the older of the two release lines still carrying security fixes,
so a build machine is never required to be on the newest one:

    curl -sSL "https://go.dev/dl/?mode=json" | python -c "import json,sys; print([r['version'] for r in json.load(sys.stdin)])"
    ['go1.26.5', 'go1.25.12']

Run 2026-08-08. That pair moves, so the floor is recorded in `go.mod` by #17 and
`go.mod` is the authority for it, not this sentence.

The published site stays what it is. HTML and CSS, with the small amount of
JavaScript already in it. Nothing below argues for rewriting a page.

## Can the means express a rule a machine refuses

Yes, and this is the question that decided it.

Almost every rule this project has written down is a refusal. An unpairable
checksum is refused. A declaration that does not resolve is refused. An account
name spelled out in the source is refused. A version entry with no checksum is
refused. Each of those is a function that reads something and returns a verdict,
and each one needs a test that plants the violation and watches the verdict turn
red, because a guard nobody has seen refuse anything might be matching nothing at
all.

Go puts the refusal and the test that proves it bites in the same language, the
same package and the same run. The alternative shape, a shell fragment inside a
workflow file, has no suite, cannot be run against a fixture without pushing a
branch, and cannot be proven to bite except by breaking the tree on purpose. Two
of the workflows this repository already carries are that shape:

    grep -ln 'run: |' .github/workflows/*.yml
    .github/workflows/dco.yml
    .github/workflows/unicode-guard.yml

Run 2026-08-08 at 6a98de6. Both are worth keeping, because they are cheap and
because rewriting somebody's working scanner is not free. What they are not is a
pattern to extend once the checks start encoding decisions rather than scanning
for characters.

## What it adds that the tree does not already carry

A language and a toolchain, and the cost is paid knowingly.

The tree carries HTML, CSS, a little JavaScript, and workflow YAML. Go is a new
thing to install, pin, update and scan, and there is no honest way to write that
cost down as zero.

What reduces it is that one download supplies the compiler, the test runner, the
formatter, the coverage instrumentation and the dependency pinning, so the count
of things to install and keep current is one rather than a runtime plus a package
manager plus a formatter plus a test framework. The work the generator does,
reading a JSON API over HTTPS and writing a JSON file, is standard library work,
so the dependency set the two supply-chain workflows on this board have to look
at can plausibly stay empty. Whether it does stay empty is a fact about the tree
that #17 will establish and this file cannot.

## Would the tests run in the same job

Yes. One command runs every test, locally and in the job, and there is no second
apparatus. That matters because the checks and the generator are the same
program: the leg that refuses a hardcoded account name is Go code with a Go test
behind it, not a separate tool with its own way of being run.

The environment-bound tests from #13 are the same runner with a build tag, so the
thing somebody has to remember is a flag rather than a second toolchain.

## What was not chosen

Node. The closest call, and the argument for it is real: the tree already has
JavaScript in it, and a test runner now ships with the runtime. It loses on the
dependency surface. The JavaScript in this tree runs in a browser, so Node is a
runtime the tree does not have rather than one it already carries, and a
formatter is a third-party dependency with its own update cadence. On a board
whose gate is partly about what it depends on, starting with a lockfile to
maintain is a cost paid on every scan.

Python. Nothing here needs it, and the version and environment question, which
interpreter and which packaging, is a recurring cost rather than a one-time one.
It stays the right tool for a one-off measurement, and several numbers in the
neighbouring decision files were produced with it, which is not the same as
building on it.

Shell inside workflow YAML. Ruled out for the reason in the first section rather
than by preference. It is the right means for the small number of places where
the interface is genuinely a shell, and it is not a place to put a rule that has
to be proven.

C# would put the generator in the same language as the server it publishes to,
and that reads like an advantage until you notice the generator never runs
against the server. It talks to a release API and writes a file. What it would
buy is familiarity with the manifest's own shape, which is a JSON document
anything can write, and what it would cost is a heavier toolchain for a program
that fits in a few hundred lines.

## What this does not settle

Whether the answer is right. It is argued here rather than measured, and the
place it gets tested is #17, where the toolchain lands and the dependency set
either stays empty or does not. Reversing it costs less before #17 than after,
and the reason to reverse it would be a measurement, not a preference.
