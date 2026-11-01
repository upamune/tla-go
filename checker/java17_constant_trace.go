package checker

// tryJava17ConstantTableau retains TLC's particle identities and promises
// when reproducing its Boolean-particle omission. Replacing the tableau with
// TRUE finds a violation but loses the graph needed to recover TLC's trace.
func (s *search) tryJava17ConstantTableau(property temporalProperty) (bool, bool, error) {
	if property.java17Tableau == nil || !s.java17TraceEligible() {
		return false, false, nil
	}
	solution := newLiveSolution(property.java17Tableau, nil)
	if !java17TraceGraphWithinLimit(len(s.nodes), len(solution.tableau.nodes)) {
		return false, false, nil
	}
	order, ok := s.javaLiveOrder()
	if !ok {
		return false, false, nil
	}
	found, err := s.checkJava17ConstantTableau(property, &solution, order)
	return found, true, err
}

func (s *search) checkJava17ConstantTableau(property temporalProperty, solution *liveSolution, order [][]int) (bool, error) {
	b := reducedTableauProduct{source: s, graph: search{model: s.model, result: s.result}, indices: map[string]int{}, byBase: make([][]int, len(s.nodes)), done: make([]bool, len(s.nodes))}
	b.tableau = java17ReducedTableau(solution)
	if err := b.buildInOrder(order); err != nil {
		return false, err
	}
	f := fairSearch{search: &b.graph, property: property, bad: make([]bool, len(b.nodes)), trigger: make([]bool, len(b.nodes)), enabled: make([][]int8, len(s.model.fairness)), acceptance: java17TableauAcceptance(solution, b.nodes), compactWitnesses: true}
	for i := range f.enabled {
		if !s.model.fairness[i].recurrence {
			f.enabled[i] = make([]int8, len(b.nodes))
		}
	}
	allowed := f.reachableStates()
	found, err := f.findReducedTableauCycle(allowed)
	if found {
		s.result = b.graph.result
		s.reducedTableauTrace = true
	}
	return found, err
}

// The product can contain one node per state and tableau particle. Keep the
// additional graph bounded; the existing path still reports the violation.
func java17TraceGraphWithinLimit(states, particles int) bool {
	return particles > 0 && states <= 1_000_000/particles
}

// Multiple properties and recurrence fairness are combined before Java builds
// its tableau; processing each property separately can select another trace.
func (s *search) java17TraceEligible() bool {
	if len(s.model.livenessOrder) != 1 || !s.stableFingerprintStates() || !s.stableRecurrenceSuccessors() {
		return false
	}
	for _, fair := range s.model.fairness {
		if fair.recurrence {
			return false
		}
	}
	return true
}

// Particle closure can branch exponentially on eventualities and
// disjunctions. Leave large formulas on the existing bounded path.
func java17TraceTableauFormula(formula *liveNode) *liveNode {
	if java17TraceTableauWithinLimit(formula) {
		return formula
	}
	return nil
}

func java17TraceTableauWithinLimit(formula *liveNode) bool {
	pending := []*liveNode{formula}
	nodes, branches := 0, 0
	for len(pending) > 0 {
		n := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if n == nil {
			continue
		}
		nodes++
		if n.kind == liveEven || n.kind == liveDisj {
			branches++
		}
		if nodes > 64 || branches > 8 {
			return false
		}
		pending = append(pending, n.body)
		pending = append(pending, n.items...)
	}
	return true
}

func java17ReducedTableau(solution *liveSolution) reducedTableau {
	t := solution.tableau
	result := reducedTableau{alternatives: make([]reducedTableauAlternative, len(t.nodes)), next: make([][]int, len(t.nodes))}
	for i := 0; i < t.initial; i++ {
		result.initial = append(result.initial, i)
	}
	for i, node := range t.nodes {
		for _, target := range node.next {
			result.next[i] = append(result.next[i], target.index)
		}
	}
	return result
}

func java17TableauAcceptance(solution *liveSolution, nodes []reducedTableauProductNode) [][]bool {
	result := make([][]bool, len(solution.promises))
	for i, promise := range solution.promises {
		result[i] = make([]bool, len(nodes))
		for j, node := range nodes {
			result[i][j] = solution.tableau.nodes[node.tableau].par.fulfills(promise)
		}
	}
	return result
}
