package checker

// These transformations follow LiveExprNode.simplify, pushNeg, toDNF,
// flattenSingleJunctions, makeBinary and extractPromises in TLC 1.7.4.
// Junction order is significant: it fixes the tableau and graph order.

func (n *liveNode) simplify() *liveNode {
	switch n.kind {
	case liveNeg:
		body := n.body.simplify()
		if body.kind == liveBool {
			return liveBoolNode(!body.value)
		}
		return liveUnary(liveNeg, body)
	case liveConj, liveDisj:
		return n.simplifyJunction()
	case liveEven, liveAll:
		body := n.body.simplify()
		if body.kind == n.kind {
			body = body.body
		}
		return liveUnary(n.kind, body)
	}
	return n
}

func (n *liveNode) simplifyJunction() *liveNode {
	absorbing := n.kind == liveDisj
	result := &liveNode{kind: n.kind}
	for _, item := range n.items {
		item = item.simplify()
		if item.kind != liveBool {
			result.add(item)
		} else if item.value == absorbing {
			return liveBoolNode(absorbing)
		}
	}
	switch len(result.items) {
	case 0:
		return liveBoolNode(!absorbing)
	case 1:
		return result.items[0]
	}
	return result
}

func (n *liveNode) pushNeg() *liveNode {
	switch n.kind {
	case liveBool:
		return liveBoolNode(!n.value)
	case liveNeg:
		return n.body
	case liveConj, liveDisj:
		result := &liveNode{kind: liveDualJunction(n.kind)}
		for _, item := range n.items {
			result.add(item.pushNeg())
		}
		return result
	case liveEven:
		return liveUnary(liveAll, n.body.pushNeg())
	case liveAll:
		return liveUnary(liveEven, n.body.pushNeg())
	}
	return liveUnary(liveNeg, n)
}

func liveDualJunction(kind liveKind) liveKind {
	if kind == liveConj {
		return liveDisj
	}
	return liveConj
}

func (n *liveNode) toDNF() *liveNode {
	switch n.kind {
	case liveNeg:
		if n.body.kind == liveState || n.body.kind == liveAction {
			return n
		}
		return n.body.pushNeg().toDNF()
	case liveConj:
		return n.conjunctionToDNF()
	case liveDisj:
		return n.disjunctionToDNF()
	}
	return n
}

func (n *liveNode) conjunctionToDNF() *liveNode {
	var items []*liveNode
	total := 1
	for _, item := range n.items {
		item = item.toDNF()
		switch item.kind {
		case liveDisj:
			items = append(items, item)
			total *= len(item.items)
		case liveConj:
			items = append(items, item.items...)
		default:
			items = append(items, item)
		}
	}
	if total == 1 {
		return &liveNode{kind: liveConj, items: items}
	}
	return liveDistribute(items, total)
}

// liveDistribute expands a conjunction of disjunctions in TLC's order: the
// first disjunction varies slowest.
func liveDistribute(items []*liveNode, total int) *liveNode {
	result := make([]*liveNode, total)
	for i := range result {
		result[i] = &liveNode{kind: liveConj}
	}
	copies, repeat := 1, total
	for _, item := range items {
		if item.kind != liveDisj {
			for _, conjunction := range result {
				conjunction.add(item)
			}
			continue
		}
		repeat /= len(item.items)
		liveSpread(result, item, copies, repeat)
		copies *= len(item.items)
	}
	return &liveNode{kind: liveDisj, items: result}
}

func liveSpread(result []*liveNode, disjunction *liveNode, copies, repeat int) {
	index := 0
	for range copies {
		for _, item := range disjunction.items {
			for range repeat {
				result[index].add(item)
				index++
			}
		}
	}
}

// TLC merges every []<>P disjunct into one []<>(P1 \/ ... \/ Pn).
func (n *liveNode) disjunctionToDNF() *liveNode {
	recurrent, result := &liveNode{kind: liveDisj}, &liveNode{kind: liveDisj}
	for _, item := range n.items {
		item = item.toDNF()
		parts := []*liveNode{item}
		if item.kind == liveDisj {
			parts = item.items
		}
		for _, part := range parts {
			if body := part.aeBody(); body != nil {
				recurrent.add(body)
			} else {
				result.add(part)
			}
		}
	}
	switch len(recurrent.items) {
	case 0:
	case 1:
		result.add(liveUnary(liveAll, liveUnary(liveEven, recurrent.items[0])))
	default:
		result.add(liveUnary(liveAll, liveUnary(liveEven, recurrent)))
	}
	return result
}

func (n *liveNode) flattenSingleJunctions() *liveNode {
	switch n.kind {
	case liveNeg:
		if n.body.kind == liveNeg {
			return n.body.body.flattenSingleJunctions()
		}
		return liveUnary(liveNeg, n.body.flattenSingleJunctions())
	case liveConj, liveDisj:
		if len(n.items) == 1 {
			return n.items[0].flattenSingleJunctions()
		}
		result := &liveNode{kind: n.kind}
		for _, item := range n.items {
			result.add(item.flattenSingleJunctions())
		}
		return result
	case liveEven, liveAll, liveNext:
		return liveUnary(n.kind, n.body.flattenSingleJunctions())
	}
	return n
}

func (n *liveNode) makeBinary() *liveNode {
	switch n.kind {
	case liveNeg, liveEven, liveAll, liveNext:
		return liveUnary(n.kind, n.body.makeBinary())
	case liveConj, liveDisj:
		if len(n.items) == 1 {
			return n.items[0].makeBinary()
		}
		middle := len(n.items) / 2
		left, right := &liveNode{kind: n.kind}, &liveNode{kind: n.kind}
		for _, item := range n.items[:middle] {
			left.add(item)
		}
		for _, item := range n.items[middle:] {
			right.add(item)
		}
		return liveJunction(n.kind, left.makeBinary(), right.makeBinary())
	}
	return n
}

func (n *liveNode) extractPromises(promises *livePar) {
	switch n.kind {
	case liveEven:
		if !promises.member(n) {
			promises.items = append(promises.items, n)
		}
		n.body.extractPromises(promises)
	case liveNeg, liveAll, liveNext:
		n.body.extractPromises(promises)
	case liveConj, liveDisj:
		n.items[0].extractPromises(promises)
		n.items[1].extractPromises(promises)
	}
}
