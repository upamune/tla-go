package checker

import "testing"

func BenchmarkGuardedSafety(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 100\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == [](x = 0 => [](x >= 0))\n===="
	for _, tc := range []struct{ name, config string }{
		{"baseline", "INIT Init NEXT Next"},
		{"guarded", "INIT Init NEXT Next PROPERTY P"},
		{"guarded_fair", "SPECIFICATION Spec PROPERTY P"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				result, err := Check(source, tc.config)
				if err != nil || result.Status != Success {
					b.Fatalf("got %+v, %v", result, err)
				}
			}
		})
	}
}
