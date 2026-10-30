package checker

import (
	"context"
	"fmt"
	"testing"
)

func TestScopedConfigAssignmentsInInstances(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nR(n) == n + 1\nI == INSTANCE Inner\nP(a) == INSTANCE Inner\nInit == x = 0\nNext == x' = I!F(x)\nInv == x /= 2 /\\ P(9)!F(0) = 1\n===="
	inner := "---- MODULE Inner ----\nEXTENDS Naturals\nF(n) == n + 1\n===="
	for _, tc := range []struct {
		config              string
		status              Status
		generated, distinct int
	}{
		{"F = [Inner] 1", Success, 3, 2},
		{"F(0) = [Inner] 1 F(1) = [Inner] 2 F(2) = [Inner] 2", InvariantViolation, 3, 3},
		{"F = [Inner] 2 F <- [Inner] R", InvariantViolation, 3, 3},
		{"F(0) = [Inner] 7 F <- [Inner] R", InvariantViolation, 3, 3},
		{"F = [Inner] 2 I!F <- R", InvariantViolation, 1, 1},
		{"F = [Inner] 2 I!F = 1", InvariantViolation, 1, 1},
	} {
		t.Run(tc.config, func(t *testing.T) {
			loader := func(_ context.Context, name string) (string, error) {
				if name != "Inner" {
					return "", fmt.Errorf("unexpected module %s", name)
				}
				return inner, nil
			}
			result, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv CONSTANT "+tc.config, loader)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != tc.status || result.GeneratedStates != tc.generated || result.DistinctStates != tc.distinct {
				t.Fatalf("got %+v", result)
			}
		})
	}
}

func TestScopedStandardValueAssignment(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\nInv == x \\in Nat\n===="
	result, err := Check(source, "INIT Init NEXT Next INVARIANT Inv CONSTANT Nat = [Naturals] {0, 1}")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != InvariantViolation || result.DistinctStates != 3 {
		t.Fatalf("got %+v", result)
	}
}
