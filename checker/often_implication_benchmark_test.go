package checker

import "testing"

func BenchmarkOftenImplication(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 100\nA == x < 50 /\\ x' = x + 1\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == []<><<A>>_x => []<><<x' = 200>>_x\n===="
	b.ReportAllocs()
	for b.Loop() {
		result, err := Check(source, "SPECIFICATION Spec PROPERTY P")
		if err != nil || result.Status != LivenessViolation {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}
