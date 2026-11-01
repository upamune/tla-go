package checker

import (
	"context"
	"testing"
)

func BenchmarkModelLivenessEvents(b *testing.B) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 40\nP == []<>(x = 0) \\/ []<>(x = 20)\n===="
	opts := Options{OnSearch: func(SearchEvent) {}}
	b.ReportAllocs()
	for b.Loop() {
		result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE", opts)
		if err != nil || result.Status != LivenessViolation {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}

func BenchmarkTableauLivenessEvents(b *testing.B) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 40\nP == <>(x = 40) /\\ <>(x = 41)\n===="
	opts := Options{OnSearch: func(SearchEvent) {}}
	b.ReportAllocs()
	for b.Loop() {
		result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE", opts)
		if err != nil || result.Status != LivenessViolation {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}
