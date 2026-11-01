package checker

import (
	"reflect"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

// TLC 1.7.4's OpApplNodeWrapper reports a node with no evaluations when all
// nonzero descendants have the same count, but omits that node's secondary
// cost. An inconsistent subtree instead reports the nonzero nodes separately.
func TestCoveragePropertyReportNestedCounts(t *testing.T) {
	r := func(line int) parser.Range { return parser.Range{Module: "M", Start: parser.Position{Line: line}} }
	node := func(line, count, cost int, children ...*coverageNode) *coverageNode {
		return &coverageNode{expr: &parser.Expression{Location: parser.Location{Range: r(line)}}, count: count, cost: cost, children: children}
	}
	root := node(1, 3, 0,
		node(2, 0, 9, node(3, 2, 0), node(4, 2, 0)),
		node(5, 1, 4, node(6, 1, 0)),
		node(7, 0, 6, node(8, 1, 0), node(9, 2, 0)),
		node(10, 0, 0),
	)
	want := []CoverageExpression{
		{Range: r(1), Count: 3},
		{Range: r(2), Depth: 1, Count: 2},
		{Range: r(5), Depth: 1, Count: 1, Cost: 4},
		{Range: r(8), Depth: 1, Count: 1},
		{Range: r(9), Depth: 1, Count: 2},
	}
	if got := root.report(nil, 0); !reflect.DeepEqual(got, want) {
		t.Fatalf("coverage report = %#v, want %#v", got, want)
	}
}
