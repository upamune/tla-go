package checker

import (
	"testing"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

func TestCompactCycleWitnessesKeepsDistinctConcreteSuccessors(t *testing.T) {
	one, _ := value.Integer(1)
	two, _ := value.Integer(2)
	first := graphEdge{to: 1, action: "Next", next: evaluator.State{"x": one}}
	second := graphEdge{to: 1, action: "Next", next: evaluator.State{"x": two}}
	f := fairSearch{search: &search{model: &model{names: []string{"x"}}}}
	witnesses := []cycleWitness{{node: 0, edge: &first}, {node: 0, edge: &first}, {node: 0, edge: &second}, {node: 0}}
	got, err := f.compactCycleWitnesses(witnesses)
	if err != nil || len(got) != 3 || got[0].edge != &first || got[1].edge != &second || got[2].edge != nil {
		t.Fatalf("got %+v, %v; want first, second, and state witnesses", got, err)
	}
}
