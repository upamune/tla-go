package checker

import "testing"

func BenchmarkPureRecurrence(b *testing.B) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 1000\nP == []<>(x = 0)\n===="
	b.ReportAllocs()
	for b.Loop() {
		result, err := Check(source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE")
		if err != nil || result.Status != LivenessViolation {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}
