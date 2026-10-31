package checker

import (
	"context"
	"testing"
)

func TestMaxSetSizeFailures(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x \\in {i \\in 1..11 : TRUE}\nNext == UNCHANGED x\n===="
	for _, tc := range []struct {
		bound  int
		status Status
	}{{0, Success}, {11, Success}, {5, EvaluationFailure}} {
		result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next", Options{MaxSetSize: tc.bound})
		if err != nil || result.Status != tc.status {
			t.Fatalf("bound %d: result %+v, error %v", tc.bound, result, err)
		}
		failure := result.Failure
		if tc.status == EvaluationFailure && (failure.Message != "Attempted to construct a set with too many elements (>5)." || failure.EvaluationDetail != emptyCallStackDetail || failure.ExitCode != 75) {
			t.Fatalf("bound %d: failure %+v", tc.bound, failure)
		}
	}
}
