package main

import (
	"strings"
	"testing"
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
