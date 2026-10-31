package evaluator

import "testing"

func BenchmarkJavaRandomAdvanceDoubles(b *testing.B) {
	random := NewJavaRandom(1)
	b.ReportAllocs()
	for b.Loop() {
		random.SetSeed(1)
		random.AdvanceDoubles(1<<62 + 12345)
	}
}
