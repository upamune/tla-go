package checker

import "slices"

type shortCycleKey struct {
	node int
	mask uint64
}

type shortCycleFrame struct {
	key    shortCycleKey
	parent int
	edge   graphEdge
}

// A short accepting cycle avoids replaying unnecessary transitions when
// rebuilding a liveness trace. The general witness builder remains the fallback.
func (f *fairSearch) recordShortestTableauCycle(component []int) bool {
	if len(f.acceptance) > 15 || len(component) == 0 {
		return false
	}
	members := make([]bool, len(f.search.nodes))
	for _, node := range component {
		members[node] = true
	}
	start := component[0]
	all := uint64(1)<<len(f.acceptance) - 1
	first := shortCycleKey{node: start, mask: f.shortCycleMask(start)}
	queue := []shortCycleFrame{{key: first, parent: -1}}
	seen := map[shortCycleKey]bool{first: true}
	for head := 0; head < len(queue) && len(queue) < 50000; head++ {
		if found := f.advanceShortCycle(head, start, all, members, &queue, seen); found {
			return true
		}
	}
	return false
}

func (f *fairSearch) shortCycleMask(node int) uint64 {
	var mask uint64
	for i, acceptance := range f.acceptance {
		if acceptance[node] {
			mask |= uint64(1) << i
		}
	}
	return mask
}

func (f *fairSearch) advanceShortCycle(head, start int, all uint64, members []bool, queue *[]shortCycleFrame, seen map[shortCycleKey]bool) bool {
	current := (*queue)[head]
	for _, edge := range f.search.edges[current.key.node] {
		if !members[edge.to] {
			continue
		}
		next := shortCycleKey{node: edge.to, mask: current.key.mask | f.shortCycleMask(edge.to)}
		if edge.to == start && next.mask == all {
			f.recordShortCycle(*queue, head, edge)
			return true
		}
		if !seen[next] {
			seen[next] = true
			*queue = append(*queue, shortCycleFrame{key: next, parent: head, edge: edge})
		}
	}
	return false
}

func (f *fairSearch) recordShortCycle(queue []shortCycleFrame, head int, closing graphEdge) {
	var path []shortCycleFrame
	for head > 0 {
		path = append(path, queue[head])
		head = queue[head].parent
	}
	trace := f.prefix(queue[0].key.node)
	loop := len(trace) - 1
	for _, frame := range slices.Backward(path) {
		trace = append(trace, TraceStep{Action: frame.edge.action, State: f.search.nodes[frame.key.node].state, ActionRange: actionSourceRange(frame.edge.source)})
	}
	f.search.result.Status = LivenessViolation
	f.search.result.Violation = &Violation{Name: f.property.name, Trace: trace, LoopStart: &loop, LoopAction: closing.action, LoopActionRange: actionSourceRange(closing.source)}
}
