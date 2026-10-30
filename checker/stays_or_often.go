package checker

import "github.com/upamune/tla-go/syntax/parser"

// <>[]P \/ []<>Q fails only when P fails infinitely often and Q eventually stops.
func (m *model) classifyStaysOrOften(p temporalProperty, expr *parser.Expression) (temporalProperty, bool) {
	if expr.Kind != "Or" || len(expr.Args) != 2 {
		return p, false
	}
	left, ok := m.temporalAlias(expr.Args[0])
	if !ok {
		return p, false
	}
	right, ok := m.temporalAlias(expr.Args[1])
	if !ok {
		return p, false
	}
	if result, ok := m.staysOrOftenParts(p, left, right); ok {
		return result, true
	}
	return m.staysOrOftenParts(p, right, left)
}

func (m *model) staysOrOftenParts(p temporalProperty, stays, recurrence *parser.Expression) (temporalProperty, bool) {
	predicate, mode := temporalPredicate(stays)
	if predicate == nil || mode != eventuallyAlways || !m.isStateExpression(predicate) {
		return p, false
	}
	p.trigger = predicate
	p, ok := m.classifyOftenConsequence(p, recurrence)
	if !ok {
		return p, false
	}
	if p.mode == oftenImpliesAction {
		p.mode = staysOrOftenAction
	} else {
		p.mode = staysOrOftenState
	}
	return p, true
}

func (s *search) checkStaysOrOften(p temporalProperty) (bool, error) {
	return s.checkRecurrenceGoal(p, p.trigger, false)
}

func (s *search) lassoViolatesStaysOrOften(p temporalProperty, v *Violation) (bool, error) {
	required := p
	required.predicate = p.trigger
	bad, err := s.lassoContainsBadState(required, v.Trace[*v.LoopStart:])
	if err != nil || !bad {
		return false, err
	}
	return s.lassoMissesRecurrenceGoal(p, v)
}
