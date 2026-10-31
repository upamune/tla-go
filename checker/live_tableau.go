package checker

import (
	"slices"
)

// livePar is a TBPar: an ordered particle whose members are compared with
// liveEqual. Some closure steps update a particle in place, as TLC does.
type livePar struct{ items []*liveNode }

type liveTriple struct{ whole, first, second *liveNode }

type liveClosure struct{ alphas, betas []liveTriple }

// liveTableau is TLC's TBGraph. Node indices are their creation order.
type liveTableau struct {
	nodes   []*liveTableauNode
	initial int
}

type liveTableauNode struct {
	index      int
	par        *livePar
	next       []*liveTableauNode
	statePreds []*liveNode
}

func (p *livePar) member(n *liveNode) bool {
	return slices.ContainsFunc(p.items, func(item *liveNode) bool { return liveEqual(n, item) })
}

func (p *livePar) contains(other *livePar) bool {
	for _, item := range other.items {
		if !p.member(item) {
			return false
		}
	}
	return true
}

func (p *livePar) equal(other *livePar) bool { return p.contains(other) && other.contains(p) }

func (p *livePar) appended(items ...*liveNode) *livePar {
	return &livePar{items: append(slices.Clip(slices.Clone(p.items)), items...)}
}

func (p *livePar) fulfills(promise *liveNode) bool {
	return !p.member(promise) || p.member(promise.body)
}

func (p *livePar) locallyConsistent() bool {
	var positive, negative livePar
	for _, item := range p.items {
		if item.kind == liveState {
			positive.items = append(positive.items, item)
		} else if item.kind == liveNeg && item.body.kind == liveState {
			negative.items = append(negative.items, item.body)
		}
	}
	return !slices.ContainsFunc(positive.items, negative.member)
}

func (p *livePar) positiveClosure() *livePar {
	pending := slices.Clone(p.items)
	result := &livePar{}
	for len(pending) > 0 {
		item := pending[len(pending)-1]
		pending = result.addPositive(item, pending[:len(pending)-1])
	}
	return result
}

func (p *livePar) addPositive(item *liveNode, pending []*liveNode) []*liveNode {
	switch item.kind {
	case liveNeg:
		return append(pending, item.body)
	case liveNext:
		p.items = append(p.items, item)
		return append(pending, item.body)
	case liveEven, liveAll:
		p.items = append(p.items, item, liveUnary(liveNext, item))
		return append(pending, item.body)
	case liveConj, liveDisj:
		pending = append(pending, item.items...)
	}
	p.items = append(p.items, item)
	return pending
}

func (p *livePar) triples(alpha bool) []liveTriple {
	var result []liveTriple
	for _, item := range p.items {
		if first, second := liveTripleParts(item, alpha); first != nil {
			result = append(result, liveTriple{whole: item, first: first, second: second})
		}
	}
	return result
}

// liveTripleParts splits [] and /\ (alpha) or <> and \/ (beta) formulas.
func liveTripleParts(item *liveNode, alpha bool) (*liveNode, *liveNode) {
	temporal, junction := liveAll, liveConj
	if !alpha {
		temporal, junction = liveEven, liveDisj
	}
	switch item.kind {
	case temporal:
		return item.body, liveUnary(liveNext, item)
	case junction:
		return item.items[0], item.items[1]
	}
	return nil, nil
}

func (p *livePar) impliedSuccessors() *livePar {
	result := &livePar{}
	for _, item := range p.items {
		if item.kind == liveNext {
			result.items = append(result.items, item.body)
		}
	}
	return result
}

func (p *livePar) particleClosure() []*livePar {
	positive := p.positiveClosure()
	closure := liveClosure{alphas: positive.triples(true), betas: positive.triples(false)}
	return closure.particles(p)
}

func (c liveClosure) particles(terms *livePar) []*livePar {
	if !terms.locallyConsistent() {
		return nil
	}
	expanded := terms
	for i := 0; i < len(expanded.items); i++ {
		if first, second := liveTripleParts(expanded.items[i], true); first != nil {
			expanded = expanded.withBoth(first, second)
		}
	}
	c.saturate(expanded)
	// Without new alpha parts TLC updates terms itself, so it skips this check.
	if len(expanded.items) > len(terms.items) && !expanded.locallyConsistent() {
		return nil
	}
	return c.particlesBeta(expanded)
}

func (p *livePar) withBoth(first, second *liveNode) *livePar {
	switch {
	case p.member(first) && p.member(second):
		return p
	case p.member(first):
		return p.appended(second)
	case p.member(second):
		return p.appended(first)
	}
	return p.appended(first, second)
}

func (c liveClosure) saturate(terms *livePar) {
	for changed := true; changed; {
		changed = false
		for _, alpha := range c.alphas {
			if terms.member(alpha.first) && terms.member(alpha.second) && !terms.member(alpha.whole) {
				terms.items = append(terms.items, alpha.whole)
				changed = true
			}
		}
	}
}

func (c liveClosure) particlesBeta(terms *livePar) []*livePar {
	if first, second := terms.unexpandedBeta(); first != nil {
		return liveParticleUnion(c.particles(terms.appended(first)), c.particles(terms.appended(second)))
	}
	for _, beta := range c.betas {
		if (terms.member(beta.first) || terms.member(beta.second)) && !terms.member(beta.whole) {
			return c.particles(terms.appended(beta.whole))
		}
	}
	return []*livePar{terms}
}

// unexpandedBeta returns the parts of the first <> or \/ formula of which
// the particle contains neither part.
func (p *livePar) unexpandedBeta() (*liveNode, *liveNode) {
	for _, item := range p.items {
		first, second := liveTripleParts(item, false)
		if first != nil && !p.member(first) && !p.member(second) {
			return first, second
		}
	}
	return nil, nil
}

func liveParticleUnion(first, second []*livePar) []*livePar {
	result := make([]*livePar, 0, len(first)+len(second))
	for _, par := range first {
		if !slices.ContainsFunc(second, par.equal) {
			result = append(result, par)
		}
	}
	return append(result, second...)
}

func newLiveTableau(formula *liveNode) *liveTableau {
	t := &liveTableau{}
	for _, par := range (&livePar{items: []*liveNode{formula}}).particleClosure() {
		t.nodes = append(t.nodes, newLiveTableauNode(par))
	}
	t.initial = len(t.nodes)
	for i := 0; i < len(t.nodes); i++ {
		source := t.nodes[i]
		for _, par := range source.par.impliedSuccessors().particleClosure() {
			source.next = append(source.next, t.node(par))
		}
	}
	for i, node := range t.nodes {
		node.index = i
	}
	return t
}

func (t *liveTableau) node(par *livePar) *liveTableauNode {
	for _, node := range t.nodes {
		if par.equal(node.par) {
			return node
		}
	}
	node := newLiveTableauNode(par)
	t.nodes = append(t.nodes, node)
	return node
}

func newLiveTableauNode(par *livePar) *liveTableauNode {
	node := &liveTableauNode{par: par}
	for _, item := range par.items {
		if item.kind == liveState || item.kind == liveNeg && item.body.kind == liveState {
			node.statePreds = append(node.statePreds, item)
		}
	}
	return node
}

func (n *liveTableauNode) consistent(trace *liveTrace, state int) (bool, error) {
	for _, predicate := range n.statePreds {
		holds, err := predicate.eval(trace, state, -1)
		if err != nil || !holds {
			return false, err
		}
	}
	return true, nil
}
