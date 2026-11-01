package checker

import (
	"context"
	"testing"
)

func BenchmarkCoverageDirectUnchanged(b *testing.B) {
	const source = `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x \in 0..999
Next == UNCHANGED x
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
