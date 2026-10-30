package checker

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkParameterizedInstanceState(b *testing.B) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nI(z) == INSTANCE A WITH y <- z\nInit == x = 0\nNext == x' = (x + 1) % 128\nInv == I(x)!Nonnegative\n====\n"
	const dependency = "---- MODULE A ----\nEXTENDS Naturals\nVARIABLE y\nNonnegative == y >= 0\n====\n"
	loader := func(_ context.Context, name string) (string, error) {
		if name != "A" {
			return "", fmt.Errorf("unexpected module %s", name)
		}
		return dependency, nil
	}
	b.ReportAllocs()
	for b.Loop() {
		result, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv", loader)
		if err != nil || result.Status != Success || result.DistinctStates != 128 {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}

func BenchmarkParameterizedInstanceConstant(b *testing.B) {
	const source = "---- MODULE M ----\nI(p) == INSTANCE A WITH C <- p\nASSUME I(1)!Valid\n====\n"
	const dependency = "---- MODULE A ----\nCONSTANT C\nValid == C = 1\n====\n"
	loader := func(_ context.Context, name string) (string, error) {
		if name != "A" {
			return "", fmt.Errorf("unexpected module %s", name)
		}
		return dependency, nil
	}
	b.ReportAllocs()
	for b.Loop() {
		result, err := CheckWithLoader(context.Background(), source, "", loader)
		if err != nil || result.Status != Success {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}
