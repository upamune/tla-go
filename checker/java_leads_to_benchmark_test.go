package checker

import (
	"fmt"
	"testing"
)

func BenchmarkJavaTLCLeadsTo(b *testing.B) {
	for _, tc := range []struct {
		name   string
		goal   int
		status Status
	}{
		{"holds", 0, Success},
		{"violated", 201, LivenessViolation},
	} {
		b.Run(tc.name, func(b *testing.B) {
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == /\\ x < 199 /\\ x' = x + 1\nP == (x = 0) ~> (x = %d)\n====\n", tc.goal)
			cfg := "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE"
			b.ReportAllocs()
			for b.Loop() {
				result, err := Check(source, cfg)
				if err != nil || result.Status != tc.status || result.DistinctStates != 200 {
					b.Fatalf("got %+v, %v", result, err)
				}
			}
		})
	}
}
