package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
)

func (s *search) checkSafetyProperties(state evaluator.State, parent int, action string) (bool, error) {
	for _, p := range s.model.safety {
		if (parent < 0 && p.mode != initialOnly) || (parent >= 0 && p.mode != actionAlways) {
			continue
		}
		ok, err := s.holdsSafetyProperty(p, state, parent)
		if err != nil {
			return s.propertyRuntimeFailure(err, p.name, state, parent, action)
		}
		if !ok {
			trace := append(s.trace(parent), TraceStep{Action: action, State: state, ActionRange: actionSourceRange(s.actionSource)})
			s.result.Status = LivenessViolation
			s.result.Violation = &Violation{Name: p.name, Trace: trace}
			return false, nil
		}
	}
	return true, nil
}

func (s *search) holdsSafetyProperty(p temporalProperty, state evaluator.State, parent int) (bool, error) {
	if parent < 0 {
		return s.model.holdsProperty(p.predicate, state, p.locals)
	}
	oldLevel := s.model.tlcRuntime.level
	s.model.tlcRuntime.level = s.nodes[parent].depth
	defer func() { s.model.tlcRuntime.level = oldLevel }()
	return s.model.holdsActionProperty(p, s.nodes[parent].state, state)
}

func (m *model) holdsActionProperty(p temporalProperty, current, next evaluator.State) (bool, error) {
	context := evaluator.Context{Current: current, Next: next, Constants: m.constants, Locals: p.locals, Enabled: m.enabledCallback}
	result, err := m.program.Evaluate(p.predicate, context)
	if err != nil {
		return false, err
	}
	ok, valid := result.Bool()
	if !valid {
		return false, fmt.Errorf("action property must be boolean, got %s", result)
	}
	return ok, nil
}
