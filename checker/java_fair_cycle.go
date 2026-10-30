package checker

import "fmt"

type javaFairRequirement struct {
	index int
	done  bool
}

type javaFairCycle struct {
	f           *fairSearch
	product     *search
	order       [][]int
	members     []bool
	seen        []bool
	requirement []javaFairRequirement
	count       int
	current     int
	nodes       []int
	path        []graphEdge
	witnesses   []cycleWitness
}

func (f *fairSearch) recordJavaFairCycle(product *search, order [][]int, root int, component, base []int, count int) error {
	c := javaFairCycle{f: f, product: product, order: order, members: make([]bool, len(product.nodes)), seen: make([]bool, len(product.nodes)), count: count, current: root, nodes: []int{root}}
	for _, node := range component {
		c.members[node] = true
	}
	for index, fair := range f.search.model.fairness {
		_, err := f.enabledStates(index, fair, base)
		if err != nil {
			return err
		}
		c.requirement = append(c.requirement, javaFairRequirement{index: index})
	}
	c.seen[root] = true
	if err := c.search(); err != nil {
		return err
	}
	f.debugCycle(root-count, c.witnesses)
	return c.record(root)
}

func (c *javaFairCycle) search() error {
	for !c.complete() {
		if err := c.step(); err != nil {
			return err
		}
	}
	return nil
}

func (c *javaFairCycle) step() error {
	if err := c.product.model.ctx.Err(); err != nil {
		return err
	}
	edge, progressed, found, err := c.chooseEdge()
	if err != nil {
		return err
	}
	if found {
		c.take(edge, progressed)
		return nil
	}
	if !c.backtrack() {
		return fmt.Errorf("no fairness witness in TLC component")
	}
	return nil
}

func (c *javaFairCycle) complete() bool {
	for _, requirement := range c.requirement {
		if !requirement.done {
			return false
		}
	}
	return true
}

func (c *javaFairCycle) chooseEdge() (graphEdge, bool, bool, error) {
	var unseen graphEdge
	hasUnseen := false
	for _, choice := range c.order[c.current] {
		edge := c.product.edges[c.current][choice]
		if !c.members[edge.to] {
			continue
		}
		progress, err := c.markActions(edge)
		if err != nil || progress {
			return edge, true, true, err
		}
		if !c.seen[edge.to] {
			unseen, hasUnseen = edge, true
		}
	}
	return unseen, false, hasUnseen, nil
}

func (c *javaFairCycle) markActions(edge graphEdge) (bool, error) {
	changed := false
	for i := range c.requirement {
		r := &c.requirement[i]
		if r.done {
			continue
		}
		ok, err := c.satisfiesWeakFairness(r.index, edge)
		if err != nil {
			return false, err
		}
		if ok {
			r.done, changed = true, true
			if c.f.search.model.debug != nil {
				baseEdge := edge
				baseEdge.to -= c.count
				c.witnesses = append(c.witnesses, cycleWitness{node: c.current - c.count, edge: &baseEdge})
			}
		}
	}
	return changed, nil
}

func (c *javaFairCycle) satisfiesWeakFairness(index int, edge graphEdge) (bool, error) {
	if c.f.enabled[index][c.current-c.count] == -1 {
		return true, nil
	}
	fair := c.f.search.model.fairness[index]
	return c.f.search.model.holdsFair(fair, c.product.nodes[c.current].state, c.product.nodes[edge.to].state)
}

func (c *javaFairCycle) take(edge graphEdge, progress bool) {
	if progress {
		clear(c.seen)
	} else {
		c.seen[edge.to] = true
	}
	c.path = append(c.path, edge)
	c.current = edge.to
	c.nodes = append(c.nodes, c.current)
}

func (c *javaFairCycle) backtrack() bool {
	for len(c.path) > 0 {
		c.path = c.path[:len(c.path)-1]
		c.nodes = c.nodes[:len(c.nodes)-1]
		c.current = c.nodes[len(c.nodes)-1]
		for _, choice := range c.order[c.current] {
			edge := c.product.edges[c.current][choice]
			if c.members[edge.to] && !c.seen[edge.to] {
				c.take(edge, false)
				return true
			}
		}
	}
	return false
}

func (c *javaFairCycle) record(root int) error {
	trace, err := c.product.javaLeadsToPath(c.order, root, c.count)
	if err != nil {
		return err
	}
	loop := len(trace) - 1
	previous := root
	for _, edge := range c.path {
		if edge.to != previous {
			trace = append(trace, TraceStep{Action: edge.action, State: c.product.nodes[edge.to].state, ActionRange: actionSourceRange(edge.source)})
		}
		previous = edge.to
	}
	if c.current != root {
		back, err := c.product.javaLivePath([]int{c.current}, root, c.members, c.order)
		if err != nil {
			return err
		}
		trace = append(trace, back[1:]...)
	}
	closing := "Stuttering"
	if len(trace) > loop+1 {
		last := trace[len(trace)-1]
		trace = trace[:len(trace)-1]
		closing = last.Action
		c.f.search.result.Violation = &Violation{Name: c.f.property.name, Trace: trace, LoopStart: &loop, LoopAction: closing, LoopActionRange: last.ActionRange}
	} else {
		c.f.search.result.Violation = &Violation{Name: c.f.property.name, Trace: trace, LoopStart: &loop, LoopAction: closing}
	}
	c.f.search.result.Status = LivenessViolation
	return nil
}
