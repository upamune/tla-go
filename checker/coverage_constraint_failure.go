package checker

import (
	"errors"
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

var errCoverageConstraintReplay = errors.New("-coverage cannot report this successor constraint error replay")

// TLC repeats the failing action to obtain a call stack. Restrict replay to
// effect-free actions and constraints until observable effects can be ordered.
func (s *search) replayCoverageCandidateFailure(cause error, index int) error {
	m := s.model
	failure, ok := asRuntimeError(cause)
	if !ok || failure.ToolCode != 2145 || !m.coverageConstraintReplaySafe() {
		return errCoverageConstraintReplay
	}
	err := s.replayCoverageConstraintAction(index)
	if replay, ok := asRuntimeError(err); ok && replay.ToolCode == failure.ToolCode && replay.Message == failure.Message {
		return nil
	}
	if s.model.ctx.Err() != nil {
		return s.model.ctx.Err()
	}
	return errCoverageConstraintReplay
}

func (s *search) replayCoverageConstraintAction(index int) error {
	m := s.model
	next, err := m.definition(m.cfg.Next)
	if err != nil {
		return err
	}
	previous := m.tlcRuntime.traceCurrent
	m.tlcRuntime.traceCurrent = index
	defer func() { m.tlcRuntime.traceCurrent = previous }()
	var pending []coverageFailureCandidate
	flush := func() (bool, error) {
		return s.replayCoverageConstraintBatch(index, &pending)
	}
	gen := generator{model: m, current: s.nodes[index].state, actionStart: flush, emit: func(state evaluator.State, action string, source *parser.Expression) (bool, error) {
		pending = append(pending, coverageFailureCandidate{state: state, action: action, source: source})
		return true, nil
	}}
	more, err := gen.solveNext(next, m.partialState())
	if err == nil && more {
		_, err = flush()
	}
	return err
}

func (m *model) coverageConstraintReplaySafe() bool {
	if !m.coverageConstraintReplayConfiguration() {
		return false
	}
	for _, name := range append([]string{m.cfg.Next}, m.cfg.Constraints...) {
		if !m.coverageConstraintReplayExpression(name) {
			return false
		}
	}
	return true
}

func (m *model) coverageConstraintReplayConfiguration() bool {
	return !m.coverage.hasExistential && !m.hasReduction() && !m.tlcRuntime.options.ModernTLCStatistics && len(m.cfg.ActionConstraints)+len(m.cfg.Possible) == 0
}

func (m *model) coverageConstraintReplayExpression(name string) bool {
	expr, err := m.definition(name)
	if err != nil || m.expressionHasEffects(expr, map[string]bool{}) || m.readsRandom(expr, map[string]bool{}) {
		return false
	}
	if name == m.cfg.Next {
		return !m.expressionUsesTLC(expr, map[string]bool{}) && !m.initialReplayRuntimeExpression(expr, map[string]bool{})
	}
	return m.coverageConstraintStableReads(expr, map[string]bool{})
}

// Only the two constant keys known to be undefined in 1.7.4 are replayed.
// Other TLCGet values can depend on time, counters, or mutable registers.
func (m *model) coverageConstraintStableReads(expr *parser.Expression, seen map[string]bool) bool {
	if !coverageConstraintStableNode(expr) {
		return false
	}
	if expr.Kind == "Apply" && tlcOperatorName(expr) == "TLCGet" {
		return m.initialReplayUndefinedStatistics(expr)
	}
	if (expr.Kind == "Apply" || expr.Kind == "Identifier") && initialReplayRuntimeOperator(tlcOperatorName(expr)) {
		return false
	}
	if !m.coverageConstraintStableDefinition(expr, seen) {
		return false
	}
	ok := true
	walkChildren(expr, func(child *parser.Expression) {
		if ok {
			ok = m.coverageConstraintStableReads(child, seen)
		}
	})
	return ok
}

func coverageConstraintStableNode(expr *parser.Expression) bool {
	if expr == nil || len(expr.Selectors)+len(expr.Instances)+len(expr.EnabledBindings)+len(expr.EnabledScopes)+len(expr.InstanceScopes)+len(expr.Definitions) != 0 {
		return false
	}
	return expr.Kind != "Identifier" || !coverageEffectfulReference(tlcOperatorName(expr))
}

func coverageEffectfulReference(name string) bool {
	switch name {
	case "TLCGet", "TLCSet", "TLCGetOrDefault", "TLCGetAndSet", "Print", "PrintT", "AssertEq", "JavaTime":
		return true
	default:
		return isRandomName(name)
	}
}

func (m *model) coverageConstraintStableDefinition(expr *parser.Expression, seen map[string]bool) bool {
	definition, found := m.defs[expr.Text]
	if !found || expr.Kind != "Apply" && expr.Kind != "Identifier" {
		return true
	}
	if seen[expr.Text] {
		return false
	}
	for _, param := range definition.Params {
		if param.Arity != 0 {
			return false
		}
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return m.coverageConstraintStableReads(definition.Body, seen)
}

func (s *search) replayCoverageConstraintBatch(index int, pending *[]coverageFailureCandidate) (bool, error) {
	batch := *pending
	*pending = nil
	for _, candidate := range batch {
		action := candidate.action
		if action == "" {
			action = s.model.cfg.Next
		}
		s.model.coverage.generated(action, false)
	}
	if len(batch) == 0 {
		return true, nil
	}
	_, err := s.satisfiesConstraints(batch[0].state, index)
	if err == nil {
		return false, fmt.Errorf("%w: first candidate did not fail", errCoverageConstraintReplay)
	}
	return false, err
}
