package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// TLC reconstructs safety traces from initial states and successive actions.
// These evaluations also update the original cost model. The supported
// coverage action forms are pure; existential actions retain their existing
// fail-closed handling because their recovery costs are not yet modeled.
func (m *model) replayCoverageSafetyTrace(result Result) error {
	if !m.coverageSafetyReplayNeeded(result) {
		return nil
	}
	if m.lassoReplayHasEffects() || m.tlcRuntime.search == nil {
		return fmt.Errorf("-coverage cannot report an effectful or unavailable safety trace recovery")
	}
	return m.tlcRuntime.search.coverageSafetyTrace(result)
}

func (m *model) coverageSafetyReplayNeeded(result Result) bool {
	return m.coverage != nil && !m.coverage.hasExistential && result.Violation != nil && len(result.Violation.Trace) >= 2 && (result.Status == Deadlock || result.Status == InvariantViolation)
}

func (s *search) coverageSafetyTrace(result Result) error {
	trace := result.Violation.Trace
	// An invariant failure on the successor of an initial state uses the
	// original initial state directly. All other noninitial paths recover it.
	if result.Status == Deadlock || len(trace) > 2 {
		if err := s.coverageReplayInitial(trace[0].State); err != nil {
			return err
		}
	}
	for i := 1; i < len(trace); i++ {
		if err := s.coverageReplaySuccessor(trace[i-1].State, trace[i].State); err != nil {
			return err
		}
	}
	return nil
}

func (s *search) coverageReplayInitial(target evaluator.State) error {
	init, err := s.model.definition(s.model.cfg.Init)
	if err != nil {
		return err
	}
	found := false
	gen := generator{model: s.model, initial: true, borrowComplete: true, emit: func(state evaluator.State, action string, _ *parser.Expression) (bool, error) {
		s.model.coverage.generated(action, true)
		s.model.coverage.distinct(action, true)
		if !found {
			found, err = s.sameConcreteState(state, target)
		}
		return true, err
	}}
	partial := s.model.takePartial()
	_, err = gen.solve(&workList{item: work{expr: init}}, partial, "")
	s.model.returnPartial(partial)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("-coverage failed to recover the initial state")
	}
	return nil
}

func (s *search) coverageReplaySuccessor(current, target evaluator.State) error {
	next, err := s.model.definition(s.model.cfg.Next)
	if err != nil {
		return err
	}
	found := false
	gen := generator{model: s.model, current: current, borrowComplete: true, emit: func(state evaluator.State, action string, _ *parser.Expression) (bool, error) {
		s.model.coverage.generated(action, false)
		if !found {
			found, err = s.sameConcreteState(state, target)
		}
		return true, err
	}, actionStart: func() (bool, error) { return !found, nil }}
	partial := s.model.takePartial()
	_, err = gen.solveNext(next, partial)
	s.model.returnPartial(partial)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("-coverage failed to recover the next state")
	}
	return nil
}
