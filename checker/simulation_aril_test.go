package checker

import (
	"context"
	"testing"
)

func TestSimulationAril(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x=0\nA == /\\ x=0 /\\ x'=1\nB == /\\ x=0 /\\ x'=2\nStay == /\\ x>0 /\\ UNCHANGED x\nNext == A \\/ B \\/ Stay\nInv == x#2\n===="
	for _, tc := range []struct {
		aril      int64
		status    Status
		generated int
	}{
		{-2, Success, 6}, {0, Success, 6}, {1, Success, 6}, {2, InvariantViolation, 2}, {1000000, InvariantViolation, 2},
	} {
		opts := Options{Seed: 1, Simulation: &SimulationOptions{Traces: 1, Depth: 5, Aril: tc.aril}}
		result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv", opts)
		if err != nil || result.Status != tc.status || result.GeneratedStates != tc.generated {
			t.Errorf("aril %d: result %+v, error %v", tc.aril, result, err)
		}
	}
}
