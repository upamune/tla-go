package checker

func (f *fairSearch) checkRecurrence(index int, fair fairnessCondition, component []int, members []bool) (bool, []bool, error) {
	witness, found, err := f.recurrenceWitness(fair, component, members)
	if err == nil && found {
		f.cacheRecurrenceWitness(index, witness)
	}
	return found, nil, err
}

func (f *fairSearch) cacheRecurrenceWitness(index int, witness cycleWitness) {
	if f.recurrenceKnown == nil {
		f.recurrenceWitnesses = make([]cycleWitness, len(f.search.model.fairness))
		f.recurrenceKnown = make([]bool, len(f.search.model.fairness))
	}
	f.recurrenceWitnesses[index] = witness
	f.recurrenceKnown[index] = true
}

// A recurrence assumption holds on a finite-state lasso exactly when its
// state predicate or action occurs somewhere on the cycle.
func (f *fairSearch) recurrenceWitness(fair fairnessCondition, component []int, members []bool) (cycleWitness, bool, error) {
	if fair.recurrenceAction {
		return f.recurrenceActionWitness(fair, component, members)
	}
	for _, node := range component {
		ok, err := f.search.model.holdsBaseProperty(fair.action, f.search.nodes[node].state, fair.locals)
		if err != nil || ok {
			return cycleWitness{node: node}, ok, err
		}
	}
	return cycleWitness{}, false, nil
}

func (f *fairSearch) recurrenceActionWitness(fair fairnessCondition, component []int, members []bool) (cycleWitness, bool, error) {
	property := temporalProperty{predicate: fair.action, locals: fair.locals}
	for _, node := range component {
		for _, edge := range f.search.edges[node] {
			if !members[edge.to] {
				continue
			}
			ok, err := f.search.actionPropertyForEdge(property, node, edge)
			if err != nil || ok {
				return cycleWitness{node: node, edge: &edge}, ok, err
			}
		}
	}
	return cycleWitness{}, false, nil
}
