package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// TLC reconstructs paths and action labels before reevaluating properties.
// Replaying Init and Next preserves their effects on runtime registers.
func (s *search) replayLassoEffects(violation *Violation) error {
	if !s.model.lassoReplayHasEffects() {
		return nil
	}
	r := s.model.tlcRuntime
	previousGraph, previousLevel, previousCurrent := r.levelGraph, r.level, r.traceCurrent
	s.rememberLevelTrace(violation.Trace)
	r.levelGraph, r.level, r.traceCurrent = nil, 0, -1
	defer func() { r.levelGraph, r.level, r.traceCurrent = previousGraph, previousLevel, previousCurrent }()
	trace, err := s.replayLassoPath(violation.Trace)
	if err != nil {
		return err
	}
	violation.Trace = trace
	if s.replayedIdentityView() {
		r.viewTraceReplayed = true
	}
	if len(violation.Trace) < 2 {
		return nil
	}
	oldMuted := r.outputMuted
	r.outputMuted = true
	defer func() { r.outputMuted = oldMuted }()
	_, err = s.replayLassoPath(violation.Trace)
	return err
}

func (s *search) rememberLevelTrace(trace []TraceStep) {
	r := s.model.tlcRuntime
	if r.levelGraph == nil || r.search == nil {
		return
	}
	r.initTraceIndices()
	for _, step := range trace {
		if level, ok := r.lookupLevel(step.State); ok {
			r.rememberTraceState(step.State, level.index)
		}
	}
}

func (s *search) recoverFailureTrace(index int) ([]TraceStep, error) {
	trace := s.trace(index)
	if !s.model.hasReduction() || len(trace) < 2 {
		return trace, nil
	}
	r := s.model.tlcRuntime
	previousLevel := r.level
	r.level = 0
	defer func() { r.level = previousLevel }()
	recovered, err := s.replayLassoPath(trace)
	if err != nil {
		return nil, err
	}
	return recovered, nil
}

func (s *search) replayedIdentityView() bool {
	if !s.model.hasReduction() || s.model.tlcRuntime.search == nil {
		return false
	}
	original := s.model.tlcRuntime.search
	for _, edges := range original.edges {
		if original.validateGeneralEdges(edges) != nil {
			return false
		}
	}
	return true
}

func (m *model) lassoReplayHasEffects() bool {
	if !m.environmentIsImpure() {
		return false
	}
	for _, name := range []string{m.cfg.Init, m.cfg.Next} {
		expr, err := m.definition(name)
		if err == nil && (m.expressionHasEffects(expr, map[string]bool{}) || m.readsRandom(expr, map[string]bool{})) {
			return true
		}
	}
	return false
}

func (m *model) environmentIsImpure() bool {
	for _, definition := range m.defs {
		if m.replayConstantDefinition(definition) {
			continue
		}
		if m.expressionUsesTLC(definition.Body, map[string]bool{}) || m.replayExpressionHasEffects(definition.Body, map[string]bool{}) {
			return true
		}
	}
	return false
}

func (m *model) replayExpressionHasEffects(expr *parser.Expression, seen map[string]bool) bool {
	if expr == nil || m.isTLCDefer(expr) {
		return false
	}
	if isEffectfulBuiltin(expr) || expr.Kind == "Apply" && isRandomName(tlcOperatorName(expr)) {
		return true
	}
	if m.replayDefinitionHasEffects(expr, seen) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = m.replayExpressionHasEffects(child, seen)
		}
	})
	return found
}

func (m *model) replayDefinitionHasEffects(expr *parser.Expression, seen map[string]bool) bool {
	if expr.Kind != "Identifier" && expr.Kind != "Apply" {
		return false
	}
	definition, ok := m.defs[expr.Text]
	if !ok || seen[expr.Text] || m.replayConstantDefinition(definition) {
		return false
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return m.replayExpressionHasEffects(definition.Body, seen)
}

func (m *model) replayConstantDefinition(definition parser.Definition) bool {
	return definition.Kind == "Operator" && len(definition.Params) == 0 && !m.readsState(definition.Body, map[string]bool{}) && !m.hasAction(definition.Body, map[string]bool{})
}

func (s *search) replayLassoPath(trace []TraceStep) ([]TraceStep, error) {
	initial, err := s.replayInitialState(trace[0].State)
	if err != nil {
		return nil, err
	}
	recovered := append([]TraceStep(nil), trace...)
	recovered[0].State = initial
	for i := 1; i < len(trace); i++ {
		same, err := s.sameReplayState(trace[i-1].State, trace[i].State)
		if err != nil {
			return nil, err
		}
		if same {
			recovered[i].State, recovered[i].Action = recovered[i-1].State, "Stuttering"
			recovered[i].ActionRange = parser.Range{}
			continue
		}
		recovered[i], err = s.replaySuccessors(recovered[i-1].State, trace[i].State)
		if err != nil {
			return nil, err
		}
	}
	return recovered, nil
}

func (s *search) sameReplayState(a, b evaluator.State) (bool, error) {
	if !s.model.hasReduction() {
		return s.sameConcreteState(a, b)
	}
	left, err := s.model.storedViewKey(a)
	if err != nil {
		return false, err
	}
	right, err := s.model.storedViewKey(b)
	return left == right, err
}

func (s *search) replayInitialState(target evaluator.State) (evaluator.State, error) {
	init, err := s.model.definition(s.model.cfg.Init)
	if err != nil {
		return nil, err
	}
	var candidates []TraceStep
	gen := generator{model: s.model, initial: true, emit: func(state evaluator.State, _ string, _ *parser.Expression) (bool, error) {
		candidates = append(candidates, TraceStep{State: state})
		return true, nil
	}}
	if _, err := gen.solve([]work{{expr: init}}, evaluator.State{}, ""); err != nil {
		return nil, err
	}
	step, found, err := s.replayFindCandidate(candidates, target)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("Failed to recover the initial state from its fingerprint.")
	}
	return step.State, nil
}

func (s *search) replaySuccessors(current, target evaluator.State) (TraceStep, error) {
	next, err := s.model.definition(s.model.cfg.Next)
	if err != nil {
		return TraceStep{}, err
	}
	var candidates []TraceStep
	gen := generator{model: s.model, current: current, emit: func(state evaluator.State, action string, source *parser.Expression) (bool, error) {
		candidates = append(candidates, TraceStep{State: state, Action: action, ActionRange: actionSourceRange(source)})
		return true, nil
	}}
	if _, err := gen.solveNext(next); err != nil {
		return TraceStep{}, err
	}
	step, found, err := s.replayFindCandidate(candidates, target)
	if err != nil {
		return TraceStep{}, err
	}
	if !found {
		return TraceStep{}, fmt.Errorf("Failed to recover the next state from its fingerprint.")
	}
	return step, nil
}

func (s *search) replayFindCandidate(candidates []TraceStep, target evaluator.State) (TraceStep, bool, error) {
	for _, candidate := range candidates {
		same, err := s.sameReplayState(candidate.State, target)
		if err != nil || same {
			return candidate, same, err
		}
	}
	return TraceStep{}, false, nil
}
