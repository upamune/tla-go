package checker

import (
	"fmt"
	"strconv"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// TLC evaluates the view before constraints, including on duplicate candidates.
func (s *search) candidateKeyAndConstraints(state evaluator.State, parent int) (string, bool, error) {
	if s.model.hasReduction() {
		key, err := s.model.stateKey(state)
		if err != nil {
			return "", false, err
		}
		inModel, err := s.satisfiesConstraints(state, parent)
		return key, inModel, err
	}
	inModel, err := s.satisfiesConstraints(state, parent)
	if err != nil {
		return "", false, err
	}
	key, err := s.model.stateKey(state)
	return key, inModel, err
}

func (m *model) viewKey(state evaluator.State) (string, error) {
	expr, err := m.definition(m.cfg.View)
	if err != nil {
		return "", err
	}
	v, err := m.eval(expr, state, nil)
	if err != nil {
		return "", err
	}
	if !m.tlcFingerprint {
		return v.Key()
	}
	fingerprint, err := value.TLCFingerprint([]value.Value{v})
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(fingerprint, 16), nil
}

func (s *search) edgeNext(edge graphEdge) evaluator.State {
	if edge.next != nil {
		return edge.next
	}
	return s.nodes[edge.to].state
}

func (m *model) recoverViewResult(result *Result) error {
	if result.Violation == nil || len(result.Violation.Trace) == 0 {
		return nil
	}
	v := result.Violation
	oldLevel := m.setViewRecoveryLevel(result.Status, v.LoopStart)
	defer func() { m.tlcRuntime.level = oldLevel }()
	pathLength := viewRecoveryPathLength(result.Status, v)
	if pathLength < 2 && v.LoopStart == nil {
		return nil
	}
	trace, err := m.recoverViewTrace(v.Trace, pathLength)
	if err != nil {
		return err
	}
	if err := m.checkViewLoop(trace, v.Trace, v.LoopStart); err != nil {
		return err
	}
	v.Trace = trace
	return nil
}

func viewRecoveryPathLength(status Status, violation *Violation) int {
	n := len(violation.Trace)
	if violation.LoopStart == nil && status != Deadlock {
		return n - 1
	}
	return n
}

func (m *model) setViewRecoveryLevel(status Status, loop *int) int {
	old := m.tlcRuntime.level
	if status == Deadlock || loop != nil {
		m.tlcRuntime.level = 0
	} else {
		m.tlcRuntime.level++
	}
	return old
}

func (m *model) checkViewLoop(trace, original []TraceStep, loop *int) error {
	if loop == nil || *loop == len(trace)-1 {
		return nil
	}
	if *loop < 0 || *loop >= len(trace) {
		return fmt.Errorf("invalid counterexample loop index")
	}
	target, err := m.storedViewKey(original[*loop].State)
	if err != nil {
		return err
	}
	_, found, err := m.recoverViewSuccessor(trace[len(trace)-1].State, target)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("Failed to recover the next state from its fingerprint.")
	}
	return nil
}

func (m *model) recoverViewTrace(trace []TraceStep, pathLength int) ([]TraceStep, error) {
	recovered := append([]TraceStep(nil), trace...)
	target, err := m.storedViewKey(trace[0].State)
	if err != nil {
		return nil, err
	}
	initial, found, err := m.recoverViewInitial(target)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("Failed to recover the initial state from its fingerprint.")
	}
	recovered[0].State = initial
	for i := 1; i < pathLength; i++ {
		if err := m.recoverViewStep(recovered, trace, i); err != nil {
			return nil, err
		}
	}
	return recovered, nil
}

func (m *model) recoverViewStep(recovered, trace []TraceStep, i int) error {
	if trace[i].Action == "Stuttering" {
		recovered[i].State = recovered[i-1].State
		return nil
	}
	target, err := m.storedViewKey(trace[i].State)
	if err != nil {
		return err
	}
	step, found, err := m.recoverViewSuccessor(recovered[i-1].State, target)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("Failed to recover the next state from its fingerprint.")
	}
	recovered[i] = step
	return nil
}

