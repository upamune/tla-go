package checker

import "github.com/upamune/tla-go/syntax/parser"

// P ~> []Q fails when P occurs and Q is false infinitely often afterwards.
func (m *model) classifyLeadsToStaying(p temporalProperty, expr *parser.Expression) (temporalProperty, bool) {
	if !isLeadsTo(expr) || !m.isStateExpression(expr.Args[0]) {
		return p, false
	}
	goal, ok := m.temporalAlias(expr.Args[1])
	if !ok || goal.Kind != "Always" || len(goal.Args) != 1 || !m.isStateExpression(goal.Args[0]) {
		return p, false
	}
	p.trigger, p.predicate, p.mode = expr.Args[0], goal.Args[0], leadsToStaying
	return p, true
}

func (s *search) lassoViolatesLeadsToStaying(p temporalProperty, v *Violation) (bool, error) {
	triggered, err := s.lassoContainsState(p.trigger, p, v.Trace)
	if err != nil || !triggered {
		return false, err
	}
	return s.lassoContainsBadState(p, v.Trace[*v.LoopStart:])
}

func (s *search) lassoContainsBadState(p temporalProperty, trace []TraceStep) (bool, error) {
	for _, step := range trace {
		if err := s.model.ctx.Err(); err != nil {
			return false, err
		}
		ok, err := s.model.holdsProperty(p.predicate, step.State, p.locals)
		if err != nil || !ok {
			return !ok, err
		}
	}
	return false, nil
}

func (s *search) lassoContainsState(expr *parser.Expression, p temporalProperty, trace []TraceStep) (bool, error) {
	for _, step := range trace {
		if err := s.model.ctx.Err(); err != nil {
			return false, err
		}
		ok, err := s.model.holdsProperty(expr, step.State, p.locals)
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}
