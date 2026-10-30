package checker

type javaLiveFrame struct {
	node, next int
}

// javaLiveComponents visits components in the order of TLC LiveWorker's
// depth-first search: initial states first and hash-slot successors last first.
type javaLiveComponents struct {
	search *search
	order  [][]int
	index  []int
	low    []int
	active []bool
	stack  []int
	frames []javaLiveFrame
	serial int
	visit  func(int, []int) (bool, error)
}

func (s *search) visitJavaLiveComponents(order [][]int, visit func(int, []int) (bool, error)) (bool, error) {
	w := javaLiveComponents{search: s, order: order, index: make([]int, len(s.nodes)), low: make([]int, len(s.nodes)), active: make([]bool, len(s.nodes)), visit: visit}
	for i := range w.index {
		w.index[i] = -1
	}
	for _, root := range s.initial {
		if w.index[root] < 0 {
			found, err := w.explore(root)
			if found || err != nil {
				return found, err
			}
		}
	}
	return false, nil
}

func (w *javaLiveComponents) enter(node int) {
	w.index[node], w.low[node] = w.serial, w.serial
	w.serial++
	w.active[node] = true
	w.stack = append(w.stack, node)
	w.frames = append(w.frames, javaLiveFrame{node: node, next: len(w.order[node]) - 1})
}

func (w *javaLiveComponents) explore(root int) (bool, error) {
	w.enter(root)
	for len(w.frames) > 0 {
		if err := w.search.model.ctx.Err(); err != nil {
			return false, err
		}
		frame := &w.frames[len(w.frames)-1]
		if frame.next >= 0 {
			w.follow(frame)
			continue
		}
		found, err := w.finish(frame.node)
		if found || err != nil {
			return found, err
		}
	}
	return false, nil
}

func (w *javaLiveComponents) follow(frame *javaLiveFrame) {
	node := frame.node
	choice := w.order[node][frame.next]
	frame.next--
	next := w.search.edges[node][choice].to
	if w.index[next] < 0 {
		w.enter(next)
	} else if w.active[next] {
		w.low[node] = min(w.low[node], w.index[next])
	}
}

func (w *javaLiveComponents) finish(node int) (bool, error) {
	w.frames = w.frames[:len(w.frames)-1]
	if len(w.frames) > 0 {
		parent := w.frames[len(w.frames)-1].node
		w.low[parent] = min(w.low[parent], w.low[node])
	}
	if w.low[node] != w.index[node] {
		return false, nil
	}
	var component []int
	for {
		n := w.stack[len(w.stack)-1]
		w.stack = w.stack[:len(w.stack)-1]
		w.active[n] = false
		component = append(component, n)
		if n == node {
			break
		}
	}
	return w.visit(node, component)
}
