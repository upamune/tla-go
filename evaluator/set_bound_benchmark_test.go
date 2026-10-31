package evaluator

import (
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

// BenchmarkSetConstruction measures the sets and set operations that are
// checked against -maxSetSize.
func BenchmarkSetConstruction(b *testing.B) {
	m, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Naturals\nMap == {i * 2 : i \\in 1..2000}\nFilter == {i \\in 1..2000 : i % 3 = 0}\n" +
		"Operations == \\A i \\in 1..200 : ({i, 1} \\cup {2}) \\ ({3} \\cap {i}) # {}\nUnion == \\A i \\in 1..200 : UNION {{i}, {1, 2}} # {}\n====")
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
