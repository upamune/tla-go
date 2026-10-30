package checker

import "testing"

func BenchmarkSpecificationRecurrence(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 100\nWeak == Init /\\ [][Next]_x /\\ WF_x(Next)\nRecurring == Init /\\ [][Next]_x /\\ []<>(x = 50) /\\ []<><<Next>>_x\nP == <>(x = 200)\n===="
	for _, spec := range []string{"Weak", "Recurring"} {
		b.Run(spec, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				result, err := Check(source, "SPECIFICATION "+spec+" PROPERTY P")
				if err != nil || result.Status != LivenessViolation {
					b.Fatalf("got %+v, %v", result, err)
				}
			}
		})
	}
}
