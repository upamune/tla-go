package checker

func (s *search) generalPropertyGraph(property temporalProperty) (*search, bool, error) {
	if s.supportsReducedRecurrence(property) {
		if order, ok := s.javaLiveOrder(); ok {
			return s.reducedRecurrenceGraph(order), true, nil
		}
	}
	return s, false, s.validateGeneralReduction(property)
}

// Java can check Boolean combinations of recurrence conditions without a
// tableau. Its liveness graph then keeps parallel transitions until their
// target has itself been written to the graph.
func (s *search) supportsReducedRecurrence(property temporalProperty) bool {
	return s.model.cfg.Symmetry != "" && s.model.cfg.View == "" &&
		!s.model.environmentIsImpure() && s.stableFingerprintStates() &&
		s.stableRecurrenceSuccessors() && s.pureReducedFormula(property.general) &&
		recurrenceOnly(property.general)
}

func (s *search) stableRecurrenceSuccessors() bool {
	for _, edges := range s.edges {
		for _, edge := range edges {
			for _, v := range edge.next {
				if !v.StableFingerprint() {
					return false
				}
			}
		}
	}
	return true
}

func recurrenceOnly(formula *ltlFormula) bool {
	if formula == nil {
		return false
	}
	if isFairnessRecurrence(formula) {
		return true
	}
	switch formula.kind {
	case ltlTrue, ltlFalse:
		return true
	case ltlAnd, ltlOr:
		return recurrenceOnly(formula.left) && recurrenceOnly(formula.right)
	default:
		return false
	}
}

func (s *search) reducedRecurrenceGraph(order [][]int) *search {
	graph := *s
	graph.edges = make([][]graphEdge, len(s.edges))
	graph.ltlValues = make(map[ltlValueKey]bool, len(s.ltlValues))
	remap := make([]map[int]int, len(s.edges))
	for node, choices := range order {
		graph.edges[node], remap[node] = reducedRecurrenceEdges(node, s.edges[node], choices)
	}
	copyReducedRecurrenceValues(s.ltlValues, graph.ltlValues, remap)
	return &graph
}

func reducedRecurrenceEdges(node int, edges []graphEdge, choices []int) ([]graphEdge, map[int]int) {
	var result []graphEdge
	seen := map[int]bool{}
	remap := map[int]int{}
	for _, choice := range choices {
		edge := edges[choice]
		// Nodes are expanded in discovery order. TLC has written earlier
		// targets to its live graph, but not the current or later targets.
		if seen[edge.to] && edge.to < node {
			continue
		}
		seen[edge.to] = true
		remap[choice] = len(result)
		result = append(result, edge)
	}
	return result, remap
}

func copyReducedRecurrenceValues(source, target map[ltlValueKey]bool, remap []map[int]int) {
	for key, v := range source {
		if key.choice >= 0 {
			choice, ok := remap[key.base][key.choice]
			if !ok {
				continue
			}
			key.choice = choice
		}
		target[key] = v
	}
}
