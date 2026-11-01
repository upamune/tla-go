package checker

import (
	"context"
	"testing"
)

func BenchmarkCoverageDisabledSubsetInvariant(b *testing.B) {
	const source = `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x \in 0..9
A == x' = x
B == x' = x
Next == A \/ B
Exp(N) == \A e \in SUBSET (1..N) : \A f \in e : f \in 1..N
Inv == /\ x \in Nat
       /\ Exp(6)
====
`
	const cfg = "INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n"
	for b.Loop() {
		result, err := CheckWithOptions(context.Background(), source, cfg, Options{})
		if err != nil || result.Status != Success {
			b.Fatalf("result %v: %v", result.Status, err)
		}
	}
}
