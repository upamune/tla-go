package evaluator

import (
	"testing"

	"github.com/upamune/tla-go/value"
)

func BenchmarkSequenceSubsequence(b *testing.B) {
	seq, _ := value.Tuple([]value.Value{value.Bool(true), value.Bool(false), value.Bool(true), value.Bool(false)})
	start, _ := value.Integer(2)
	end, _ := value.Integer(3)
	args := []value.Value{seq, start, end}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := sequenceSubsequence(args); err != nil {
			b.Fatal(err)
		}
	}
}
