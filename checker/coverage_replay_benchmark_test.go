package checker

import (
	"context"
	"testing"
)

func BenchmarkCoverageSafetyTraceReplay(b *testing.B) {
	const source = `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
A == /\ x < 999
     /\ x' = x + 1
B == /\ x < 999
     /\ x' = x + 1
Next == A \/ B
Inv == x < 999
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
				if err != nil || result.Status != InvariantViolation {
					b.Fatalf("status=%v error=%v", result.Status, err)
				}
			}
		})
	}
}
