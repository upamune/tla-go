package checker

import (
	"fmt"
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

func (s *search) checkJavaStability(property temporalProperty) (bool, bool, error) {
	if s.model.hasReduction() || !s.stableFingerprintStates() {
		return false, false, nil
	}
	bad, any, err := s.javaStabilityStates(property)
	if err != nil || !any {
		return false, true, err
	}
	order, ok := s.javaLiveOrder()
	if !ok {
		return false, false, nil
	}
	found, err := s.checkJavaEventuallyAlways(property, bad, order)
	return found, true, err
}

func (s *search) javaStabilityStates(property temporalProperty) ([]bool, bool, error) {
	bad := make([]bool, len(s.nodes))
	any := false
	for node := range s.nodes {
		if err := s.model.ctx.Err(); err != nil {
			return nil, false, err
		}
		holds, cached := s.cachedPureRecurrence(property, node)
		var err error
		if !cached {
			holds, err = s.model.holdsProperty(property.predicate, s.nodes[node].state, property.locals)
		}
		if err != nil {
			return nil, false, err
		}
		bad[node] = !holds
		any = any || bad[node]
	}
	return bad, any, nil
}

func (s *search) checkJavaEventuallyAlways(property temporalProperty, bad []bool, order [][]int) (bool, error) {
	return s.visitJavaLiveComponents(order, func(root int, component []int) (bool, error) {
		return s.checkJavaStabilityComponent(property, root, component, bad, order)
	})
}

func (s *search) checkJavaStabilityComponent(property temporalProperty, root int, component []int, bad []bool, order [][]int) (bool, error) {
	if len(component) == 1 && !s.stutters(root) {
		return false, nil
	}
	found := false
	for _, node := range component {
		found = found || bad[node]
	}
	if !found {
		return false, nil
	}
	members := make([]bool, len(s.nodes))
	for _, node := range component {
		members[node] = true
	}
	return true, s.recordJavaStabilityCycle(property.name, root, bad, members, order)
}

func (s *search) recordJavaStabilityCycle(name string, root int, bad, members []bool, order [][]int) error {
	trace, err := s.javaLivePath(s.initial, root, nil, order)
	if err != nil {
		return err
	}
	loop := len(trace) - 1
	path, target, err := s.javaStabilityWitness(root, bad, members, order)
	if err != nil {
		return err
	}
	for _, edge := range path {
		trace = append(trace, TraceStep{Action: edge.action, State: s.nodes[edge.to].state, ActionRange: actionSourceRange(edge.source)})
	}
	closing := "Stuttering"
	var closingRange parser.Range
	if target != root {
		back, err := s.javaLivePath([]int{target}, root, members, order)
		if err != nil {
			return err
		}
		closing = back[len(back)-1].Action
		closingRange = back[len(back)-1].ActionRange
		trace = append(trace, back[1:len(back)-1]...)
	}
	s.result.Status = LivenessViolation
	s.result.Violation = &Violation{Name: name, Trace: trace, LoopStart: &loop, LoopAction: closing, LoopActionRange: closingRange}
	return nil
}

// javaLivePath uses the same FIFO successor order as DiskGraph.getPath and
// LiveWorker's cycle-closing search.
func (s *search) javaLivePath(starts []int, target int, members []bool, order [][]int) ([]TraceStep, error) {
	parents := make([]traceParent, len(s.nodes))
	queue := make([]int, 0, len(s.nodes))
	for _, start := range starts {
		if !parents[start].seen {
			parents[start] = traceParent{index: -1, seen: true}
			queue = append(queue, start)
		}
	}
	for head := 0; head < len(queue) && !parents[target].seen; head++ {
		s.javaLivePathNeighbors(queue[head], members, order, parents, &queue)
	}
	if !parents[target].seen {
		return nil, fmt.Errorf("no path in TLC liveness graph")
	}
	var path []TraceStep
	for node := target; node >= 0; node = parents[node].index {
		path = append(path, TraceStep{Action: parents[node].action, State: s.nodes[node].state, ActionRange: actionSourceRange(parents[node].source)})
	}
	slices.Reverse(path)
	return path, nil
}

func (s *search) javaLivePathNeighbors(node int, members []bool, order [][]int, parents []traceParent, queue *[]int) {
	for _, choice := range order[node] {
		edge := s.edges[node][choice]
		if parents[edge.to].seen || members != nil && !members[edge.to] {
			continue
		}
		parents[edge.to] = traceParent{index: node, action: edge.action, source: edge.source, seen: true}
		*queue = append(*queue, edge.to)
	}
}

func (s *search) javaStabilityWitness(root int, bad, members []bool, order [][]int) ([]graphEdge, int, error) {
	visited := make([]bool, len(s.nodes))
	visited[root] = true
	nodes := []int{root}
	var path []graphEdge
	last := true
	for len(nodes) > 0 {
		node := nodes[len(nodes)-1]
		if bad[node] {
			return path, node, nil
		}
		edge, ok := s.javaWitnessNeighbor(node, visited, members, order, last)
		if ok {
			visited[edge.to] = true
			nodes = append(nodes, edge.to)
			path = append(path, edge)
			last = true
			continue
		}
		nodes = nodes[:len(nodes)-1]
		if len(path) > 0 {
			path = path[:len(path)-1]
		}
		last = false
	}
	return nil, 0, fmt.Errorf("no stability witness in TLC liveness component")
}

func (s *search) javaWitnessNeighbor(node int, visited, members []bool, order [][]int, last bool) (graphEdge, bool) {
	var result graphEdge
	found := false
	for _, choice := range order[node] {
		edge := s.edges[node][choice]
		if !members[edge.to] || visited[edge.to] {
			continue
		}
		result, found = edge, true
		if !last {
			break
		}
	}
	return result, found
}
