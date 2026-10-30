package checker

// Without fairness, every reachable bad-state component contains a stuttering
// edge. Find the first component in depth-first completion order without
// constructing the reverse graph used for general fairness checking.
func (s *search) checkUnfairEventually(p temporalProperty) (bool, error) {
	f := fairSearch{search: s, property: p, bad: make([]bool, len(s.nodes)), trigger: make([]bool, len(s.nodes))}
	if err := f.evaluateStates(); err != nil {
		return false, err
	}
	allowed := f.reachableStates()
	root := s.firstBadComponent(allowed)
	if root < 0 {
		return false, nil
	}
	return true, f.recordCycle([]int{root})
}

type (
	componentFrame     struct{ node, next int }
	badComponentSearch struct {
		search     *search
		allowed    []bool
		index, low []int
		serial     int
		stack      []componentFrame
	}
)

func (s *search) firstBadComponent(allowed []bool) int {
	f := badComponentSearch{search: s, allowed: allowed, index: make([]int, len(s.nodes)), low: make([]int, len(s.nodes))}
	for start := range allowed {
		if allowed[start] && f.index[start] == 0 {
			if root := f.visit(start); root >= 0 {
				return root
			}
		}
	}
	return -1
}

func (f *badComponentSearch) visit(start int) int {
	f.stack = append(f.stack, componentFrame{node: start})
	for len(f.stack) > 0 {
		if root, done := f.advance(); done {
			return root
		}
	}
	return -1
}

func (f *badComponentSearch) advance() (int, bool) {
	top := &f.stack[len(f.stack)-1]
	n := top.node
	if f.index[n] == 0 {
		f.serial++
		f.index[n] = f.serial
		f.low[n] = f.serial
	}
	if top.next < len(f.search.edges[n]) {
		f.advanceEdge(top)
		return 0, false
	}
	if f.low[n] == f.index[n] {
		return n, true
	}
	f.stack = f.stack[:len(f.stack)-1]
	if len(f.stack) > 0 {
		parent := f.stack[len(f.stack)-1].node
		f.low[parent] = min(f.low[parent], f.low[n])
	}
	return 0, false
}

func (f *badComponentSearch) advanceEdge(top *componentFrame) {
	n := top.node
	next := f.search.edges[n][top.next].to
	top.next++
	if !f.allowed[next] {
		return
	}
	if f.index[next] == 0 {
		f.stack = append(f.stack, componentFrame{node: next})
		return
	}
	// No component has been completed yet, so each discovered node is active.
	f.low[n] = min(f.low[n], f.index[next])
}
