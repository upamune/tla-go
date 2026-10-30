package checker

// With fairness, a finite safety violation is relevant only if it can be
// extended to a fair infinite behavior. Search fair cycles reachable from any
// state where an armed monitor has observed a violation.
func (s *search) checkFairGuardedProperty(property temporalProperty) (bool, error) {
	monitor := &guardedMonitor{property: property, nodes: make([]guardedNode, len(s.nodes))}
	bad, err := monitor.fairViolationSeeds(s)
	if err != nil {
		return false, err
	}
	f := fairSearch{search: s, property: property, trigger: bad, guarded: monitor, enabled: make([][]int8, len(s.model.fairness))}
	for i := range f.enabled {
		if !s.model.fairness[i].recurrence {
			f.enabled[i] = make([]int8, len(s.nodes))
		}
	}
	return f.findFairCycle(f.reachableStates())
}

func (g *guardedMonitor) fairViolationSeeds(s *search) ([]bool, error) {
	for index := range s.nodes {
		trigger, err := s.model.holdsProperty(g.property.trigger, s.nodes[index].state, g.property.locals)
		if err != nil {
			return nil, err
		}
		if trigger {
			g.nodes[index].armed = true
			g.nodes[index].parent = -1
			g.nodes[index].expanded = true
			g.ready = append(g.ready, index)
		}
	}
	bad := make([]bool, len(s.nodes))
	for head := 0; head < len(g.ready); head++ {
		if err := s.model.ctx.Err(); err != nil {
			return nil, err
		}
		index := g.ready[head]
		valid, err := s.model.holdsProperty(g.property.predicate, s.nodes[index].state, g.property.locals)
		if err != nil {
			return nil, err
		}
		bad[index] = !valid
		g.propagateFairEdges(s, index)
	}
	return bad, nil
}

func (g *guardedMonitor) propagateFairEdges(s *search, index int) {
	for _, edge := range s.edges[index] {
		if g.nodes[edge.to].armed {
			continue
		}
		g.nodes[edge.to].armed = true
		g.nodes[edge.to].parent = index
		g.nodes[edge.to].action = edge.action
		g.nodes[edge.to].source = edge.source
		g.ready = append(g.ready, edge.to)
	}
}
