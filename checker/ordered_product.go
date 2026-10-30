package checker

import "github.com/upamune/tla-go/syntax/parser"

type orderedState struct {
	node  int
	stage int
}

type orderedBuilder struct {
	source   *search
	product  search
	property temporalProperty
	indices  map[orderedState]int
	queue    []orderedState
	observed []orderedObservation
}

type orderedObservation struct {
	value, known bool
}

// The product tracks how many ordered predicates have been satisfied.
// Paths that satisfy every predicate are removed.
func (s *search) orderedProduct(p temporalProperty) (*search, error) {
	b := orderedBuilder{source: s, property: p, indices: map[orderedState]int{}, observed: make([]orderedObservation, len(s.nodes)*len(p.sequence)), product: search{model: s.model, result: s.result, fairChecks: s.fairChecks}}
	for _, initial := range s.initial {
		state, keep, err := b.advance(initial, 0)
		if err != nil {
			return nil, err
		}
		if keep {
			index := b.add(state, -1, "", nil)
			b.product.initial = append(b.product.initial, index)
		}
	}
	if err := b.expand(); err != nil {
		return nil, err
	}
	return &b.product, nil
}

func (b *orderedBuilder) advance(index, stage int) (orderedState, bool, error) {
	for stage < len(b.property.sequence) {
		holds, err := b.holdsAt(index, stage)
		if err != nil {
			return orderedState{}, false, err
		}
		if !holds {
			return orderedState{node: index, stage: stage}, true, nil
		}
		stage++
	}
	return orderedState{}, false, nil
}

func (b *orderedBuilder) add(state orderedState, parent int, action string, source *parser.Expression) int {
	if index, ok := b.indices[state]; ok {
		return index
	}
	index := len(b.product.nodes)
	b.indices[state] = index
	b.queue = append(b.queue, state)
	b.product.nodes = append(b.product.nodes, stateNode{state: b.source.nodes[state.node].state, parent: parent, action: action, source: source})
	b.product.edges = append(b.product.edges, nil)
	copyFairStateValues(b.source, &b.product, index, state.node)
	return index
}

func (b *orderedBuilder) expand() error {
	for head := 0; head < len(b.queue); head++ {
		if err := b.source.model.ctx.Err(); err != nil {
			return err
		}
		if err := b.expandState(head, b.queue[head]); err != nil {
			return err
		}
	}
	return nil
}

func (b *orderedBuilder) expandState(index int, state orderedState) error {
	for baseChoice, edge := range b.source.edges[state.node] {
		next, keep, err := b.advance(edge.to, state.stage)
		if err != nil {
			return err
		}
		if !keep {
			continue
		}
		target := b.add(next, index, edge.action, edge.source)
		choice := len(b.product.edges[index])
		b.product.edges[index] = append(b.product.edges[index], graphEdge{to: target, action: edge.action, source: edge.source, next: edge.next})
		copyFairActionValues(b.source, &b.product, index, choice, state.node, baseChoice)
	}
	return nil
}

func (b *orderedBuilder) holdsAt(index, stage int) (bool, error) {
	cached := &b.observed[index*len(b.property.sequence)+stage]
	if cached.known {
		return cached.value, nil
	}
	result, err := b.source.model.holdsProperty(b.property.sequence[stage], b.source.nodes[index].state, b.property.locals)
	if err == nil {
		cached.value, cached.known = result, true
	}
	return result, err
}
