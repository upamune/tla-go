package checker

import (
	"context"
	"testing"
)

func TestModelContinue(t *testing.T) {
	for _, tc := range []struct {
		name, body, cfg               string
		status                        Status
		violations, generated, states int
	}{
		{"initial", "Init == x \\in {0,1}\nNext == UNCHANGED x\nInv == x=0", "INVARIANT Inv", Success, 1, 4, 2},
		{"multiple initial", "Init == x=0\nNext == UNCHANGED x\nA == x=1\nB == x=2", "INVARIANT A B", Success, 2, 2, 1},
		{"successor", "Init == x=0\nNext == x' \\in {0,1,2}\nInv == x=0", "INVARIANT Inv", Success, 2, 10, 3},
		{"action", "Init == x=0\nNext == x' \\in {0,1}\nP == [][x'=x]_x", "PROPERTY P", Success, 2, 5, 2},
		{"deadlock still stops", "Init == x=0\nNext == /\\ x=0 /\\ x'=1", "", Deadlock, 0, 2, 2},
		{"evaluation still stops", "Init == x=0\nNext == x'=1\nInv == IF x=0 THEN TRUE ELSE 0=\"a\"", "INVARIANT Inv", EvaluationFailure, 0, 2, 2},
		{"evaluation after violation", "Init == x=0\nNext == x' \\in {0,1,2}\nInv == IF x=0 THEN TRUE ELSE IF x=1 THEN FALSE ELSE 0=\"a\"", "INVARIANT Inv", EvaluationFailure, 1, 4, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nVARIABLE x\n" + tc.body + "\n===="
			var events []Result
			opts := Options{Continue: true, OnViolation: func(result Result) { events = append(events, result) }}
			result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next "+tc.cfg, opts)
			if err != nil || result.Status != tc.status || result.GeneratedStates != tc.generated || result.DistinctStates != tc.states || result.ContinuedViolations != tc.violations || len(events) != tc.violations {
				t.Fatalf("result %+v, events %+v, error %v", result, events, err)
			}
			for index, event := range events {
				if event.Violation == nil || len(event.Violation.Trace) == 0 || event.ContinuedViolations != index+1 {
					t.Fatalf("event %d: %+v", index, event)
				}
			}
		})
	}
}

func TestModelContinueWithoutCallback(t *testing.T) {
	const source = "---- MODULE M ----\nVARIABLE x\nInit == x=0\nNext == x' \\in {0,1,2}\nInv == x=0\n===="
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv", Options{Continue: true})
	if err != nil || result.Status != Success || result.ContinuedViolations != 2 || result.DistinctStates != 3 {
		t.Fatalf("result %+v, error %v", result, err)
	}
}
