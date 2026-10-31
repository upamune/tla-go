package checker

// dfidWorker ports DFIDWorker.run for one iteration. TLC picks initial states
// and successors at random; the worker takes the first eligible one in
// generation order instead.
type dfidWorker struct {
	search *dfidSearch
	// curLevel is the number of states on the stack below the current one.
	// Like TLC, it stays 1 after a search from an initial state, so traces from
	// later initial states repeat their first state.
	curLevel  int
	stack     []dfidFrame
	nextInit  int
	moreLevel bool
}

// dfidFrame holds a stack state and its successors not completed when found.
type dfidFrame struct {
	node       dfidNode
	successors []dfidNode
	cursor     int
}

func (w *dfidWorker) run() error {
	for !w.search.stopped {
		init, ok := w.chooseInit()
		if !ok {
			return nil
		}
		if more, err := w.searchFrom(init); !more || err != nil {
			return err
		}
	}
	return nil
}

func (w *dfidWorker) chooseInit() (dfidNode, bool) {
	states := &w.search.states
	for w.nextInit < len(w.search.inits) {
		init := w.search.inits[w.nextInit]
		w.nextInit++
		if !states.completed(states.get(init.key)) {
			return init, true
		}
	}
	return dfidNode{}, false
}

// searchFrom explores below an initial state. It returns false when TLC's
// worker would stop after an exception.
func (w *dfidWorker) searchFrom(init dfidNode) (bool, error) {
	w.setFrame(0, init)
	if more, err := w.expand(0); !more || err != nil {
		return more, err
	}
	w.curLevel = 1
	for {
		frame := &w.stack[w.curLevel-1]
		if child, found := w.chooseSuccessor(frame); found {
			if more, err := w.descend(child); !more || err != nil {
				return more, err
			}
		} else if !w.backtrack(frame) {
			return true, nil
		}
	}
}

// chooseSuccessor is DFIDWorker.getNext. Successors that TLC found at a
// shallower level than this path are left to that level.
func (w *dfidWorker) chooseSuccessor(frame *dfidFrame) (dfidNode, bool) {
	states := &w.search.states
	for frame.cursor < len(frame.successors) {
		child := frame.successors[frame.cursor]
		frame.cursor++
		status := states.get(child.key)
		if !states.completed(status) && w.curLevel < dfidLevel(status) {
			return child, true
		}
	}
	return dfidNode{}, false
}

func (w *dfidWorker) descend(child dfidNode) (bool, error) {
	w.setFrame(w.curLevel, child)
	more, err := w.expand(w.curLevel)
	w.curLevel++
	return more, err
}

func (w *dfidWorker) backtrack(frame *dfidFrame) bool {
	w.search.states.setLeveled(frame.node.key)
	if w.curLevel == 1 {
		return false
	}
	w.curLevel--
	return true
}

func (w *dfidWorker) setFrame(index int, node dfidNode) {
	if index == len(w.stack) {
		w.stack = append(w.stack, dfidFrame{})
	}
	frame := &w.stack[index]
	frame.node, frame.successors, frame.cursor = node, frame.successors[:0], 0
}

// expand is DFIDModelChecker.doNext for the state at index. TLC's leaf flag
// is always false here: states are expanded only above the iteration level.
func (w *dfidWorker) expand(index int) (bool, error) {
	frame := &w.stack[index]
	e := newDFIDExpansion(w.search, frame.node)
	e.successors, e.trace = &frame.successors, w.trace
	more, err := e.run()
	w.moreLevel = w.moreLevel || !e.noLeaf
	return more, err
}

// trace is DFIDWorker.printTrace: the stack below curLevel, the expanded
// state, and the successor when there is one.
func (w *dfidWorker) trace(current dfidNode, successor *dfidNode) []TraceStep {
	trace := make([]TraceStep, 0, w.curLevel+2)
	for _, frame := range w.stack[:w.curLevel] {
		trace = append(trace, frame.node.step())
	}
	trace = append(trace, current.step())
	if successor != nil {
		trace = append(trace, successor.step())
	}
	return trace
}

func (n dfidNode) step() TraceStep {
	return TraceStep{Action: n.action, State: n.state, ActionRange: actionSourceRange(n.source)}
}
