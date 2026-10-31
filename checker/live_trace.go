package checker

import (
	"slices"

	"github.com/upamune/tla-go/evaluator"
)

// liveTraceNode is a BEGraphNode or BTGraphNode of LiveCheck1. A node holds
// the truth values of its solution's state checks and, per outgoing edge,
// those of its action checks.
type liveTraceNode struct {
	state   int
	tableau int
	checks  []bool
	next    []int
	actions [][]bool
	number  int64
}

type liveTraceKey struct {
	state   string
	tableau int
}

// liveTrace is the sampled trace that LiveCheck1 checks. Like TLC
// fingerprints, its keys identify states up to VIEW and SYMMETRY. TLC
// evaluates each check on every node and edge of each solution's graph;
// tla-go evaluates a leaf once per state, or per step for an action, which
// only skips repeated side effects.
type liveTrace struct {
	states []evaluator.State
	keys   []string
	// leaves holds, by leaf tag and slot, 1 for a true and -1 for a false leaf.
	leaves [][]int8
}

func newLiveTrace(m *model, states []evaluator.State) (*liveTrace, error) {
	t := &liveTrace{states: states, keys: make([]string, len(states))}
	for i, state := range states {
		key, err := m.stateKey(state)
		if err != nil {
			return nil, err
		}
		t.keys[i] = key
	}
	return t, nil
}

func (t *liveTrace) leaf(n *liveNode, current, next int) (bool, error) {
	slot, cached := liveLeafSlot(n, current, next)
	values := t.leafValues(n.leaf.tag)
	if cached && values[slot] != 0 {
		return values[slot] > 0, nil
	}
	var nextState evaluator.State
	if next >= 0 {
		nextState = t.states[next]
	}
	holds, err := n.leaf.eval(t.states[current], nextState)
	if err == nil && cached {
		values[slot] = -1
		if holds {
			values[slot] = 1
		}
	}
	return holds, err
}

// liveLeafSlot caches a state leaf per state and an action leaf per
// stuttering or forward step, the edges of LiveCheck1's graphs.
func liveLeafSlot(n *liveNode, current, next int) (int, bool) {
	switch {
	case n.kind == liveState || next == current:
		return 2 * current, true
	case next == current+1:
		return 2*current + 1, true
	}
	return 0, false
}

func (t *liveTrace) leafValues(tag int) []int8 {
	for len(t.leaves) <= tag {
		t.leaves = append(t.leaves, nil)
	}
	if t.leaves[tag] == nil {
		t.leaves[tag] = make([]int8, 2*len(t.states))
	}
	return t.leaves[tag]
}

// liveTraceGraph is the behavior graph that LiveCheck1.constructBEGraph
// builds from one simulated trace and one solution.
type liveTraceGraph struct {
	solution *liveSolution
	*liveTrace
	nodes   []liveTraceNode
	initial []int
	index   map[liveTraceKey]int
}

func newLiveTraceGraph(trace *liveTrace, solution *liveSolution) (*liveTraceGraph, error) {
	g := &liveTraceGraph{solution: solution, liveTrace: trace, index: map[liveTraceKey]int{}}
	if solution.tableau == nil {
		return g, g.buildPlain()
	}
	return g, g.buildTableau()
}

func (g *liveTraceGraph) checkState(state int) ([]bool, error) {
	return g.checks(g.solution.checkState, state, -1)
}

func (g *liveTraceGraph) checkAction(from, to int) ([]bool, error) {
	return g.checks(g.solution.checkAction, from, to)
}

func (g *liveTraceGraph) checks(checks []*liveNode, current, next int) ([]bool, error) {
	result := make([]bool, len(checks))
	for i, check := range checks {
		holds, err := check.eval(g.liveTrace, current, next)
		if err != nil {
			return nil, err
		}
		result[i] = holds
	}
	return result, nil
}

func (g *liveTraceGraph) addNode(state, tableau int, checks []bool) int {
	index := len(g.nodes)
	g.nodes = append(g.nodes, liveTraceNode{state: state, tableau: tableau, checks: checks})
	g.index[liveTraceKey{state: g.keys[state], tableau: tableau}] = index
	return index
}

func (g *liveTraceGraph) link(from, to int, actions []bool) {
	node := &g.nodes[from]
	node.next = append(node.next, to)
	node.actions = append(node.actions, actions)
}

func (g *liveTraceGraph) hasEdge(from, to int) bool {
	return slices.Contains(g.nodes[from].next, to)
}

