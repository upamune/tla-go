package checker

import (
	"github.com/upamune/tla-go/evaluator"
)

// GraphEvent describes a state or transition emitted during state exploration.
// Initial events have no parent. New marks the first appearance of a state.
// View is the state as TLC prints it through VIEW when Options.ViewStates is
// set; fingerprints follow VIEW whenever one is configured, except in text
// dump mode where no graph fingerprints are computed.
type GraphEvent struct {
	State       evaluator.State
	Order       []string
	View        string
	Fingerprint uint64
	// RankFingerprint is the separately evaluated DOT rank node.
	RankFingerprint uint64
	Parent          uint64
	Action          string
	Depth           int
	Initial         bool
	New             bool
	Stuttering      bool
}

func (s *search) graphStuttering(index int) error {
	if s.model.graph == nil || len(s.model.livenessOrder) == 0 {
		return nil
	}
	state := s.nodes[index].state
	if s.model.textGraph {
		return s.model.graph(GraphEvent{State: state, Order: s.graphOrder, View: s.graphView(state), Depth: s.nodes[index].depth + 1, Stuttering: true})
	}
	fingerprint, err := s.graphFingerprint(state)
	if err != nil {
		return err
	}
	if !s.model.dotGraph {
		return s.model.graph(GraphEvent{State: state, Order: s.graphOrder, View: s.graphView(state), Fingerprint: fingerprint, Parent: fingerprint, Depth: s.nodes[index].depth + 1, Stuttering: true})
	}
	parent, err := s.graphFingerprint(state)
	if err != nil {
		return err
	}
	rank, err := s.graphFingerprint(state)
	if err != nil {
		return err
	}
	return s.model.graph(GraphEvent{State: state, Order: s.graphOrder, Fingerprint: fingerprint, Parent: parent, RankFingerprint: rank, Depth: s.nodes[index].depth + 1, Stuttering: true})
}

func (s *search) graphRepresentative(state evaluator.State, key string) evaluator.State {
	if s.model.dotGraph {
		return state
	}
	if representative, ok := s.graphStates[key]; ok {
		return representative
	}
	return state
}

// checkGraphCandidate checks a new state and reports it to the state graph.
// Java TLC writes included states before checking their properties. Generic
// OnGraph observers retain their existing successor notification order.
func (s *search) checkGraphCandidate(state evaluator.State, parent int, action string, included bool) (bool, error) {
	if included && s.graphBeforeCheck(parent) {
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

func (s *search) graphBeforeCheck(parent int) bool {
	return parent < 0 || s.model.dotGraph || s.model.textGraph
}

func (s *search) graphTransition(state evaluator.State, parent int, action string, isNew bool) error {
	if s.model.graph == nil || parent < 0 && !isNew {
		return nil
	}
	if s.model.textGraph {
		if !isNew {
			return nil
		}
		return s.model.graph(GraphEvent{State: state, Order: s.graphOrder, View: s.graphView(state), Action: action, Depth: s.candidateLevel(parent), Initial: parent < 0, New: true})
	}
	if s.model.dotGraph {
		return s.graphDOTTransition(state, parent, action, isNew)
	}
	return s.graphGenericTransition(state, parent, action, isNew)
}

func (s *search) graphGenericTransition(state evaluator.State, parent int, action string, isNew bool) error {
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

func (s *search) graphDOTTransition(state evaluator.State, parent int, action string, isNew bool) error {
	fingerprint, err := s.graphFingerprint(state)
	if err != nil {
		return err
	}
	event := GraphEvent{State: state, Order: s.graphOrder, Fingerprint: fingerprint, Action: action, Depth: s.candidateLevel(parent), Initial: parent < 0, New: isNew}
	rankState := state
	if parent >= 0 {
		rankState = s.nodes[parent].state
		event.Parent, err = s.graphFingerprint(rankState)
		if err != nil {
			return err
		}
	}
	if isNew {
		event.View = s.graphView(state)
	}
	event.RankFingerprint, err = s.graphFingerprint(rankState)
	if err != nil {
		return err
	}
	return s.model.graph(event)
}

func (s *search) graphFingerprint(state evaluator.State) (uint64, error) {
	return s.model.javaFingerprint(state, s.graphOrder)
}

// graphView returns the text of a state under -view, which Java TLC's
// TLCStateMut.toString also uses for state dumps.
func (s *search) graphView(state evaluator.State) string {
	if !s.model.viewStates || s.model.cfg.View == "" {
		return ""
	}
	return s.model.stateView(state)
}
