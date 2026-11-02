package checker

import (
	"context"
	"errors"
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// TLC re-evaluates initial states with CallStackTool after a runtime failure.
// The diagnostic pass contributes to coverage but not the search statistics.
func (s *search) replayInitialCoverage(init *parser.Expression) error {
	m := s.model
	if m.coverage == nil || m.coverage.hasExistential || s.result.Status != EvaluationFailure {
		return nil
	}
	if m.initialCoverageReplayUsesRuntime(init) {
		return fmt.Errorf("-coverage cannot report initial error replay with mutable TLC or random operators")
	}
	s.publishInitialCoverageFailureHeader()
	runtime := m.tlcRuntime
	initial, level := runtime.initial, runtime.level
	current, candidate := runtime.traceCurrent, runtime.traceCandidate
	previousReplay := runtime.coverageInitReplay
	defer func() {
		runtime.initial, runtime.level = initial, level
		runtime.traceCurrent, runtime.traceCandidate = current, candidate
		runtime.coverageInitReplay = previousReplay
	}()
	runtime.initial, runtime.level, runtime.traceCurrent = 0, 0, -1
	runtime.coverageInitReplay = true
	gen := generator{model: m, initial: true, emit: s.replayInitialCoverageCandidate}
	_, err := gen.solve(&workList{item: work{expr: init}}, m.partialState(), "")
	if ctxErr := m.ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if coverageReplayControlError(err) {
		return err
	}
	// A changed outcome can arise from side effects. Until the different TLC
	// diagnostic result is modeled, do not emit potentially misleading costs.
	if err == nil {
		m.coverage.unreliableReplay = true
	}
	return nil
}

// Runtime reads, writes, and random operators can change both the failed
// candidate and the error during CallStackTool replay. Do not report the
// original diagnostic or partial costs as though replay were equivalent.
func (m *model) initialCoverageReplayUsesRuntime(init *parser.Expression) bool {
	if m.initialReplayRuntimeExpression(init, map[string]bool{}) {
		return true
	}
	for _, name := range append(append([]string(nil), m.cfg.Constraints...), m.cfg.Invariants...) {
		expr, err := m.definition(name)
		if err != nil || m.initialReplayRuntimeExpression(expr, map[string]bool{}) {
			return true
		}
	}
	return false
}

func (m *model) initialReplayRuntimeExpression(expr *parser.Expression, seen map[string]bool) bool {
	if expr == nil {
		return false
	}
	if m.initialReplayRuntimeDefinition(expr, seen) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = m.initialReplayRuntimeExpression(child, seen)
		}
	})
	return found
}

func (m *model) initialReplayRuntimeDefinition(expr *parser.Expression, seen map[string]bool) bool {
	if expr.Kind != "Apply" && expr.Kind != "Identifier" {
		return false
	}
	if initialReplayRuntimeOperator(tlcOperatorName(expr)) {
		return true
	}
	def, ok := m.defs[expr.Text]
	if !ok || seen[expr.Text] {
		return false
	}
	seen[expr.Text] = true
	return m.initialReplayRuntimeExpression(def.Body, seen)
}

func initialReplayRuntimeOperator(name string) bool {
	switch name {
	case "TLCGet", "TLCSet", "TLCGetOrDefault", "TLCGetAndSet", "JavaTime":
		return true
	default:
		return isRandomName(name)
	}
}

func coverageReplayControlError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errCoverageNamedConstraintRead)
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