func (g *liveTraceGraph) tableauNode(node int) *liveTableauNode {
	return g.solution.tableau.nodes[g.nodes[node].tableau]
}

// Without a tableau TLC merges equal states and gives each new state the
// state checks of its predecessor in the trace.
func (g *liveTraceGraph) buildPlain() error {
	checks, err := g.checkState(0)
	if err != nil {
		return err
	}
	stutter, err := g.checkAction(0, 0)
	if err != nil {
		return err
	}
	source := g.addNode(0, -1, checks)
	g.initial = []int{source}
	g.link(source, source, stutter)
	for i := 1; i < len(g.states); i++ {
		if source, err = g.addPlainState(source, i); err != nil {
			return err
		}
	}
	return nil
}

func (g *liveTraceGraph) addPlainState(source, state int) (int, error) {
	target, ok := g.index[liveTraceKey{state: g.keys[state], tableau: -1}]
	if ok && g.hasEdge(source, target) {
		return target, nil
	}
	if !ok {
		var err error
		if target, err = g.addPlainNode(state); err != nil {
			return 0, err
		}
	}
	step, err := g.checkAction(state-1, state)
	if err != nil {
		return 0, err
	}
	g.link(source, target, step)
	return target, nil
}

func (g *liveTraceGraph) addPlainNode(state int) (int, error) {
	checks, err := g.checkState(state - 1)
	if err != nil {
		return 0, err
	}
	stutter, err := g.checkAction(state, state)
	if err != nil {
		return 0, err
	}
	target := g.addNode(state, -1, checks)
	g.link(target, target, stutter)
	return target, nil
}

// With a tableau only the product nodes created for the previous trace state
// are expanded, so a revisited state ends the construction.
func (g *liveTraceGraph) buildTableau() error {
	frontier, err := g.initialTableauNodes()
	if err != nil {
		return err
	}
	for i := 1; i < len(g.states); i++ {
		if frontier, err = g.advanceTableau(frontier, i); err != nil {
			return err
		}
	}
	return nil
}

func (g *liveTraceGraph) initialTableauNodes() ([]int, error) {
	checks, err := g.checkState(0)
	if err != nil {
		return nil, err
	}
	stutter, err := g.checkAction(0, 0)
	if err != nil {
		return nil, err
	}
	for _, node := range g.solution.tableau.nodes[:g.solution.tableau.initial] {
		consistent, err := node.consistent(g.liveTrace, 0)
		if err != nil {
			return nil, err
		}
		if consistent {
			g.initial = append(g.initial, g.addNode(0, node.index, checks))
		}
	}
	g.linkInitialStuttering(stutter)
	return g.initial, nil
}

// TLC links the initial nodes among themselves but does not create nodes for
// the other tableau successors of the initial state.
func (g *liveTraceGraph) linkInitialStuttering(stutter []bool) {
	for _, source := range g.initial {
		for _, next := range g.tableauNode(source).next {
			if target, ok := g.index[liveTraceKey{state: g.keys[0], tableau: next.index}]; ok {
				g.link(source, target, stutter)
			}
		}
	}
}

func (g *liveTraceGraph) advanceTableau(frontier []int, state int) ([]int, error) {
	checks, err := g.checkState(state)
	if err != nil {
		return nil, err
	}
	step, err := g.checkAction(state-1, state)
	if err != nil {
		return nil, err
	}
	var created []int
	for _, source := range frontier {
		if err := g.expandTableau(source, state, checks, step, &created); err != nil {
			return nil, err
		}
	}
	stutter, err := g.checkAction(state, state)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(created); i++ {
		if err := g.expandTableau(created[i], state, checks, stutter, &created); err != nil {
			return nil, err
		}
	}
	return created, nil
}

func (g *liveTraceGraph) expandTableau(source, state int, checks, actions []bool, created *[]int) error {
	for _, next := range g.tableauNode(source).next {
		target, ok := g.index[liveTraceKey{state: g.keys[state], tableau: next.index}]
		if ok {
			if !g.hasEdge(source, target) {
				g.link(source, target, actions)
			}
			continue
		}
		consistent, err := next.consistent(g.liveTrace, state)
		if err != nil {
			return err
		}
		if consistent {
			target = g.addNode(state, next.index, checks)
			g.link(source, target, actions)
			*created = append(*created, target)
		}
	}
	return nil
}
