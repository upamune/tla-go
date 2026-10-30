package checker

import (
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

// GraphEvent describes a state or transition emitted during state exploration.
// Initial events have no parent. New marks the first appearance of a state.
type GraphEvent struct {
	State       evaluator.State
	Order       []string
	Fingerprint uint64
	Parent      uint64
	Action      string
	Depth       int
	Initial     bool
	New         bool
	Stuttering  bool
}

func (s *search) graphStuttering(index int) error {
	if s.model.graph == nil || len(s.model.livenessOrder) == 0 {
		return nil
	}
	state := s.nodes[index].state
	fingerprint, err := s.graphFingerprint(state)
	if err != nil {
		return err
	}
	return s.model.graph(GraphEvent{State: state, Order: s.graphOrder, Fingerprint: fingerprint, Parent: fingerprint, Depth: s.nodes[index].depth + 1, Stuttering: true})
}

func (s *search) graphRepresentative(state evaluator.State, key string) evaluator.State {
	if representative, ok := s.graphStates[key]; ok {
		return representative
	}
	return state
}

func (s *search) checkGraphCandidate(state evaluator.State, parent int, action string, included bool) (bool, error) {
	more, err := s.checkNewCandidate(state, parent, action)
	if err != nil || !included || parent < 0 && !more {
		return more, err
	}
	if err := s.graphTransition(state, parent, action, true); err != nil {
		return false, err
	}
	return more, nil
}

func (s *search) graphTransition(state evaluator.State, parent int, action string, isNew bool) error {
	if s.model.graph == nil || parent < 0 && !isNew {
		return nil
	}
	fingerprint, err := s.graphFingerprint(state)
	if err != nil {
		return err
	}
	event := GraphEvent{State: state, Order: s.graphOrder, Fingerprint: fingerprint, Action: action, Depth: s.candidateLevel(parent), Initial: parent < 0, New: isNew}
	if parent >= 0 {
		event.Parent, err = s.graphFingerprint(s.nodes[parent].state)
		if err != nil {
			return err
		}
	}
	return s.model.graph(event)
}

func (s *search) graphFingerprint(state evaluator.State) (uint64, error) {
	values := make([]value.Value, len(s.graphOrder))
	for i, name := range s.graphOrder {
		values[i] = state[name]
	}
	rank := func(text string) (int, bool) { n, ok := s.model.valueRanks[text]; return n, ok }
	return value.TLCFingerprintWithOrdering(values, rank, s.model.compareOrderedValues)
}
