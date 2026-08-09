package main

import (
	"strings"
	"testing"

	"flowfin.dev/hub/internal/harness"
)

func TestNoVerbIsRefused(t *testing.T) {
	var out strings.Builder
	if err := run(nil, &out); err == nil {
		t.Fatal("the entry point with no verb exited zero")
	}
	if !strings.Contains(out.String(), "the legs, in order: build, test, format") {
		t.Fatalf("usage does not list the legs:\n%s", out.String())
	}
}

func TestAnUnknownVerbIsRefused(t *testing.T) {
	var out strings.Builder
	err := run([]string{"gate-all"}, &out)
	if err == nil {
		t.Fatal("an unknown verb exited zero")
	}
	if !strings.Contains(err.Error(), "gate-all") {
		t.Fatalf("the refusal does not name the verb: %v", err)
	}
}

func TestAnUnknownLegIsRefusedBeforeAnythingRuns(t *testing.T) {
	// A mistyped leg name in a workflow step would otherwise be a job that runs
	// nothing and reports green.
	var out strings.Builder
	err := run([]string{"gate", "formatting"}, &out)
	if err == nil {
		t.Fatal("an unknown leg name exited zero")
	}
	if !strings.Contains(err.Error(), "formatting") || !strings.Contains(err.Error(), "build, test, format") {
		t.Fatalf("the refusal does not name the typo and the legs: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("something ran before the leg name was checked:\n%s", out.String())
	}
}

func TestTheUsageSeparatesTheLegsFromTheRequirements(t *testing.T) {
	// A requirement listed as a leg is the invitation to require it on main,
	// which is the merge waiting on somebody else's service.
	var out strings.Builder
	if err := run(nil, &out); err == nil {
		t.Fatal("the entry point with no verb exited zero")
	}
	text := out.String()
	if !strings.Contains(text, "go run . harness") {
		t.Fatalf("usage does not name the harness verb:\n%s", text)
	}
	legs, requirements, found := strings.Cut(text, "the harness requirements")
	if !found {
		t.Fatalf("usage does not separate the two lists:\n%s", text)
	}
	for _, name := range harness.Names(harness.Requirements()) {
		if strings.Contains(legs, name) {
			t.Errorf("requirement %s is listed among the legs:\n%s", name, text)
		}
		if !strings.Contains(requirements, name) {
			t.Errorf("requirement %s is not listed at all:\n%s", name, text)
		}
	}
}

func TestAnUnknownRequirementIsRefusedBeforeAnythingRuns(t *testing.T) {
	var out strings.Builder
	err := run([]string{"harness", "needs-netwrok"}, &out)
	if err == nil {
		t.Fatal("a mistyped requirement exited zero")
	}
	if !strings.Contains(err.Error(), "needs-netwrok") || !strings.Contains(err.Error(), "needs-network") {
		t.Fatalf("the refusal does not name the typo and the requirements: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("something ran before the requirement name was checked:\n%s", out.String())
	}
}

func TestTwoRequirementsAtOnceAreRefused(t *testing.T) {
	// Each requirement is a different thing the runner has to supply, so a run
	// asking for two says nothing about which of them was missing.
	var out strings.Builder
	err := run([]string{"harness", "needs-network", "needs-jellyfin"}, &out)
	if err == nil {
		t.Fatal("two requirements at once exited zero")
	}
	if !strings.Contains(err.Error(), "one requirement at a time") {
		t.Fatalf("the refusal does not say why: %v", err)
	}
}

func TestTheHarnessReportRunsNothing(t *testing.T) {
	// `go run . harness` with no requirement says what the harness holds. If it
	// ran anything it would be reaching the network from a command somebody
	// typed to find out what the harness is.
	var out strings.Builder
	if err := run([]string{"harness"}, &out); err != nil {
		t.Fatalf("the harness report failed: %v", err)
	}
	text := out.String()
	for _, name := range harness.Names(harness.Requirements()) {
		if !strings.Contains(text, name) {
			t.Errorf("the report does not name %s:\n%s", name, text)
		}
	}
	if !strings.Contains(text, "None of them runs in the gate") {
		t.Fatalf("the report does not say the gate is separate from it:\n%s", text)
	}
}
