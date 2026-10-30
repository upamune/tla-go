package checker

import "github.com/upamune/tla-go/syntax/parser"

// Remove a duplicated initial state from a loop only when its temporal
// predicate is unchanged and state recurrence assumptions are independent of
// TLC runtime values. The original non-stuttering cycle is then preserved.
func (s *search) trimLevelLoop(result *Result) error {
	v := result.Violation
	if !s.canTrimLevelLoop(v) {
		return nil
	}
	last := len(v.Trace) - 1
	same, err := s.sameConcreteState(v.Trace[0].State, v.Trace[last].State)
	if err != nil || !same {
		return nil
	}
	p := s.model.livenessOrder[0]
	firstHolds, err := s.model.holdsProperty(p.predicate, v.Trace[0].State, p.locals)
	if err != nil || firstHolds {
		return err
	}
	lastHolds, err := s.model.holdsProperty(p.predicate, v.Trace[last].State, p.locals)
	if err != nil || lastHolds {
		return err
	}
	loop := 0
	v.LoopStart = &loop
	v.LoopAction = v.Trace[last].Action
	v.LoopActionRange = v.Trace[last].ActionRange
	v.Trace = v.Trace[:last]
	return nil
}

func (s *search) canTrimLevelLoop(v *Violation) bool {
	if !levelLoopShape(v) || len(s.model.livenessOrder) != 1 {
		return false
	}
	p := s.model.livenessOrder[0]
	if p.mode != eventually || s.model.expressionHasEffects(p.predicate, map[string]bool{}) {
		return false
	}
	return !s.model.stateRecurrenceUsesTLC()
}

func (m *model) stateRecurrenceUsesTLC() bool {
	for _, fair := range m.fairness {
		if m.expressionHasEffects(fair.action, map[string]bool{}) {
			return true
		}
		if fair.recurrence && !fair.recurrenceAction && m.expressionUsesTLC(fair.action, map[string]bool{}) {
			return true
		}
	}
	return false
}

func levelLoopShape(v *Violation) bool {
	return v != nil && v.LoopStart != nil && len(v.Trace) >= 3 && (*v.LoopStart == 1 || *v.LoopStart == 0 && v.LoopAction == "Stuttering")
}

func (m *model) expressionHasEffects(expr *parser.Expression, seen map[string]bool) bool {
	if expr == nil || m.isTLCDefer(expr) {
		return false
	}
	if isEffectfulBuiltin(expr) {
		return true
	}
	if m.definitionHasEffects(expr, seen) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = m.expressionHasEffects(child, seen)
		}
	})
	return found
}

func (m *model) definitionHasEffects(expr *parser.Expression, seen map[string]bool) bool {
	if expr.Kind != "Apply" && expr.Kind != "Identifier" {
		return false
	}
	d, ok := m.defs[expr.Text]
	if !ok || seen[expr.Text] {
		return false
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return m.expressionHasEffects(d.Body, seen)
}

func isEffectfulBuiltin(expr *parser.Expression) bool {
	name := tlcOperatorName(expr)
	if expr.Kind == "Identifier" && name == "JavaTime" {
		return true
	}
	return expr.Kind == "Apply" && (name == "Print" || name == "PrintT" || name == "TLCSet" || name == "AssertEq" || name == "TLCGetAndSet")
}