func (m *model) storedViewKey(state evaluator.State) (string, error) {
	r := m.tlcRuntime
	if r.search != nil {
		r.initTraceIndices()
		if index, ok := r.traceIndices[traceStateID(state)]; ok {
			return r.viewKeys[index], nil
		}
	}
	return m.stateKey(state)
}

func (m *model) recoverViewInitial(target string) (evaluator.State, bool, error) {
	init, err := m.definition(m.cfg.Init)
	if err != nil {
		return nil, false, err
	}
	var result evaluator.State
	found := false
	gen := generator{model: m, initial: true, emit: func(state evaluator.State, _ string, _ *parser.Expression) (bool, error) {
		key, err := m.stateKey(state)
		if err != nil {
			return false, err
		}
		if key == target {
			result, found = state, true
			return false, nil
		}
		return true, nil
	}}
	_, err = gen.solve([]work{{expr: init}}, evaluator.State{}, "")
	return result, found, err
}

func (m *model) recoverViewSuccessor(current evaluator.State, target string) (TraceStep, bool, error) {
	next, err := m.definition(m.cfg.Next)
	if err != nil {
		return TraceStep{}, false, err
	}
	var result TraceStep
	found := false
	gen := generator{model: m, current: current, emit: func(state evaluator.State, action string, source *parser.Expression) (bool, error) {
		key, err := m.stateKey(state)
		if err != nil {
			return false, err
		}
		if key == target {
			result = TraceStep{State: state, Action: action, ActionRange: actionSourceRange(source)}
			found = true
			return false, nil
		}
		return true, nil
	}}
	_, err = gen.solve([]work{{expr: next, label: true}}, evaluator.State{}, m.cfg.Next)
	return result, found, err
}

// TLC inserts concrete successors into its live graph before the implicit
// stuttering successor, which matters when a VIEW merges both to one node.
func (s *search) moveViewStutterLast(index int) {
	if len(s.edges) <= index || len(s.edges[index]) == 0 {
		return
	}
	edges := s.edges[index]
	if edges[0].action != "Stuttering" || edges[0].next != nil {
		return
	}
	first := edges[0]
	copy(edges, edges[1:])
	edges[len(edges)-1] = first
}

// A tableau node retains the first successor for each reduced target, as TLC
// does when several concrete successors map to the same tableau node.
func (s *search) viewTableauGraph() *search {
	graph := *s
	graph.edges = make([][]graphEdge, len(s.edges))
	for i, edges := range s.edges {
		seen := make(map[int]bool, len(edges))
		for _, edge := range edges {
			if !seen[edge.to] {
				seen[edge.to] = true
				graph.edges[i] = append(graph.edges[i], edge)
			}
		}
	}
	return &graph
}

// TLC's liveness graph also fingerprints each stored successor once, in the
// order in which successors enter the graph.
func (s *search) fingerprintViewSuccessors(index int) error {
	r := s.model.tlcRuntime
	previous, previousLevel := r.traceCurrent, r.level
	r.traceCurrent = index
	defer func() { r.traceCurrent, r.level = previous, previousLevel }()
	for _, edge := range s.edges[index] {
		if r.viewLivenessSeen[edge.to] {
			continue
		}
		r.viewLivenessSeen[edge.to] = true
		r.level = s.viewSuccessorLevel(index, edge.to)
		if _, err := s.model.stateKey(s.nodes[edge.to].state); err != nil {
			return err
		}
	}
	return nil
}

func (s *search) viewSuccessorLevel(source, target int) int {
	r := s.model.tlcRuntime
	if traceStateID(s.nodes[target].state) == traceStateID(r.viewLastCandidate) {
		return r.viewCandidateLevel
	}
	if source == target {
		return s.nodes[source].depth
	}
	return 0
}

func (m *model) hasReduction() bool { return m.cfg.View != "" || m.cfg.Symmetry != "" }
