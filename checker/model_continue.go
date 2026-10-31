package checker

import "github.com/upamune/tla-go/evaluator"

func (m *model) configureContinuation(options Options) {
	if options.Simulation == nil {
		m.continueChecking, m.onContinuedViolation = options.Continue, options.OnViolation
	}
}

// In TLC 1.7.4, the model-checking worker wraps evaluation failures in a
// RuntimeException when continuation is enabled, producing a general error.
func (m *model) adjustContinuedEvaluationFailure(result *Result) {
	if m.continueChecking && result.Failure != nil &&
		(result.Failure.ToolCode == 2111 || result.Failure.ToolCode == 2113) {
		result.Failure.ExitCode = 255
	}
}

func (s *search) continueInitialInvariants(parent int) bool {
	return s.model.continueChecking && parent < 0
}

func (s *search) continueInvariantViolation() bool {
	if !s.model.continueChecking || s.result.Status != InvariantViolation || s.result.Failure != nil {
		return false
	}
	s.emitContinuedViolation()
	return true
}

func (s *search) checkSafetyContinuing(state evaluator.State, parent int, action string) (bool, error) {
	more, err := s.checkSafetyProperties(state, parent, action)
	if err != nil || more || !s.continueActionViolation(parent) {
		return more, err
	}
	return true, nil
}

func (s *search) continueActionViolation(parent int) bool {
	if !s.model.continueChecking || parent < 0 || s.result.Status != LivenessViolation || s.result.Failure != nil {
		return false
	}
	s.emitContinuedViolation()
	return true
}

func (s *search) emitContinuedViolation() {
	s.result.ContinuedViolations++
	if s.model.onContinuedViolation != nil {
		if s.model.cfg.Alias != "" {
			s.model.applyAlias(&s.result)
		}
		s.model.applyTraceOrder(&s.result)
		s.model.applyActionRanges(&s.result)
		s.model.onContinuedViolation(s.result)
	}
	s.result.Status = Success
	s.result.Violation = nil
}

func (s *search) checkInitialInvariantsContinuing(state evaluator.State, action string) (bool, error) {
	for _, name := range s.model.cfg.Invariants {
		if s.model.trueConfigured[name] {
			continue
		}
		expr, _ := s.model.definition(name)
		ok, err := s.model.holds(expr, state, nil)
		if err != nil {
			return s.invariantRuntimeFailure(err, name, state, -1, action)
		}
		if !ok {
			s.emitInitialInvariantViolation(name, state, action)
		}
	}
	return s.checkInitialStateSafetyContinuing(state, action)
}

func (s *search) checkInitialStateSafetyContinuing(state evaluator.State, action string) (bool, error) {
	for _, property := range s.model.safety {
		if property.mode != stateAlways {
			continue
		}
		name := s.model.safetyViolationName(property)
		ok, err := s.model.holdsProperty(property.predicate, state, property.locals)
		if err != nil {
			return s.invariantRuntimeFailure(err, name, state, -1, action)
		}
		if !ok {
			s.emitInitialInvariantViolation(name, state, action)
		}
	}
	return true, nil
}

func (s *search) emitInitialInvariantViolation(name string, state evaluator.State, action string) {
	s.result.Status = InvariantViolation
	s.result.Violation = &Violation{Name: name, Trace: []TraceStep{{Action: action, State: state, ActionRange: actionSourceRange(s.actionSource)}}}
	s.emitContinuedViolation()
}
