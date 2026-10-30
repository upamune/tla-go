package checker

import (
	"context"
	"fmt"
	"testing"
)

func TestConfigScopedDefinitionInInstances(t *testing.T) {
	// This is the scoped override case from upstream instances.test.ts.
	source := "---- MODULE M ----\nEXTENDS Naturals\nCONSTANT Acc\nVARIABLE x\nMCBallot == 0..2\nV == INSTANCE Inner\nP(A) == INSTANCE Inner WITH Acc <- A\nInit == x = 0\nNext == UNCHANGED x\nInv == V!Safe /\\ P({3})!Safe /\\ V!Ballot = 0..2 /\\ x = 0\n===="
	config := "INIT Init\nNEXT Next\nCONSTANT Acc = {1, 2}\nBallot <-[Inner] MCBallot\nINVARIANT Inv"
	inner := "---- MODULE Inner ----\nEXTENDS Naturals\nCONSTANT Acc\nBallot == Nat\nSafe == \\A a \\in Acc, b \\in Ballot : b >= 0\n===="
	result, err := CheckWithLoader(context.Background(), source, config, func(_ context.Context, name string) (string, error) {
		if name != "Inner" {
			return "", fmt.Errorf("unknown module %s", name)
		}
		return inner, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != Success || result.GeneratedStates != 2 || result.DistinctStates != 1 {
		t.Fatalf("got %+v", result)
	}
}

func TestConfigScopedOverridePreservesOperatorParameters(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nReplacement(n) == n + 10\nI(p) == INSTANCE Inner\nInit == x = 0\nNext == UNCHANGED x\nInv == I(3)!F(1) = 11\n===="
	inner := "---- MODULE Inner ----\nEXTENDS Naturals\nF(n) == n + 1\n===="
	result, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv CONSTANT F <- [Inner] Replacement", func(_ context.Context, _ string) (string, error) { return inner, nil })
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != Success {
		t.Fatalf("got %+v", result)
	}
}

func TestConfigOverrideDoesNotCaptureTarget(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nF(Replacement) == Replacement + 1\nReplacement(n) == n + 10\nInit == x = 0\nNext == UNCHANGED x\nInv == F(1) = 11\n===="
	result, err := Check(source, "INIT Init NEXT Next INVARIANT Inv CONSTANT F <- Replacement")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != Success {
		t.Fatalf("got %+v", result)
	}
}

func TestScopedStandardOverrideRejectsCycles(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nReplacement == Nat\nInit == x = 0\nNext == UNCHANGED x\n===="
	if result, err := Check(source, "INIT Init NEXT Next CONSTANT Nat <- [Naturals] Replacement"); err == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestUnsupportedScopedSubstitutionDoesNotSilentlySucceed(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nReplacement == 0\nInit == x = 0\nNext == UNCHANGED x\n===="
	if result, err := Check(source, "INIT Init NEXT Next CONSTANT Other <- [Naturals] Replacement"); err == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
}
