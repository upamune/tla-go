package evaluator

import (
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func BenchmarkUnchangedEvaluation(b *testing.B) {
	for _, tc := range []struct{ name, expression string }{
		{"tuple", "UNCHANGED vars"},
		{"scalar", "UNCHANGED x"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			m, err := parser.ParseModule("---- MODULE M ----\nVARIABLES x, y, z\nvars == <<x,y,z>>\nCheck == " + tc.expression + "\n====")
			if err != nil {
				b.Fatal(err)
			}
			p := New(m)
			one, _ := value.Integer(1)
			ctx := Context{Current: State{"x": one, "y": one, "z": one}, Next: State{"x": one, "y": one, "z": one}}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				v, err := p.Evaluate(m.Definitions[1].Body, ctx)
				if err != nil || v.String() != "TRUE" {
					b.Fatalf("got %v, %v", v, err)
				}
			}
		})
	}
}
