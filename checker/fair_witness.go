package checker

import "fmt"

type cycleWitness struct {
	node int
	edge *graphEdge
}

func (f *fairSearch) cycleWitnesses(component []int, members []bool) ([]cycleWitness, error) {
	result, err := f.propertyWitnesses(component, members)
	if err != nil {
		return nil, err
	}
	for index, fair := range f.search.model.fairness {
		witness, needed, err := f.fairWitness(index, fair, component, members)
		if err != nil {
			return nil, err
		}
		if needed {
			result = append(result, witness)
		}
	}
	return result, nil
}

func (f *fairSearch) propertyWitnesses(component []int, members []bool) ([]cycleWitness, error) {
	var result []cycleWitness
	if f.property.mode == eventuallyAlways || f.property.mode == leadsToStaying {
		result = append(result, f.badWitness(component))
	}
	if len(f.required) > 0 {
		result = append(result, f.requiredWitness(component))
	}
	if f.requiredAction != nil {
		witness, ok, err := f.requiredActionWitness(component, members)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("required action witness was not found")
		}
		result = append(result, witness)
	}
	tableau, err := f.tableauWitnesses(component, members)
	if err != nil {
		return nil, err
	}
	result = append(result, tableau...)
	return result, nil
}

func (f *fairSearch) tableauWitnesses(component []int, members []bool) ([]cycleWitness, error) {
	var result []cycleWitness
	for _, acceptance := range f.acceptance {
		witness, err := f.tableauAcceptanceWitness(component, members, acceptance)
		if err != nil {
			return nil, err
		}
		result = append(result, witness)
	}
	if len(result) == 0 && f.property.mode == generalTemporal {
		witness, err := f.tableauEdgeWitness(component[0], members)
		if err != nil {
			return nil, err
		}
		result = append(result, witness)
	}
	return result, nil
}

func (f *fairSearch) tableauAcceptanceWitness(component []int, members, acceptance []bool) (cycleWitness, error) {
	for _, node := range component {
		if acceptance[node] {
			return f.tableauEdgeWitness(node, members)
		}
	}
	return cycleWitness{}, fmt.Errorf("tableau component has no acceptance witness")
}

func (f *fairSearch) tableauEdgeWitness(node int, members []bool) (cycleWitness, error) {
	for _, edge := range f.search.edges[node] {
		if members[edge.to] {
			return cycleWitness{node: node, edge: &edge}, nil
		}
	}
	return cycleWitness{}, fmt.Errorf("tableau component has no internal edge")
}

func (f *fairSearch) badWitness(component []int) cycleWitness {
	for _, node := range component {
		if f.bad[node] {
			return cycleWitness{node: node}
		}
	}
	panic("violating state was not found")
}

func (f *fairSearch) fairWitness(index int, fair fairnessCondition, component []int, members []bool) (cycleWitness, bool, error) {
	if fair.recurrence {
		if len(f.recurrenceKnown) > index && f.recurrenceKnown[index] {
			return f.recurrenceWitnesses[index], true, nil
		}
		return f.recurrenceWitness(fair, component, members)
	}
	return f.weakStrongFairWitness(index, fair, component, members)
}

func (f *fairSearch) weakStrongFairWitness(index int, fair fairnessCondition, component []int, members []bool) (cycleWitness, bool, error) {
	enabled, err := f.enabledStates(index, fair, component)
	if err != nil {
		return cycleWitness{}, false, err
	}
	if fair.strong && enabled == 0 {
		return cycleWitness{}, false, nil
	}
	if !fair.strong && enabled < len(component) {
		return f.disabledWitness(index, component), true, nil
	}
	witness, ok, err := f.findFairWitness(index, fair, component, members)
	if err != nil {
		return cycleWitness{}, false, err
	}
	if !ok {
		return cycleWitness{}, false, fmt.Errorf("fair component has no fairness witness")
	}
	return witness, true, nil
}

func (f *fairSearch) disabledWitness(index int, component []int) cycleWitness {
	for _, node := range component {
		if f.enabled[index][node] == -1 {
			return cycleWitness{node: node}
		}
	}
	panic("disabled fairness witness was not found")
}

func (f *fairSearch) findFairWitness(index int, fair fairnessCondition, component []int, members []bool) (cycleWitness, bool, error) {
	for _, node := range component {
		witness, ok, err := f.search.findFairEdge(index, fair, node, members)
		if err != nil || ok {
			return witness, ok, err
		}
	}
	return cycleWitness{}, false, nil
}

func (s *search) findFairEdge(index int, fair fairnessCondition, node int, members []bool) (cycleWitness, bool, error) {
	for choice, edge := range s.edges[node] {
		if !members[edge.to] || edge.to == node && edge.next == nil {
			continue
		}
		ok, cached := s.cachedFairAction(index, node, choice)
		var err error
		if !cached {
			ok, err = s.model.holdsFair(fair, s.nodes[node].state, s.edgeNext(edge))
		}
		if err != nil || ok {
			return cycleWitness{node: node, edge: &edge}, ok, err
		}
	}
	return cycleWitness{}, false, nil
}
