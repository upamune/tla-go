package checker

import (
	"context"
	"testing"
)

func TestSimulationContinue(t *testing.T) {
	for _, tc := range []struct {
		name, definitions, cfg string
		status                 Status
		violations, generated  int
	}{
		{"invariant", "Init == x=0\nNext == x'=1\nInv == x=0", "INIT Init NEXT Next INVARIANT Inv", ContinuedViolation, 3, 4},
		{"action", "Init == x=0\nNext == x'=1\nP == [][x'=x]_x", "INIT Init NEXT Next PROPERTY P", ContinuedViolation, 3, 4},
		{"deadlock", "Init == x=0\nNext == /\\ x=0 /\\ x'=1", "INIT Init NEXT Next", ContinuedViolation, 3, 4},
		{"success", "Init == x=0\nNext == UNCHANGED x", "INIT Init NEXT Next", Success, 0, 16},
		{"initial violation stops", "Init == x=0\nNext == UNCHANGED x\nInv == x=1", "INIT Init NEXT Next INVARIANT Inv", InvariantViolation, 0, 1},
		{"liveness stops", "Init == x=0\nNext == UNCHANGED x\nP == <>(x=1)", "INIT Init NEXT Next PROPERTY P", LivenessViolation, 0, 6},
		{"evaluation stops", "Init == x=0\nNext == x'=1\nInv == IF x=0 THEN TRUE ELSE 0=\"a\"", "INIT Init NEXT Next INVARIANT Inv", EvaluationFailure, 0, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nVARIABLE x\n" + tc.definitions + "\n===="
			var events []Result
			opts := Options{Simulation: &SimulationOptions{Traces: 3, Depth: 5, Continue: true, OnViolation: func(result Result) { events = append(events, result) }}}
			result, err := CheckWithOptions(context.Background(), source, tc.cfg, opts)
			if err != nil || result.Status != tc.status || result.GeneratedStates != tc.generated || result.ContinuedViolations != tc.violations || len(events) != tc.violations {
				t.Fatalf("result %+v, events %+v, error %v", result, events, err)
			}
			for index, event := range events {
				if event.Violation == nil || len(event.Violation.Trace) == 0 {
					t.Fatalf("missing violation trace: %+v", event)
				}
				if event.ContinuedViolations != index+1 {
					t.Fatalf("event %d has count %d", index, event.ContinuedViolations)
				}
			}
		})
	}
}
