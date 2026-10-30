package checker

import (
	"fmt"
	"testing"
)

func BenchmarkJavaTLCEventuallyAlways(b *testing.B) {
	for _, tc := range []struct {
		name, predicate string
		status          Status
	}{
		{"holds", "x < 1000", Success},
		{"violated", "x = 0", LivenessViolation},
	} {
		b.Run(tc.name, func(b *testing.B) {
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == /\\ x < 999 /\\ x' = x + 1\nP == <>[](%s)\n====\n", tc.predicate)
			cfg := "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE"
			b.ReportAllocs()
			for b.Loop() {
				result, err := Check(source, cfg)
				if err != nil || result.Status != tc.status || result.DistinctStates != 1000 {
					b.Fatalf("got %+v, %v", result, err)
				}
				if tc.status == LivenessViolation && len(result.Violation.Trace) != 1000 {
					b.Fatalf("unexpected trace length: %d", len(result.Violation.Trace))
				}
			}
		})
	}
}
