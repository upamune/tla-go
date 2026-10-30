package main

import (
	"bytes"
	"testing"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestPrintTraceAliasFieldOrder(t *testing.T) {
	var out bytes.Buffer
	trace := []checker.TraceStep{{State: evaluator.State{"b": value.Bool(true), "a": value.Bool(false)}, StateOrder: []string{"b", "a"}}}
	printTrace(&out, trace, &parser.Module{})
	if got, want := out.String(), "State 1: <Initial predicate>\n/\\ b = TRUE\n/\\ a = FALSE\n\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPrintInitialErrorStateOrder(t *testing.T) {
	state := evaluator.State{"b": value.Bool(true), "a": value.Bool(false)}
	order := []string{"b", "a"}
	var violation bytes.Buffer
	result := checker.Result{Status: checker.InvariantViolation, Violation: &checker.Violation{Name: "Inv", Trace: []checker.TraceStep{{State: state, StateOrder: order}}}}
	printResult(&violation, result, &parser.Module{})
	want := "Error: Invariant Inv is violated by the initial state:\n/\\ b = TRUE\n/\\ a = FALSE\n\n"
	if violation.String() != want {
		t.Fatalf("got %q; want %q", violation.String(), want)
	}
	var failure bytes.Buffer
	printFailure(&failure, &checker.Failure{Message: "bad", InitialState: state, InitialStateOrder: order}, &parser.Module{})
	want = "Error: bad\nWhile working on the initial state:\n/\\ b = TRUE\n/\\ a = FALSE\n\n"
	if failure.String() != want {
		t.Fatalf("got %q; want %q", failure.String(), want)
	}
}

func TestPrintTraceImportedActionLocations(t *testing.T) {
	state := evaluator.State{"x": value.Bool(true)}
	rangeOfAction := parser.Range{Module: "Imported", Start: parser.Position{Line: 5, Column: 3}, End: parser.Position{Line: 7, Column: 8}}
	trace := []checker.TraceStep{{State: state}, {State: state, Action: "Step", ActionRange: rangeOfAction}}
	var out bytes.Buffer
	printTrace(&out, trace, &parser.Module{})
	want := "State 1: <Initial predicate>\nx = TRUE\n\nState 2: <Step line 5, col 3 to line 7, col 8 of module Imported>\nx = TRUE\n\n"
	if out.String() != want {
		t.Fatalf("got %q; want %q", out.String(), want)
	}
	out.Reset()
	loop := 0
	printLoop(&out, &checker.Violation{Trace: trace, LoopStart: &loop, LoopAction: "Step", LoopActionRange: rangeOfAction}, &parser.Module{})
	want = "Back to state 1: <Step line 5, col 3 to line 7, col 8 of module Imported>\n\n"
	if out.String() != want {
		t.Fatalf("got %q; want %q", out.String(), want)
	}
}
