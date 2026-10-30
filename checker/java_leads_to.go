package checker

import (
	"fmt"
	"slices"
)

// Java TLC's tableau keeps searching for the trigger until it chooses an
// occurrence, then follows only states where the consequent is false.
// Keeping these phases separate matters even when they share a state value.
func (f *fairSearch) canUseJavaLeadsTo() bool {
	m := f.search.model
	return f.property.mode == leadsTo && f.supportsJavaLeadsToFairness() && m.tlcRuntime.levelGraph == nil &&
		!m.hasReduction() && !m.environmentIsImpure() && !m.unsafeIncrementalExpression(f.property.trigger) &&
		!m.unsafeIncrementalExpression(f.property.predicate) && f.search.stableFingerprintStates()
}

func (f *fairSearch) supportsJavaLeadsToFairness() bool {
	for _, fair := range f.search.model.fairness {
		if fair.strong || fair.recurrence || f.search.model.unstableFairExpression(fair.action) || f.search.model.unstableFairExpression(fair.subscript) {
			return false
		}
	}
	return true
}

func (f *fairSearch) checkJavaLeadsTo() (bool, bool, error) {
	if !slices.Contains(f.trigger, true) {
		return false, true, nil
	}
	s := f.search
	order, ok := s.javaLiveOrder()
	if !ok {
		return false, false, nil
	}
	product, productOrder := f.javaLeadsToProduct(order)
	count := len(s.nodes)
	if len(s.model.fairness) > 0 {
		f.orderLateJavaLeadsToNodes(&product, productOrder, count)
	}
	fallback := false
	found, err := product.visitJavaLiveComponents(productOrder, func(root int, component []int) (bool, error) {
		if root < count {
			return false, nil
		}
		f.debugJavaLeadsToComponent(root, component, count)
		if len(s.model.fairness) > 0 {
			var found bool
			var err error
			found, fallback, err = f.javaLeadsToFairComponent(&product, productOrder, root, component, count)
			return found || fallback, err
		}
		if !product.stutters(root) {
			fallback = len(component) > 1
			return fallback, nil
		}
		return true, f.recordJavaLeadsTo(&product, productOrder, root)
	})
	// A non-stuttering cycle needs the general witness builder.
	if fallback {
		return false, false, nil
	}
	return found, true, err
}

func (f *fairSearch) javaLeadsToFairComponent(product *search, order [][]int, root int, component []int, count int) (bool, bool, error) {
	base := make([]int, len(component))
	for i, node := range component {
		base[i] = node - count
	}
	valid, pruned, err := f.checkComponent(base)
	f.debugComponent(base, valid, pruned != nil)
	if !valid || err != nil {
		return false, false, err
	}
	return true, false, f.recordJavaFairCycle(product, order, root, component, base, count)
}

func (f *fairSearch) javaLeadsToProduct(order [][]int) (search, [][]int) {
	s := f.search
	count := len(s.nodes)
	product := search{model: s.model, initial: s.initial, nodes: make([]stateNode, count*2), edges: make([][]graphEdge, count*2)}
	productOrder := make([][]int, count*2)
	f.allocateJavaLeadsToEdges(&product, productOrder, order, count)
	for node := range s.nodes {
		product.nodes[node], product.nodes[node+count] = s.nodes[node], s.nodes[node]
		f.addJavaLeadsToEdges(&product, productOrder, node, count, order[node])
	}
	return product, productOrder
}

func (f *fairSearch) allocateJavaLeadsToEdges(product *search, order, choices [][]int, count int) {
	sizes := make([]int, count*2)
	for node, nodeChoices := range choices {
		sizes[node], sizes[node+count] = f.javaLeadsToEdgeSizes(node, nodeChoices)
	}
	total := 0
	for _, size := range sizes {
		total += size
	}
	edges, indices := make([]graphEdge, total), make([]int, total)
	offset := 0
	for node, size := range sizes {
		end := offset + size
		product.edges[node] = edges[offset:offset:end]
		order[node] = indices[offset:offset:end]
		offset = end
	}
}

func (f *fairSearch) javaLeadsToEdgeSizes(node int, choices []int) (int, int) {
	normal, pending := len(choices), 0
	if !f.bad[node] {
		return normal, pending
	}
	for _, choice := range choices {
		if f.bad[f.search.edges[node][choice].to] {
			pending++
		}
	}
	if f.trigger[node] {
		normal += pending
	}
	return normal, pending
}

func (f *fairSearch) addJavaLeadsToEdges(product *search, order [][]int, node, count int, choices []int) {
	for _, choice := range choices {
		edge := f.search.edges[node][choice]
		appendJavaLeadsToEdge(product, order, node, edge)
		if f.bad[node] && f.bad[edge.to] {
			pending := edge
			pending.to += count
			appendJavaLeadsToEdge(product, order, node+count, pending)
			if f.trigger[node] {
				appendJavaLeadsToEdge(product, order, node, pending)
			}
		}
	}
}

func appendJavaLeadsToEdge(product *search, order [][]int, node int, edge graphEdge) {
	order[node] = append(order[node], len(product.edges[node]))
	product.edges[node] = append(product.edges[node], edge)
}

func (f *fairSearch) recordJavaLeadsTo(product *search, order [][]int, root int) error {
	trace, err := product.javaLeadsToPath(order, root, len(f.search.nodes))
	if err != nil {
		return err
	}
	loop := len(trace) - 1
	f.search.result.Status = LivenessViolation
	f.search.result.Violation = &Violation{Name: f.property.name, Trace: trace, LoopStart: &loop, LoopAction: "Stuttering"}
	return nil
}

func (s *search) javaLeadsToPath(order [][]int, target, count int) ([]TraceStep, error) {
	parents := make([]traceParent, len(s.nodes))
	queue := make([]int, 0, len(s.nodes))
	for _, initial := range s.initial {
		if !parents[initial].seen {
			parents[initial] = traceParent{index: -1, seen: true}
			queue = append(queue, initial)
		}
	}
	for head := 0; head < len(queue) && !parents[target].seen; head++ {
		s.javaLivePathNeighbors(queue[head], nil, order, parents, &queue)
	}
	if !parents[target].seen {
		return nil, fmt.Errorf("no path in TLC leads-to graph")
	}
	var path []int
	for node := target; node >= 0; node = parents[node].index {
		path = append(path, node)
	}
	slices.Reverse(path)
	return s.javaLeadsToTrace(path, parents, count), nil
}

func (s *search) javaLeadsToTrace(path []int, parents []traceParent, count int) []TraceStep {
	trace := make([]TraceStep, 0, len(path))
	previous := -1
	for _, node := range path {
		base := node % count
		if base != previous {
			trace = append(trace, TraceStep{Action: parents[node].action, State: s.nodes[node].state, ActionRange: actionSourceRange(parents[node].source)})
		}
		previous = base
	}
	return trace
}
