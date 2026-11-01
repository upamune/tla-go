package checker

import (
	"context"
	"testing"
)

func BenchmarkSimulationToolLasso(b *testing.B) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x=0\nNext == x'=(x+1)%2\nP == []<>(x=2)\nR == <>[]<>(x=2)\n===="
	for _, tc := range []struct {
		name string
		omit bool
	}{{"default", false}, {"omit", true}} {
		b.Run(tc.name, func(b *testing.B) {
			opts := Options{Seed: 1, Simulation: &SimulationOptions{Traces: 1, Depth: 100, OmitLassoPropertyNames: tc.omit}}
			b.ReportAllocs()
			for b.Loop() {
				result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTIES P R CHECK_DEADLOCK FALSE", opts)
				if err != nil || result.Status != LivenessViolation {
					b.Fatalf("%+v %v", result, err)
				}
			}
		})
	}
}
