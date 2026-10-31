package evaluator

import (
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

// BenchmarkSetConstruction measures the sets that are checked against
// -maxSetSize while they are built.
func BenchmarkSetConstruction(b *testing.B) {
	m, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Naturals\nMap == {i * 2 : i \\in 1..2000}\nFilter == {i \\in 1..2000 : i % 3 = 0}\n====")
	if err != nil {
		b.Fatal(err)
	}
	p := New(m)
	for _, definition := range m.Definitions {
		b.Run(definition.Name, func(b *testing.B) {
			for b.Loop() {
				if _, err := p.Evaluate(definition.Body, Context{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
