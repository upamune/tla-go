package checker

import "testing"

func BenchmarkGeneralTemporal(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 40\nP == []<>(x = 0) \\/ []<>(x = 20)\n===="
	for b.Loop() {
		result, err := Check(source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE")
		if err != nil || result.Status != LivenessViolation {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}

func BenchmarkGeneralTemporalLet(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 40\nP == LET Is(y) == x=y F(y) == []<>(Is(y)) IN F(0) \\/ F(20)\n===="
	for b.Loop() {
		result, err := Check(source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE")
		if err != nil || result.Status != LivenessViolation {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}

func BenchmarkGeneralTemporalWithMergedView(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 40\nV == x % 20\nP == []<>(x = 0) \\/ []<>(x = 20)\n===="
	for b.Loop() {
		result, err := Check(source, "INIT Init NEXT Next VIEW V PROPERTY P CHECK_DEADLOCK FALSE")
		if err != nil || result.Status != LivenessViolation {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}
