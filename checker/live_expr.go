package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
)

// liveKind mirrors the classes of TLC's tlc2.tool.liveness.LiveExprNode.
// TLC random simulation (LiveCheck1) uses this representation directly, so
// its normal form, tableau and node order determine the reported trace.
type liveKind uint8

const (
	liveBool liveKind = iota
	liveState
	liveAction
	liveNeg
	liveConj
	liveDisj
	liveEven
	liveAll
	liveNext
)

type liveNode struct {
	kind  liveKind
	value bool
	body  *liveNode
	items []*liveNode
	leaf  *liveLeaf
}

// liveLeaf is an LNState or LNAction. TLC compares leaves by their tag.
type liveLeaf struct {
	tag  int
	eval func(current, next evaluator.State) (bool, error)
}

func liveBoolNode(value bool) *liveNode { return &liveNode{kind: liveBool, value: value} }

func liveUnary(kind liveKind, body *liveNode) *liveNode { return &liveNode{kind: kind, body: body} }

// liveJunction builds a junction without flattening, like TLC's two-argument
// LNConj and LNDisj constructors.
func liveJunction(kind liveKind, items ...*liveNode) *liveNode {
	return &liveNode{kind: kind, items: items}
}

// add flattens nested junctions of the same kind, like addConj and addDisj.
func (n *liveNode) add(item *liveNode) {
	if item.kind == n.kind {
		for _, nested := range item.items {
			n.add(nested)
		}
		return
	}
	n.items = append(n.items, item)
}

func (n *liveNode) level() int {
	switch n.kind {
	case liveBool:
		return 0
	case liveState:
		return 1
	case liveAction, liveNext:
		return 2
	case liveNeg:
		return n.body.level()
	case liveConj, liveDisj:
		return n.itemsLevel()
	}
	return 3
}

func (n *liveNode) itemsLevel() int {
	level := 0
	for _, item := range n.items {
		level = max(level, item.level())
	}
	return level
}

func (n *liveNode) containsAction() bool {
	switch n.kind {
	case liveAction:
		return true
	case liveBool, liveState:
		return false
	case liveConj, liveDisj:
		for _, item := range n.items {
			if item.containsAction() {
				return true
			}
		}
		return false
	}
	return n.body.containsAction()
}

func liveEqual(a, b *liveNode) bool {
	if a.kind != b.kind {
		return false
	}
	switch a.kind {
	case liveBool:
		return a.value == b.value
	case liveState, liveAction:
		return a.leaf.tag == b.leaf.tag
	case liveConj, liveDisj:
		return liveItemsEqual(a.items, b.items)
	}
	return liveEqual(a.body, b.body)
}

func liveItemsEqual(a, b []*liveNode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !liveEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

// aeBody returns P for []<>P.
func (n *liveNode) aeBody() *liveNode {
	if n.kind == liveAll && n.body.kind == liveEven {
		return n.body.body
	}
	return nil
}

// eaBody returns P for <>[]P.
func (n *liveNode) eaBody() *liveNode {
	if n.kind == liveEven && n.body.kind == liveAll {
		return n.body.body
	}
	return nil
}

// eval evaluates the formula on the state at index current of the trace and,
// for an action, the state at index next.
func (n *liveNode) eval(trace *liveTrace, current, next int) (bool, error) {
	switch n.kind {
	case liveBool:
		return n.value, nil
	case liveState, liveAction:
		return trace.leaf(n, current, next)
	case liveNeg:
		result, err := n.body.eval(trace, current, next)
		return !result, err
	case liveConj, liveDisj:
		return n.evalJunction(trace, current, next)
	case liveNext:
		return n.body.eval(trace, next, -1)
	}
	return false, fmt.Errorf("TLC cannot evaluate a temporal formula on a single transition")
}

func (n *liveNode) evalJunction(trace *liveTrace, current, next int) (bool, error) {
	// A conjunction stops at its first false item, a disjunction at its first true item.
	stop := n.kind == liveDisj
	for _, item := range n.items {
		result, err := item.eval(trace, current, next)
		if err != nil || result == stop {
			return result, err
		}
	}
	return !stop, nil
}
