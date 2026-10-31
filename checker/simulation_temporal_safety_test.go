package checker

import (
	"context"
	"errors"
	"testing"
)

// TLC splits a property only at conjunctions and zero-arity definitions, so
// in simulation a quantified [] formula is checked as a temporal property.
func TestSimulationQuantifiedSafetyIsTemporal(t *testing.T) {
	const source = "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' \\in {0, 1, 2}\nP == \\A v \\in {2} : [](x # v)\nQ(v) == [](x # v)\nR == Q(2)\nA == \\A v \\in {2} : [][x' # v]_x\n===="
	opts := Options{Seed: 1, Simulation: &SimulationOptions{Traces: 3, Depth: 6}}
	for _, name := range []string{"P", "R"} {
		got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY "+name, opts)
		if err != nil || got.Status != LivenessViolation || got.Violation == nil || got.Violation.LoopStart == nil {
			t.Fatalf("%s: got %+v, %v; want a liveness counterexample", name, got, err)
		}
	}
	_, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY A", opts)
	if validation, ok := errors.AsType[*ModelValidationError](err); !ok || validation.Code != 2214 {
		t.Fatalf("got %v; want TLC's error 2214 for an action below a quantifier", err)
	}
}
