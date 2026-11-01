package checker

import (
	"reflect"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func TestCoverageOrderLeavesUnmodeledLocationsAlone(t *testing.T) {
	for _, tc := range []struct {
		name  string
		first parser.Range
		last  parser.Range
	}{
		{"different modules", parser.Range{Module: "Z", Start: parser.Position{Line: 5, Column: 1}, End: parser.Position{Line: 5, Column: 3}}, parser.Range{Module: "A", Start: parser.Position{Line: 2, Column: 1}, End: parser.Position{Line: 2, Column: 3}}},
		{"same location", parser.Range{Module: "M", Start: parser.Position{Line: 5, Column: 1}, End: parser.Position{Line: 5, Column: 3}}, parser.Range{Module: "M", Start: parser.Position{Line: 5, Column: 1}, End: parser.Position{Line: 5, Column: 3}}},
		{"unknown location", parser.Range{}, parser.Range{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := coverageCollector{bodies: map[string]*parser.Expression{
				"B": {Location: parser.Location{Range: tc.first}},
				"A": {Location: parser.Location{Range: tc.last}},
			}}
			actions := []CoverageAction{{Name: "Init"}, {Name: "B"}, {Name: "A"}, {Name: "Inv", Property: true}}
			want := append([]CoverageAction(nil), actions...)
			c.sortCoverageActions(actions)
			if !reflect.DeepEqual(actions, want) {
				t.Fatalf("changed unmodeled order: %+v", actions)
			}
		})
	}
}
