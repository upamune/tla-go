package checker

import "github.com/upamune/tla-go/evaluator"

// A tableau can visit several internal nodes for one state. TLC removes
// consecutive repetitions of that state before recovering the trace.
func (s *search) collapseGeneralTrace(v *Violation) error {
	if !s.shouldCollapseGeneralTrace(v) {
		return nil
	}
	trace := make([]TraceStep, 0, len(v.Trace))
	loop := 0
	previous := ""
	for i, step := range v.Trace {
		key, err := s.lassoStateKey(step.State)
		if err != nil {
			return err
		}
		if i == 0 || key != previous {
			trace = append(trace, step)
		}
		if i == *v.LoopStart {
			loop = len(trace) - 1
		}
		previous = key
	}
	v.Trace, v.LoopStart = trace, &loop
	return nil
}

func (s *search) lassoStateKey(state evaluator.State) (string, error) {
	if s.model.hasReduction() {
		return s.model.storedViewKey(state)
	}
	return s.model.stateKey(state)
}

func (s *search) shouldCollapseGeneralTrace(v *Violation) bool {
	for _, property := range s.model.livenessOrder {
		if property.name == v.Name && property.mode == generalTemporal {
			if !s.model.hasReduction() {
				return true
			}
			return s.supportsReducedStateFormula(property)
		}
	}
	return false
}
