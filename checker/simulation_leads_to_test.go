package checker

import (
	"context"
	"testing"
)

func TestSimulationLeadsTo(t *testing.T) {
	for _, tc := range []struct {
		name, property string
		status         Status
		states         []int32
	}{
		{"repeated tableau state", "(x=0) ~> (x=1)", Success, nil},
		{"violation", "(x=1) ~> (x=2)", LivenessViolation, []int32{0, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nVARIABLE x\nInit == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=0\nP == " + tc.property + "\n===="
			got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P", Options{Seed: 1, Simulation: &SimulationOptions{Traces: 1, Depth: 5}})
			if err != nil || got.Status != tc.status {
				t.Fatalf("got %+v, %v; want %s", got, err, tc.status)
			}
			if tc.status != LivenessViolation {
				return
			}
			if got.Violation == nil || len(got.Violation.Trace) != len(tc.states) {
				t.Fatalf("wrong trace: %+v", got.Violation)
			}
			for i, want := range tc.states {
				value, ok := got.Violation.Trace[i].State["x"].Int()
				if !ok || value != want {
					t.Fatalf("state %d = %v, want %d", i, got.Violation.Trace[i].State, want)
				}
			}
		})
	}
}

func TestSimulationMultipleLeadsTo(t *testing.T) {
	const source = "---- MODULE M ----\nVARIABLE x\nInit == x=0\nNext == x'=0\nP == (x=1) ~> (x=2)\nQ == (x=0) ~> (x=1)\n===="
	opts := Options{Seed: 1, Simulation: &SimulationOptions{Traces: 1, Depth: 5}}
	got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P Q", opts)
	if err != nil || got.Status != LivenessViolation || got.Violation == nil || got.Violation.Name != "Q" {
		t.Fatalf("got %+v, %v; want a violation of Q", got, err)
	}
	if len(got.Violation.Trace) != 1 || got.Violation.LoopStart == nil || *got.Violation.LoopStart != 0 {
		t.Fatalf("unexpected counterexample: %+v", got.Violation)
	}
}
