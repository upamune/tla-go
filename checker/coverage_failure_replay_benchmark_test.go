package checker

import (
	"context"
	"testing"
)

func BenchmarkCoverageInvariantEvaluationFailureReplay(b *testing.B) {
	const source = `---- MODULE M ----
EXTENDS Naturals
VARIABLES x,y
Init == x = 0 /\ y = 0
B == /\ y = 199
     /\ x' = TRUE
     /\ y' = y
A == /\ y < 199
     /\ x' = x
     /\ y' = y + 1
Next == B \/ A
Inv == x < 2
====`
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled"
		}
		b.Run(name, func(b *testing.B) {
			opts := Options{Coverage: enabled}
			b.ReportAllocs()
			for b.Loop() {
				result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv", opts)
				if err != nil || result.Status != EvaluationFailure {
					b.Fatalf("status=%v error=%v", result.Status, err)
				}
			}
		})
	}
}
