package checker

import (
	"errors"
	"fmt"
	"slices"
)

var errLiveCycle = errors.New("LiveCheck failed to find a cycle in the bad component")

// liveCycle follows the first half of LiveCheck1.printErrorTrace. Starting at
// a node of the bad subcomponent, it walks until every requirement is met and
// then closes the walk into a cycle. The stack is the printed cycle.
type liveCycle struct {
	scc     *liveSCC
	low     int64
	found   liveRequirements
	count   int
	stack   []int
	current int
}

func (c *liveSCC) cycle(node int) ([]int, error) {
	y := &liveCycle{scc: c, low: c.third - 1, found: c.newRequirements(), current: node}
	y.count = len(y.found.aeState) + len(y.found.aeAction) + len(y.found.promises)
	for y.count > 0 {
		c.setNumber(y.current, c.third)
		if err := y.advance(); err != nil {
			return nil, err
		}
	}
	if err := y.close(node); err != nil {
		return nil, err
	}
	return y.stack, nil
}

func (y *liveCycle) advance() error {
	before := y.count
	for {
		y.markNode()
		if y.count <= 0 {
			return nil
		}
		first, last, moved := y.scanEdges()
		if moved {
			return nil
		}
		if y.count < before {
			return y.moveTo(first)
		}
		if err := y.backtrack(last); err != nil {
			return err
		}
	}
}

func (y *liveCycle) markNode() {
	c := y.scc
	for i, check := range c.pem.aeState {
		if !y.found.aeState[i] && c.graph.nodes[y.current].checks[check] {
			y.found.aeState[i] = true
			y.count--
		}
	}
	for i, promise := range c.solution.promises {
		if !y.found.promises[i] && c.graph.tableauNode(y.current).par.fulfills(promise) {
			y.found.promises[i] = true
			y.count--
		}
	}
}

// scanEdges moves along the first edge that meets a []<>A check. Otherwise it
// returns the last unvisited and the last visited successor in the component.
func (y *liveCycle) scanEdges() (int, int, bool) {
	c := y.scc
	first, last, before := -1, -1, y.count
	for edge, next := range c.graph.nodes[y.current].next {
		number := c.number(next)
		if y.low <= number && number <= c.third {
			first = next
			y.markActions(edge)
		}
		if y.count < before {
			y.stack = append(y.stack, y.current)
			y.current = next
			c.third++
			return first, last, true
		}
		if y.low <= number && number < c.third {
			last = next
		}
	}
	return first, last, false
}

func (y *liveCycle) markActions(edge int) {
	actions := y.scc.graph.nodes[y.current].actions[edge]
	for i, check := range y.scc.pem.aeAction {
		if !y.found.aeAction[i] && actions[check] {
			y.found.aeAction[i] = true
			y.count--
		}
	}
}

func (y *liveCycle) moveTo(next int) error {
	if next < 0 {
		return errLiveCycle
	}
	y.stack = append(y.stack, y.current)
	y.current = next
	y.scc.third++
	return nil
}

// backtrack pops the walk until a node has a visited successor. TLC pushes
// such a node twice, which only repeats a state in the cycle.
func (y *liveCycle) backtrack(next int) error {
	for next < 0 {
		if len(y.stack) == 0 {
			return errLiveCycle
		}
		y.current = y.stack[len(y.stack)-1]
		y.stack = y.stack[:len(y.stack)-1]
		if next = y.visitedSuccessor(y.current); next >= 0 {
			y.stack = append(y.stack, y.current)
		}
	}
	y.stack = append(y.stack, y.current)
	y.current = next
	y.scc.setNumber(next, y.scc.third)
	return nil
}

func (y *liveCycle) visitedSuccessor(node int) int {
	for _, next := range y.scc.graph.nodes[node].next {
		if number := y.scc.number(next); y.low <= number && number < y.scc.third {
			return next
		}
	}
	return -1
}

func (y *liveCycle) close(node int) error {
	c := y.scc
	c.third++
	c.setNumber(y.current, c.third)
	for y.current != node {
		next := y.visitedSuccessor(y.current)
		if next < 0 {
			if len(y.stack) == 0 {
				return errLiveCycle
			}
			y.current = y.stack[len(y.stack)-1]
			y.stack = y.stack[:len(y.stack)-1]
			continue
		}
		y.stack = append(y.stack, y.current)
		if next == node {
			break
		}
		y.current = next
		c.setNumber(next, c.third)
	}
	if len(y.stack) == 0 {
		y.stack = append(y.stack, y.current)
	}
	return nil
}

