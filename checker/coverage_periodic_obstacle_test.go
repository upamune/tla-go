package checker

import (
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

// A live report cannot call finish on the collector: subsequent observations
// use indices into the original expression slice.
func TestCoverageFinishIsNotALiveSnapshot(t *testing.T) {
	first := parser.Range{Module: "M", Start: parser.Position{Line: 1}}
	second := parser.Range{Module: "M", Start: parser.Position{Line: 2}}
	c := coverageCollector{
		actions: []CoverageAction{{Name: "A", Expressions: []CoverageExpression{
			{Range: first, Count: 1}, {Range: second, Count: 1},
		}}},
		bodies:      map[string]*parser.Expression{"A": {Kind: "And"}},
		expressions: map[string]map[parser.Range]int{"A": {first: 0, second: 1}},
	}
	c.finish()
	if got, index := len(c.actions[0].Expressions), c.expressions["A"][second]; got != 1 || index < got {
		t.Fatalf("expected collapsed report and stale update index, got length %d and index %d", got, index)
	}
}
