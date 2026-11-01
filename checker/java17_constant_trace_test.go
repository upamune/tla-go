package checker

import (
	"testing"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

func TestJava17TraceTableauLimits(t *testing.T) {
	formula := &liveNode{kind: liveBool}
	for range 8 {
		formula = liveUnary(liveEven, formula)
	}
	if !java17TraceTableauWithinLimit(formula) {
		t.Fatal("eight branching operators should be supported")
	}
	if java17TraceTableauWithinLimit(liveUnary(liveEven, formula)) {
		t.Fatal("nine branching operators should use the existing path")
	}
	formula = &liveNode{kind: liveBool}
	for range 63 {
		formula = liveUnary(liveAll, formula)
	}
	if !java17TraceTableauWithinLimit(formula) || java17TraceTableauWithinLimit(liveUnary(liveAll, formula)) {
		t.Fatal("unexpected formula size limit")
	}
}

func TestJava17TraceProductLimit(t *testing.T) {
	if !java17TraceGraphWithinLimit(200_000, 5) || java17TraceGraphWithinLimit(200_001, 5) || java17TraceGraphWithinLimit(1, 0) {
		t.Fatal("unexpected product graph size limit")
	}
}

func TestJava17TraceEligibility(t *testing.T) {
	s := search{model: &model{livenessOrder: []temporalProperty{{}}}}
	if !s.java17TraceEligible() {
		t.Fatal("single pure property should be eligible")
	}
	s.model.livenessOrder = append(s.model.livenessOrder, temporalProperty{})
	if s.java17TraceEligible() {
		t.Fatal("multiple properties should use the existing path")
	}
	s.model.livenessOrder = s.model.livenessOrder[:1]
	s.model.fairness = []fairnessCondition{{recurrence: true}}
	if s.java17TraceEligible() {
		t.Fatal("recurrence fairness should use the existing path")
	}
	s.model.fairness = nil
	set, err := value.Subsets(value.Interval(1, 3))
	if err != nil {
		t.Fatal(err)
	}
	s.nodes = []stateNode{{state: evaluator.State{"x": set}}}
	if s.java17TraceEligible() {
		t.Fatal("symbolic state values should use the existing path")
	}
	s.nodes = nil
	s.edges = [][]graphEdge{{{next: evaluator.State{"x": set}}}}
	if s.java17TraceEligible() {
		t.Fatal("symbolic successor values should use the existing path")
	}
}
