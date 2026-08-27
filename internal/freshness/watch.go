package freshness

// WorkflowPath is the file that asks this check on a schedule, relative to the
// repository root.
//
// It is a workflow of its own rather than a schedule added to the harness. The
// harness declares one trigger, a person asking for it, and internal/harness
// refuses any other, because a harness that runs on a timer produces a red
// nobody is holding. This check is the one place that argument does not reach:
// the state it refuses arrives while nobody is looking, and a red nobody is
// holding is exactly what has to be raised. So the schedule goes on a file whose
// whole contract is that it runs unattended, and the harness keeps its own.
//
// Nothing here watches this file by name. internal/sweep derives its watch from
// the files that declare a schedule, so declaring one is what puts this in that
// set, and a run of it ending in anything but success becomes a distinct failure
// keyed on this path and a tracking issue nobody had to ask for. What that costs
// is a delay of up to a day: the sweep runs on its own schedule and reports last
// night's failure this morning.
const WorkflowPath = ".github/workflows/freshness.yml"

// Verb is the command that file's one step runs.
//
// Naming it here rather than only in the workflow is what ties the two ends
// together: the suite refuses a file that carries the schedule, the queue and
// the hardening and runs something else, which passes every other reading of it.
const Verb = "go run . freshness"
