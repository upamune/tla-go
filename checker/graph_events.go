package checker

import (
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

// GraphEvent describes a state or transition emitted during state exploration.
// Initial events have no parent. New marks the first appearance of a state.
// View is the state as TLC prints it through VIEW when Options.ViewStates is
// set; fingerprints follow VIEW whenever one is configured.
type GraphEvent struct {
	State       evaluator.State
	Order       []string
	View        string
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
	return s.model.graph(GraphEvent{State: state, Order: s.graphOrder, View: s.graphView(state), Fingerprint: fingerprint, Parent: fingerprint, Depth: s.nodes[index].depth + 1, Stuttering: true})
}

func (s *search) graphRepresentative(state evaluator.State, key string) evaluator.State {
	if representative, ok := s.graphStates[key]; ok {
		return representative
	}
	return state
}

// checkGraphCandidate checks a new state and reports it to the state graph.
// Java TLC writes a new initial state before checking it, and a successor
// unless its check fails with an error.
func (s *search) checkGraphCandidate(state evaluator.State, parent int, action string, included bool) (bool, error) {
	if parent < 0 && included {
		if err := s.graphTransition(state, parent, action, true); err != nil {
			return false, err
		}
		return s.checkNewCandidate(state, parent, action)
	}
	more, err := s.checkNewCandidate(state, parent, action)
	if err != nil || !included {
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
	event := GraphEvent{State: state, Order: s.graphOrder, View: s.graphView(state), Fingerprint: fingerprint, Action: action, Depth: s.candidateLevel(parent), Initial: parent < 0, New: isNew}
	if parent >= 0 {
		event.Parent, err = s.graphFingerprint(s.nodes[parent].state)
		if err != nil {
			return err
		}
	}
	return s.model.graph(event)
}

func (s *search) graphFingerprint(state evaluator.State) (uint64, error) {
	values, err := s.graphFingerprintValues(state)
	if err != nil {
		return 0, err
	}
	rank := func(text string) (int, bool) { n, ok := s.model.valueRanks[text]; return n, ok }
	return s.model.fingerprintAlgorithm.Fingerprint(values, rank, s.model.compareOrderedValues)
}

// graphFingerprintValues follows TLCStateMut.fingerPrint, which fingerprints
// the value of VIEW instead of the variables when a VIEW is configured.
func (s *search) graphFingerprintValues(state evaluator.State) ([]value.Value, error) {
	if s.model.cfg.View != "" {
		expr, err := s.model.definition(s.model.cfg.View)
		if err != nil {
			return nil, err
		}
		v, err := s.model.eval(expr, state, nil)
		return []value.Value{v}, err
	}
	values := make([]value.Value, len(s.graphOrder))
	for i, name := range s.graphOrder {
		values[i] = state[name]
	}
	return values, nil
}

// graphView returns the text of a state under -view, which Java TLC's
// TLCStateMut.toString also uses for state dumps.
func (s *search) graphView(state evaluator.State) string {
	if !s.model.viewStates || s.model.cfg.View == "" {
		return ""
	}
	return s.model.stateView(state)
}
