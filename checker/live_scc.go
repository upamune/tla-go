package checker

import (
	"math"
	"slices"
)

// LiveCheck1 numbers nodes in three ranges: Tarjan's search uses the first,
// components the second and the subcomponents left after pruning <>[]A the
// third.
const (
	liveMaxFirst  int64 = 0x2000000000000000
	liveMaxSecond int64 = 0x5000000000000000
)

type liveSCC struct {
	graph    *liveTraceGraph
	solution *liveSolution
	pem      *livePEM
	stack    []int
	root     int
	bad      int

	first, second, third      int64
	startSecond, startThird   int64
	firstComponent, secondCom int64
}

func newLiveSCC(graph *liveTraceGraph) *liveSCC {
	c := &liveSCC{graph: graph, solution: graph.solution, bad: -1, first: 1, second: liveMaxFirst + 1, third: liveMaxSecond + 1}
	c.startSecond, c.startThird = c.second, c.third
	c.firstComponent, c.secondCom = c.second, c.third
	return c
}

// search runs LiveCheck1.checkSccs from each initial node in order.
func (c *liveSCC) search() bool {
	for _, root := range c.graph.initial {
		if c.number(root) != 0 {
			continue
		}
		c.root = root
		c.visit(root)
		if c.bad >= 0 {
			return true
		}
	}
	return false
}

func (c *liveSCC) number(node int) int64 { return c.graph.nodes[node].number }

func (c *liveSCC) setNumber(node int, number int64) { c.graph.nodes[node].number = number }

func (c *liveSCC) push(node int) { c.stack = append(c.stack, node) }

func (c *liveSCC) pop() int {
	node := c.stack[len(c.stack)-1]
	c.stack = c.stack[:len(c.stack)-1]
	return node
}

func (c *liveSCC) visit(node int) int64 {
	low := c.first
	c.first++
	c.setNumber(node, low)
	c.push(node)
	for _, next := range c.graph.nodes[node].next {
		number := c.number(next)
		if number == 0 {
			number = c.visit(next)
			if c.bad >= 0 {
				return 0
			}
		}
		low = min(low, number)
	}
	if low == c.number(node) {
		c.checkComponent(node)
	}
	return low
}

func (c *liveSCC) checkComponent(node int) {
	nodes := c.extractComponent(node)
	if nodes == nil {
		return
	}
	for i := range c.solution.pems {
		c.pem = &c.solution.pems[i]
		c.startSecond, c.startThird = c.second, c.third
		for _, node := range slices.Backward(nodes) {
			if c.number(node) >= c.startThird {
				continue
			}
			c.visitSubcomponent(node)
			if c.bad >= 0 {
				return
			}
		}
	}
}

// extractComponent returns the nodes of a nontrivial component, last popped
// first, and gives them a common number.
func (c *liveSCC) extractComponent(node int) []int {
	top := c.pop()
	if top == node && !c.graph.hasEdge(node, node) {
		c.setNumber(node, liveMaxFirst)
		return nil
	}
	c.firstComponent = c.second
	c.second++
	c.secondCom = c.third
	nodes := []int{top}
	c.setNumber(top, c.firstComponent)
	for top != node {
		top = c.pop()
		c.setNumber(top, c.firstComponent)
		nodes = append(nodes, top)
	}
	return nodes
}

func (c *liveSCC) edgeHolds(node, edge int, checks []int) bool {
	actions := c.graph.nodes[node].actions[edge]
	for _, check := range checks {
		if !actions[check] {
			return false
		}
	}
	return true
}

// visitSubcomponent is checkSccs1: Tarjan's search restricted to edges that
// satisfy every <>[]A check of the current possible error model.
func (c *liveSCC) visitSubcomponent(node int) int64 {
	low := c.second
	c.second++
	c.setNumber(node, low)
	c.push(node)
	for edge, next := range c.graph.nodes[node].next {
		low = min(low, c.subcomponentEdge(node, edge, next))
		if c.bad >= 0 {
			return 0
		}
	}
	if low == c.number(node) && c.extractSubcomponent(node) {
		c.checkSubcomponent(node)
	}
	return low
}

// subcomponentEdge returns the number that an edge contributes to the low
// link, visiting its target first when it has not been numbered yet.
func (c *liveSCC) subcomponentEdge(node, edge, next int) int64 {
	number := c.number(next)
	if c.firstComponent > number || !c.edgeHolds(node, edge, c.pem.eaAction) {
		return math.MaxInt64
	}
	if number < c.startSecond || c.secondCom <= number && number < c.startThird {
		return c.visitSubcomponent(next)
	}
	return number
}

func (c *liveSCC) extractSubcomponent(node int) bool {
	top := c.pop()
	if top == node && !c.canStutter(node) {
		c.setNumber(node, c.third)
		c.third++
		return false
	}
	c.setNumber(top, c.third)
	for top != node {
		top = c.pop()
		c.setNumber(top, c.third)
	}
	return true
}

func (c *liveSCC) canStutter(node int) bool {
	for edge, next := range c.graph.nodes[node].next {
		if next == node {
			return c.firstComponent <= c.number(node) && c.edgeHolds(node, edge, c.pem.eaAction)
		}
	}
	return false
}

type liveRequirements struct{ aeState, aeAction, promises []bool }

func (c *liveSCC) newRequirements() liveRequirements {
	return liveRequirements{aeState: make([]bool, len(c.pem.aeState)), aeAction: make([]bool, len(c.pem.aeAction)), promises: make([]bool, len(c.solution.promises))}
}

// checkSubcomponent reports a counterexample when the subcomponent satisfies
// every []<> check and fulfils every tableau promise.
func (c *liveSCC) checkSubcomponent(node int) {
	found := c.newRequirements()
	c.setNumber(node, c.number(node)+1)
	pending := []int{node}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		c.markStates(current, found.aeState)
		pending = c.markSubcomponentEdges(current, found.aeAction, pending)
		c.markPromises(current, found.promises)
	}
	c.third += 2
	if allTrue(found.aeState) && allTrue(found.aeAction) && allTrue(found.promises) {
		c.bad = node
	}
}

func (c *liveSCC) markStates(node int, found []bool) {
	for i, check := range c.pem.aeState {
		if !found[i] {
			found[i] = c.graph.nodes[node].checks[check]
		}
	}
}

func (c *liveSCC) markSubcomponentEdges(node int, found []bool, pending []int) []int {
	for edge, next := range c.graph.nodes[node].next {
		number := c.number(next)
		if number >= c.third {
			c.markActions(node, edge, found)
		}
		if number == c.third {
			c.setNumber(next, number+1)
			pending = append(pending, next)
		}
	}
	return pending
}

func (c *liveSCC) markActions(node, edge int, found []bool) {
	actions := c.graph.nodes[node].actions[edge]
	for i, check := range c.pem.aeAction {
		if !found[i] {
			found[i] = actions[check]
		}
	}
}

func (c *liveSCC) markPromises(node int, found []bool) {
	for i, promise := range c.solution.promises {
		if c.graph.tableauNode(node).par.fulfills(promise) {
			found[i] = true
		}
	}
}

func allTrue(values []bool) bool {
	for _, value := range values {
		if !value {
			return false
		}
	}
	return true
}
