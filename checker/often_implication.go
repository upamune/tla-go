package checker

import "github.com/upamune/tla-go/syntax/parser"

// []<>P => []<>Q fails on a fair cycle that visits P and never visits Q.
func (m *model) classifyOftenImplication(p temporalProperty, expr *parser.Expression) (temporalProperty, bool) {
	if expr.Kind != "Apply" || expr.Text != "=>" || len(expr.Args) != 2 {
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
	trigger, action, ok := m.oftenAntecedent(left)
	if !ok {
		return p, false
	}
	p.trigger = trigger
	p.triggerAction = action
	return m.classifyOftenConsequence(p, right)
}

func (m *model) oftenAntecedent(expr *parser.Expression) (*parser.Expression, bool, bool) {
	if isOftenAction(expr) {
		action := expr.Args[0].Args[0]
		valid := !m.hasTemporal(action, map[string]bool{}) && !m.hasAction(action.Args[1], map[string]bool{})
		return action, true, valid
	}
	trigger, mode := temporalPredicate(expr)
	return trigger, false, trigger != nil && mode == often && m.isStateExpression(trigger)
}

func (m *model) classifyOftenConsequence(p temporalProperty, expr *parser.Expression) (temporalProperty, bool) {
	if isOftenAction(expr) {
		p.predicate, p.mode = expr.Args[0].Args[0], oftenImpliesAction
		return p, !m.hasTemporal(p.predicate, map[string]bool{}) && !m.hasAction(p.predicate.Args[1], map[string]bool{})
	}
	goal, mode := temporalPredicate(expr)
	if goal == nil || mode != often || !m.isStateExpression(goal) {
		return p, false
	}
	p.predicate, p.mode = goal, oftenImpliesState
	return p, true
}

func (s *search) checkOftenImplication(p temporalProperty) (bool, error) {
	return s.checkRecurrenceGoal(p, p.trigger, p.triggerAction)
}

func (s *search) checkRecurrenceGoal(p temporalProperty, required *parser.Expression, requiredAction bool) (bool, error) {
	target := s
	check := p
	check.mode = often
	if p.mode == oftenImpliesAction || p.mode == staysOrOftenAction {
		var err error
		target, err = s.withoutPropertyEdges(p)
		if err != nil {
			return false, err
		}
		check.mode = actionOften
	}
	inverted := p.mode == staysOrOftenState || p.mode == staysOrOftenAction
	found, err := target.checkFairGraphRequired(check, required, requiredAction, inverted)
	if found {
		s.result = target.result
	}
	return found, err
}

func (s *search) lassoViolatesOftenImplication(p temporalProperty, v *Violation) (bool, error) {
	hasTrigger, err := s.lassoHasTrigger(p, v)
	if err != nil || !hasTrigger {
		return false, err
	}
	return s.lassoMissesRecurrenceGoal(p, v)
}

func (s *search) lassoMissesRecurrenceGoal(p temporalProperty, v *Violation) (bool, error) {
	if p.mode == oftenImpliesAction || p.mode == staysOrOftenAction {
		return s.lassoViolatesAction(p, v)
	}
	p.mode = often
	return s.lassoViolatesState(p, v)
}

func (s *search) lassoHasTrigger(p temporalProperty, v *Violation) (bool, error) {
	for i := *v.LoopStart; i < len(v.Trace); i++ {
		if err := s.model.ctx.Err(); err != nil {
			return false, err
		}
		ok, err := s.lassoTriggerAt(p, v, i)
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

func (s *search) lassoTriggerAt(p temporalProperty, v *Violation, index int) (bool, error) {
	if p.triggerAction {
		return s.holdsLassoAction(p.trigger, p.locals, v, index)
	}
	return s.model.holdsBaseProperty(p.trigger, v.Trace[index].State, p.locals)
}
