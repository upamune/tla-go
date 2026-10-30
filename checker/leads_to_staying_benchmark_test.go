package checker

import "testing"

func BenchmarkLeadsToStaying(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 100\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == (x = 10) ~> [](x < 50)\n===="
	b.ReportAllocs()
	for b.Loop() {
		result, err := Check(source, "SPECIFICATION Spec PROPERTY P")
		if err != nil || result.Status != LivenessViolation {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}
