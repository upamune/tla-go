package checker

import (
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// TLC re-evaluates initial states with CallStackTool after a runtime failure.
// The diagnostic pass contributes to coverage but not the search statistics.
func (s *search) replayInitialCoverage(init *parser.Expression) {
	m := s.model
	if m.coverage == nil || m.coverage.hasExistential || s.result.Status != EvaluationFailure {
		return
	}
	s.publishInitialCoverageFailureHeader()
	runtime := m.tlcRuntime
	initial, level := runtime.initial, runtime.level
	current, candidate := runtime.traceCurrent, runtime.traceCandidate
	defer func() {
		runtime.initial, runtime.level = initial, level
		runtime.traceCurrent, runtime.traceCandidate = current, candidate
	}()
	runtime.initial, runtime.level, runtime.traceCurrent = 0, 0, -1
	gen := generator{model: m, initial: true, emit: s.replayInitialCoverageCandidate}
	_, err := gen.solve(&workList{item: work{expr: init}}, m.partialState(), "")
	// A changed outcome can arise from side effects. Until the different TLC
	// diagnostic result is modeled, do not emit potentially misleading costs.
	if err == nil {
		m.coverage.unreliableReplay = true
	}
}

func (s *search) publishInitialCoverageFailureHeader() {
	if s.model.onFailureHeader == nil {
		return
	}
	s.model.presentTraces(&s.result)
	failure := *s.result.Failure
	failure.EvaluationDetail = ""
	s.model.onFailureHeader(failure)
	s.result.Failure.HeaderReported = true
}

func (s *search) replayInitialCoverageCandidate(state evaluator.State, action string, _ *parser.Expression) (bool, error) {
	m := s.model
	m.coverage.generated(action, true)
	m.coverage.distinct(action, true)
	s.countInitialState()
	m.tlcRuntime.traceCandidate = state
	if _, err := m.constraintsHold(m.cfg.Constraints, state, nil); err != nil {
		return false, err
	}
	name, err := m.invariantViolation(state)
	return name == "" && err == nil, err
}
