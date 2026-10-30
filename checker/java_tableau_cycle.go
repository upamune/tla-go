package checker

// TLC's counterexample walk satisfies tableau promises and fairness checks
// together, so one state or transition can discharge several requirements.
func (f *fairSearch) recordJavaTableauCycle(root int, component []int, order [][]int) error {
	c := javaFairCycle{f: f, product: f.search, order: order, members: make([]bool, len(f.search.nodes)), seen: make([]bool, len(f.search.nodes)), current: root, nodes: []int{root}, tableau: true, promises: make([]bool, len(f.acceptance))}
	for _, node := range component {
		c.members[node] = true
	}
	for index, fair := range f.search.model.fairness {
		enabled, err := f.enabledStates(index, fair, component)
		if err != nil {
			return err
		}
		if !fair.strong || enabled > 0 {
			c.requirement = append(c.requirement, javaFairRequirement{index: index})
		}
	}
	c.seen[root] = true
	if err := c.search(); err != nil {
		return err
	}
	f.debugCycle(root, c.witnesses)
	return c.recordTableau(root)
}

func (c *javaFairCycle) markTableauPromises() bool {
	changed := false
	for i := range c.promises {
		if !c.promises[i] && c.f.acceptance[i][c.current] {
			c.promises[i], changed = true, true
		}
	}
	return changed
}

func (c *javaFairCycle) recordTableau(root int) error {
	trace := c.f.prefix(root)
	loop := len(trace) - 1
	for _, edge := range c.path {
		trace = append(trace, TraceStep{Action: edge.action, State: c.product.nodes[edge.to].state, ActionRange: actionSourceRange(edge.source)})
	}
	if c.current != root {
		if err := c.product.appendPath(&trace, c.current, root, c.members); err != nil {
			return err
		}
	}
	if len(trace) == loop+1 {
		trace = append(trace, TraceStep{Action: "Stuttering", State: c.product.nodes[root].state})
	}
	last := trace[len(trace)-1]
	c.f.search.result.Status = LivenessViolation
	c.f.search.result.Violation = &Violation{Name: c.f.property.name, Trace: trace[:len(trace)-1], LoopStart: &loop, LoopAction: last.Action, LoopActionRange: last.ActionRange}
	return nil
}

func (f *fairSearch) supportsJavaTableauCycle() bool {
	for _, fair := range f.search.model.fairness {
		if fair.recurrence {
			return false
		}
	}
	return true
}
