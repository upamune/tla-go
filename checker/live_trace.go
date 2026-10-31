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

// liveTraceGraph is the behavior graph that LiveCheck1.constructBEGraph
// builds from one simulated trace and one solution. Like TLC fingerprints,
// its keys identify states up to VIEW and SYMMETRY.
type liveTraceGraph struct {
	solution *liveSolution
	states   []evaluator.State
	keys     []string
	nodes    []liveTraceNode
	initial  []int
	index    map[liveTraceKey]int
}

func newLiveTraceGraph(m *model, solution *liveSolution, states []evaluator.State) (*liveTraceGraph, error) {
	g := &liveTraceGraph{solution: solution, states: states, keys: make([]string, len(states)), index: map[liveTraceKey]int{}}
	for i, state := range states {
		key, err := m.stateKey(state)
		if err != nil {
			return nil, err
		}
		g.keys[i] = key
	}
	if solution.tableau == nil {
		return g, g.buildPlain()
	}
	return g, g.buildTableau()
}

func (g *liveTraceGraph) checkState(state int) ([]bool, error) {
	return liveChecks(g.solution.checkState, g.states[state], nil)
}

func (g *liveTraceGraph) checkAction(from, to int) ([]bool, error) {
	return liveChecks(g.solution.checkAction, g.states[from], g.states[to])
}

func liveChecks(checks []*liveNode, current, next evaluator.State) ([]bool, error) {
	result := make([]bool, len(checks))
	for i, check := range checks {
		holds, err := check.eval(current, next)
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
		consistent, err := node.consistent(g.states[0])
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
		consistent, err := next.consistent(g.states[state])
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