// path is BEGraph.getPath: a breadth-first path from start to end.
func (g *liveTraceGraph) path(start, end int) ([]int, error) {
	if start == end {
		return []int{start}, nil
	}
	parent := map[int]int{start: -1}
	for queue := []int{start}; len(queue) > 0; queue = queue[1:] {
		if g.discover(queue[0], end, parent, &queue) {
			return livePath(parent, end), nil
		}
	}
	return nil, fmt.Errorf("LiveCheck failed to construct a path in the behavior graph")
}

func (g *liveTraceGraph) discover(node, end int, parent map[int]int, queue *[]int) bool {
	for _, next := range g.nodes[node].next {
		if _, seen := parent[next]; seen {
			continue
		}
		parent[next] = node
		if next == end {
			return true
		}
		*queue = append(*queue, next)
	}
	return false
}

func livePath(parent map[int]int, end int) []int {
	var path []int
	for node := end; node >= 0; node = parent[node] {
		path = append([]int{node}, path...)
	}
	return path
}

// liveTraceBuilder prints the prefix and the cycle like printErrorTrace. TLC
// recovers each printed state from its fingerprint and the last printed
// state, and skips a node whose state equals that of the previous node.
type liveTraceBuilder struct {
	graph             *liveTraceGraph
	recover           liveRecovery
	trace             []TraceStep
	previous, printed string
	shown             int
}

// liveRecoveryError is TLC's EvalException for a state that cannot be
// recovered from its fingerprint. TLC has then printed shown states.
type liveRecoveryError struct {
	code    int
	message string
	shown   []TraceStep
}

func (e *liveRecoveryError) Error() string { return e.message }

// fail records the states that TLC printed before the failure: the prefix is
// printed only once all of its states are recovered.
func (b *liveTraceBuilder) fail(err error) error {
	if recovery, ok := errors.AsType[*liveRecoveryError](err); ok {
		recovery.shown = slices.Clone(b.trace[:b.shown])
	}
	return err
}

// liveRecovery returns the state that follows the printed steps: the first
// initial state with the given key for an empty trace, or else the first
// successor of the last printed state with the action that generates it.
type liveRecovery func(printed []TraceStep, key string) (TraceStep, error)

func (g *liveTraceGraph) nodeKey(node int) string { return g.keys[g.nodes[node].state] }

func (b *liveTraceBuilder) appendNode(node int) error {
	key := b.graph.nodeKey(node)
	repeated := len(b.trace) > 0 && key == b.previous
	b.previous = key
	if repeated {
		return nil
	}
	b.printed = key
	step, err := b.recover(b.trace, key)
	if err != nil {
		return b.fail(err)
	}
	b.trace = append(b.trace, step)
	return nil
}

func (c *liveSCC) violation(recover liveRecovery) (*Violation, error) {
	node := c.bad
	stack, err := c.cycle(node)
	if err != nil {
		return nil, err
	}
	prefix, err := c.graph.path(c.root, node)
	if err != nil {
		return nil, err
	}
	b := liveTraceBuilder{graph: c.graph, recover: recover}
	for _, step := range prefix {
		if err := b.appendNode(step); err != nil {
			return nil, err
		}
	}
	loop := len(b.trace) - 1
	b.shown = len(b.trace)
	cycleKey := b.printed
	b.previous = c.graph.nodeKey(stack[0])
	for _, step := range stack[1:] {
		if err := b.appendNode(step); err != nil {
			return nil, err
		}
		b.shown = len(b.trace)
	}
	return b.loop(node, loop, cycleKey)
}

// loop reports stuttering when the last printed state is that of the bad
// node, even if the cycle visits other states, and a back edge otherwise.
func (b *liveTraceBuilder) loop(node, loop int, cycleKey string) (*Violation, error) {
	last := len(b.trace) - 1
	v := &Violation{Trace: b.trace, LoopStart: &last, LoopAction: "Stuttering"}
	if b.graph.nodeKey(node) == b.printed {
		return v, nil
	}
	step, err := b.recover(b.trace, cycleKey)
	v.LoopStart, v.LoopAction, v.LoopActionRange = &loop, step.Action, step.ActionRange
	return v, b.fail(err)
}
