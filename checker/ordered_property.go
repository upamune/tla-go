package checker

import (
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// <>(P /\ <>Q) requires a Q at or after some P on the same behavior.
func (m *model) classifyOrderedEventually(p temporalProperty, expr *parser.Expression) (temporalProperty, bool) {
	if expr.Kind != "Eventually" || len(expr.Args) != 1 || expr.Args[0].Kind != "And" {
		return p, false
	}
	trigger, goal, ok := m.orderedEventuallyParts(expr.Args[0].Args)
	if !ok {
		return p, false
	}
	p.trigger, p.predicate, p.mode = trigger, goal, orderedEventually
	p.sequence = []*parser.Expression{trigger, goal}
	return p, true
}

func (m *model) orderedEventuallyParts(parts []*parser.Expression) (*parser.Expression, *parser.Expression, bool) {
	var collected orderedParts
	for _, part := range parts {
		if !m.addOrderedPart(&collected, part) {
			return nil, nil, false
		}
	}
	if len(collected.states) == 0 || collected.goal == nil {
		return nil, nil, false
	}
	if len(collected.states) == 1 {
		return collected.states[0], collected.goal, true
	}
	return &parser.Expression{Kind: "And", Args: collected.states}, collected.goal, true
}

type orderedParts struct {
	states []*parser.Expression
	goal   *parser.Expression
}

func (m *model) addOrderedPart(parts *orderedParts, expr *parser.Expression) bool {
	if expr.Kind == "Eventually" && len(expr.Args) == 1 {
		if parts.goal != nil || !m.isStateExpression(expr.Args[0]) {
			return false
		}
		parts.goal = expr.Args[0]
		return true
	}
	if !m.isStateExpression(expr) {
		return false
	}
	parts.states = append(parts.states, expr)
	return true
}

func (s *search) checkOrderedEventually(p temporalProperty) (bool, error) {
	product, err := s.orderedProduct(p)
	if err != nil {
		return false, err
	}
	neverSatisfied := p
	neverSatisfied.mode = eventually
	neverSatisfied.predicate = &parser.Expression{Kind: "Identifier", Text: "FALSE"}
	found, err := product.checkFairGraph(neverSatisfied)
	if found {
		s.result = product.result
	}
	return found, err
}

func (s *search) lassoViolatesOrder(p temporalProperty, violation *Violation) (bool, error) {
	stage, err := s.orderedTraceStages(p, violation.Trace, 0)
	if err != nil || stage == len(p.sequence) {
		return false, err
	}
	for {
		next, err := s.orderedTraceStages(p, violation.Trace[*violation.LoopStart:], stage)
		if err != nil || next == len(p.sequence) {
			return false, err
		}
		if next == stage {
			return true, nil
		}
		stage = next
	}
}

func (s *search) orderedTraceStages(p temporalProperty, trace []TraceStep, stage int) (int, error) {
	for _, step := range trace {
		if err := s.model.ctx.Err(); err != nil {
			return stage, err
		}
		var err error
		stage, err = s.orderedStateStages(p, step.State, stage)
		if err != nil {
			return stage, err
		}
	}
	return stage, nil
}

func (s *search) orderedStateStages(p temporalProperty, state evaluator.State, stage int) (int, error) {
	for stage < len(p.sequence) {
		holds, err := s.model.holdsProperty(p.sequence[stage], state, p.locals)
		if err != nil {
			return stage, err
		}
		if !holds {
			break
		}
		stage++
	}
	return stage, nil
}
