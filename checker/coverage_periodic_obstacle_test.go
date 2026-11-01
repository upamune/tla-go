package checker

import (
	"reflect"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func TestCoverageReportPreservesUpdatesAndSnapshots(t *testing.T) {
	first := parser.Range{Module: "M", Start: parser.Position{Line: 1}}
	second := parser.Range{Module: "M", Start: parser.Position{Line: 2}}
	body := parser.Range{Module: "M", Start: parser.Position{Line: 3}}
	propertyExpr := &parser.Expression{Location: parser.Location{Range: first}}
	property := &coverageNode{expr: propertyExpr, count: 1, cost: 2}
	c := coverageCollector{
		actions: []CoverageAction{{Name: "A", Expressions: []CoverageExpression{
			{Range: first, Count: 1}, {Range: second, Count: 1},
		}}, {Name: "P", Property: true}},
		byName:      map[string]int{"A": 0, "P": 1},
		bodies:      map[string]*parser.Expression{"A": {Kind: "And", Location: parser.Location{Range: body}}},
		expressions: map[string]map[parser.Range]int{"A": {first: 0, second: 1}},
		properties:  map[string]*coverageNode{"P": property},
	}
	before := c.report()
	wantBefore := []CoverageAction{
		{Name: "A", Expressions: []CoverageExpression{{Range: body, Count: 1}}},
		{Name: "P", Property: true, Expressions: []CoverageExpression{{Range: first, Count: 1, Cost: 2}}},
	}
	if !reflect.DeepEqual(before, wantBefore) || !reflect.DeepEqual(c.report(), before) {
		t.Fatalf("unexpected or unstable report: %+v", before)
	}
	g := generator{model: &model{coverage: &c}}
	g.cover(&parser.Expression{Location: parser.Location{Range: second}}, "A")
	c.generated("A", false)
	c.distinct("A", false)
	property.observe(propertyExpr)
	property.cost++
	after := c.report()
	wantAfter := []CoverageAction{
		{Name: "A", Generated: 1, Distinct: 1, Expressions: []CoverageExpression{{Range: first, Count: 1}, {Range: second, Count: 2}}},
		{Name: "P", Property: true, Expressions: []CoverageExpression{{Range: first, Count: 2, Cost: 3}}},
	}
	if !reflect.DeepEqual(after, wantAfter) || !reflect.DeepEqual(c.report(), after) {
		t.Fatalf("unexpected or unstable report after updates: %+v", after)
	}
	if !reflect.DeepEqual(before, wantBefore) {
		t.Fatalf("earlier report changed: %+v", before)
	}
	after[0].Expressions[1].Count = 99
	after[1].Expressions[0].Cost = 99
	if !reflect.DeepEqual(c.report(), wantAfter) {
		t.Fatal("mutating a report changed the collector")
	}
}
