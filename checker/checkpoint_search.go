package checker

import (
	"fmt"
	"maps"

	"github.com/upamune/tla-go/syntax/parser"
)

// checkpointSupported deliberately recognizes a closed language. Identity is
// supplied by the caller; this does not establish dependency-aware identity.
func (m *model) checkpointSupported() error {
	c := m.cfg
	if m.checkpointUnsupportedFeatures() || m.checkpointUnsupportedOptions() {
		return fmt.Errorf("unsupported checkpoint model or options")
	}
	for _, name := range append([]string{c.Init, c.Next}, c.Invariants...) {
		expr, err := m.definition(name)
		if err != nil || !m.checkpointExpression(expr) {
			return fmt.Errorf("unsupported checkpoint expression %q", name)
		}
	}
	return nil
}

func (m *model) checkpointUnsupportedFeatures() bool {
	c := m.cfg
	return m.parallelModules() || m.fromSpecification || m.hasReduction() || m.parallelCallbacks() || m.viewStates || len(m.module.Assumptions) != 0 || len(c.Constraints)+len(c.ActionConstraints)+len(c.Properties)+len(c.Possible)+len(c.Postconditions)+len(c.Constants)+len(c.Substitutions) != 0 || m.checkpointUnsupportedConfig()
}

func (m *model) checkpointUnsupportedConfig() bool {
	c := m.cfg
	return c.Alias != "" || c.Periodic != "" || c.RLReward != ""
}

func (m *model) checkpointUnsupportedOptions() bool {
	o := m.tlcRuntime.options
	return m.searchEvent != nil || m.warning != nil || m.livenessWarning != nil || o.OnPrint != nil || o.OnPause != nil || o.ReadFile != nil || o.WriteFile != nil || m.checkpointUnsupportedMode()
}

func (m *model) checkpointUnsupportedMode() bool {
	o := m.tlcRuntime.options
	return o.Simulation != nil || o.DepthFirst != nil || (o.Workers > 1 && !o.SerialExploration)
}

func (m *model) checkpointExpression(e *parser.Expression) bool {
	if e == nil || parallelExtraChildren(e) {
		return false
	}
	if checkpointSetMembership(e) {
		return m.checkpointExpression(e.Args[0]) && m.checkpointSetElements(e.Args[1])
	}
	if !m.parallelExpressionKind(e) {
		return false
	}
	for _, arg := range e.Args {
		if !m.checkpointExpression(arg) {
			return false
		}
	}
	return true
}

func (m *model) checkpointSetElements(e *parser.Expression) bool {
	for _, item := range e.Args {
		if !m.checkpointExpression(item) {
			return false
		}
	}
	return true
}

func checkpointSetMembership(e *parser.Expression) bool {
	return e.Kind == "Apply" && e.Text == "\\in" && len(e.Args) == 2 && e.Args[1].Kind == "SetEnumeration"
}

func (s *search) checkpoint(identity checkpointIdentity) (checkpointSnapshot, error) {
	var snap checkpointSnapshot
	if err := s.model.checkpointSupported(); err != nil {
		return snap, err
	}
	if !s.initialComplete || !s.checkpointHealthy() {
		return snap, fmt.Errorf("search is not at a resumable boundary")
	}
	snap = checkpointSnapshot{Identity: identity, Head: s.head, InitialComplete: true, Visited: maps.Clone(s.visited), Counters: checkpointCounters{Generated: s.result.GeneratedStates, Distinct: s.result.DistinctStates, Queued: s.result.QueuedStates, Depth: s.result.Depth, Initial: s.model.tlcRuntime.initial, OutDegrees: s.outDegrees}}
	for _, n := range s.nodes {
		snap.Nodes = append(snap.Nodes, checkpointNode{State: maps.Clone(n.state), Parent: n.parent, Action: n.action, ActionRange: actionSourceRange(n.source), Depth: n.depth})
	}
	if err := s.model.validateSearchCheckpoint(snap); err != nil {
		return checkpointSnapshot{}, err
	}
	return snap, nil
}

func (s *search) checkpointHealthy() bool {
	return s.result.Status == Success && s.result.Failure == nil && s.result.Violation == nil && !s.model.tlcRuntime.stopped
}

func (m *model) validateSearchCheckpoint(s checkpointSnapshot) error {
	if err := validateSearchCheckpointStructure(s); err != nil {
		return err
	}
	keys := make(map[string]bool, len(s.Nodes))
	for i, n := range s.Nodes {
		if err := m.validateCheckpointState(n, i); err != nil {
			return err
		}
		key, err := m.stateKey(n.State)
		if err != nil {
			return err
		}
		if keys[key] || !s.Visited[key] {
			return fmt.Errorf("node %d does not match visited set", i)
		}
		keys[key] = true
	}
	if len(keys) != len(s.Visited) {
		return fmt.Errorf("visited set does not match nodes")
	}
	return nil
}

func validateSearchCheckpointStructure(s checkpointSnapshot) error {
	if !s.InitialComplete {
		return fmt.Errorf("checkpoint initial generation is incomplete")
	}
	if err := validateCheckpoint(s); err != nil {
		return err
	}
	counts := make([]int, s.Head)
	for _, node := range s.Nodes {
		if node.Parent >= 0 {
			counts[node.Parent]++
		}
	}
	var degrees [32]int
	for _, count := range counts {
		degrees[min(count, len(degrees)-1)]++
	}
	if degrees != s.Counters.OutDegrees {
		return fmt.Errorf("checkpoint outdegrees do not match nodes")
	}
	return nil
}

func (m *model) validateCheckpointState(n checkpointNode, i int) error {
	if len(n.State) != len(m.names) {
		return fmt.Errorf("node %d has wrong variables", i)
	}
	for _, name := range m.names {
		if _, ok := n.State[name]; !ok {
			return fmt.Errorf("node %d missing variable %q", i, name)
		}
	}
	_, err := checkpointEncodeState(n.State)
	return err
}

func (m *model) restoreCheckpoint(snap checkpointSnapshot, identity checkpointIdentity) (*search, error) {
	if err := m.checkpointSupported(); err != nil {
		return nil, err
	}
	if m.tlcRuntime.started || m.tlcRuntime.search != nil {
		return nil, fmt.Errorf("checkpoint requires a newly prepared model")
	}
	if snap.Identity != identity {
		return nil, fmt.Errorf("checkpoint identity mismatch")
	}
	if err := m.validateSearchCheckpoint(snap); err != nil {
		return nil, err
	}
	s := newSearch(m)
	s.initialComplete = true
	s.head, s.visited, s.outDegrees = snap.Head, maps.Clone(snap.Visited), snap.Counters.OutDegrees
	s.result.GeneratedStates, s.result.DistinctStates, s.result.QueuedStates, s.result.Depth = snap.Counters.Generated, snap.Counters.Distinct, snap.Counters.Queued, snap.Counters.Depth
	for _, n := range snap.Nodes {
		var source *parser.Expression
		if n.ActionRange != (parser.Range{}) {
			source = &parser.Expression{Location: parser.Location{Range: n.ActionRange}}
		}
		s.nodes = append(s.nodes, stateNode{state: maps.Clone(n.State), parent: n.Parent, action: n.Action, source: source, depth: n.Depth})
	}
	m.tlcRuntime.started, m.tlcRuntime.initial, m.tlcRuntime.search = true, snap.Counters.Initial, &s
	m.tlcRuntime.traceCurrent, m.tlcRuntime.traceCandidate, m.tlcRuntime.traceIndices = -1, nil, nil
	return &s, nil
}
