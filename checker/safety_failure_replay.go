package checker

import (
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

func (s *search) finishFailedExpansion(index int, next *parser.Expression) (Result, error) {
	if err := s.replaySafetyFailure(index, next); err != nil {
		return Result{}, err
	}
	return s.result, nil
}

func (s *search) recordFailureCandidate(key string, parent int, inModel bool) {
	if parent >= 0 && inModel && s.result.Failure != nil {
		s.failureCandidateKey = key
	}
}

// TLC reevaluates successors after an invariant or action-property evaluation error to collect a
// call stack. Its fingerprint set and state queue are shared with this replay,
// but the replay worker's generated-state counter is not included in totals.
func (s *search) replaySafetyFailure(index int, next *parser.Expression) error {
	failure := s.result.Failure
	if failure == nil || failure.ToolCode != 2111 && failure.ToolCode != 2113 {
		return nil
	}
	seen := s.visited
	if s.candidateInModel {
		seen[s.failureCandidateKey] = true
	}
	gen := generator{model: s.model, current: s.nodes[index].state, emit: func(state evaluator.State, action string, source *parser.Expression) (bool, error) {
		return s.replaySafetyCandidate(index, state, action, source, seen)
	}}
	_, err := gen.solve(&workList{item: work{expr: next, label: true}}, s.model.partialState(), s.model.cfg.Next)
	if err == nil {
		return nil
	}
	if replay, ok := asRuntimeError(err); ok {
		failure.ExitCode = actionFailureCode(replay)
		failure.EvaluationDetail = evaluationDetail(replay)
		failure.SuppressDepth = true
		return nil
	}
	return err
}

func (s *search) replaySafetyCandidate(parent int, state evaluator.State, action string, source *parser.Expression, seen map[string]bool) (bool, error) {
	key, inModel, err := s.candidateKeyAndConstraints(state, parent)
	if err != nil {
		return false, err
	}
	unseen := s.markReplayCandidate(key, inModel, seen)
	if err := s.replayGraphCandidate(parent, state, action, source, key, inModel, unseen); err != nil {
		return false, err
	}
	if more, err := s.replayCandidateInvariant(state, unseen); err != nil || !more {
		return more, err
	}
	if more, err := s.replayActionProperties(parent, state); err != nil || !more {
		return more, err
	}
	s.queueReplayCandidate(inModel, unseen)
	return true, nil
}

func (s *search) queueReplayCandidate(inModel, unseen bool) {
	if inModel && unseen {
		s.result.QueuedStates++
	}
}

func (s *search) replayGraphCandidate(parent int, state evaluator.State, action string, source *parser.Expression, key string, inModel, unseen bool) error {
	if !inModel || s.model.graph == nil {
		return nil
	}
	if action == "" {
		action = s.model.cfg.Next
	}
	previous := s.actionSource
	s.actionSource = source
	defer func() { s.actionSource = previous }()
	return s.graphTransition(s.graphRepresentative(state, key), parent, action, unseen)
}

func (s *search) markReplayCandidate(key string, inModel bool, seen map[string]bool) bool {
	if !inModel {
		return true
	}
	if seen[key] {
		return false
	}
	seen[key] = true
	s.result.DistinctStates++
	return true
}

func (s *search) replayCandidateInvariant(state evaluator.State, unseen bool) (bool, error) {
	if !unseen {
		return true, nil
	}
	name, err := s.violation(state)
	return name == "", err
}

func (s *search) replayActionProperties(parent int, state evaluator.State) (bool, error) {
	for _, property := range s.model.safety {
		if property.mode == actionAlways {
			ok, err := s.holdsSafetyProperty(property, state, parent)
			if err != nil || !ok {
				return ok, err
			}
		}
	}
	return true, nil
}
