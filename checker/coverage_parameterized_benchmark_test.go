package checker

import (
	"context"
	"testing"
)

func BenchmarkCoverageParameterizedCalls(b *testing.B) {
	const source = `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x \in 0..999
A(n) == x'=n
Next == A(1) \/ A(2)
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
				result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next", opts)
				if err != nil || result.Status != Success || result.DistinctStates != 1000 {
					b.Fatalf("result=%+v error=%v", result, err)
				}
			}
		})
	}
}
