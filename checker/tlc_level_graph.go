package checker

import (
	"maps"
	"reflect"
	"slices"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

type tlcGraphLevel struct{ incoming, base, index int }

type levelNode struct{ base, incoming int }

type levelGraphBuilder struct {
	source  *search
	product search
	indices map[levelNode]int
	queue   []levelNode
	levels  map[uintptr]tlcGraphLevel
	values  map[int]map[int]map[*ltlFormula]bool
}

func (r *tlcRuntime) contextLevel(ctx evaluator.Context, source evaluator.State, primed bool) int {
	if r.depthFirst != nil {
		// TLC's depth-first worker never sets the current state, so
		// TLCGet("level") is always the level before the initial states.
		return 0
	}
	if r.aliasEvaluating {
		return r.level
	}
	if r.levelGraph == nil {
		if primed {
			return r.level + 1
		}
		return r.level
	}
	return r.contextGraphLevel(ctx, source, primed)
}

func (r *tlcRuntime) contextGraphLevel(ctx evaluator.Context, source evaluator.State, primed bool) int {
	if primed {
		if level, ok := r.primedGraphLevel(ctx.Current, source); ok {
			return level
		}
	}
	if current, ok := r.lookupLevel(ctx.Current); ok {
		if ctx.Next != nil || r.useBaseLevel {
			return current.base
		}
		return current.incoming
	}
	return r.level
}

func (r *tlcRuntime) primedGraphLevel(current, previous evaluator.State) (int, bool) {
	source, ok := r.lookupLevel(previous)
	if !ok {
		return 0, false
	}
	if target, ok := r.lookupLevel(current); ok && target.index == source.index {
		return source.base, true
	}
	return source.base + 1, true
}

func (r *tlcRuntime) lookupLevel(state evaluator.State) (tlcGraphLevel, bool) {
	if state == nil {
		return tlcGraphLevel{}, false
	}
	level, ok := r.levelGraph[reflect.ValueOf(state).Pointer()]
	return level, ok
}

func (m *model) temporalUsesTLC() bool {
	if slices.ContainsFunc(m.livenessOrder, m.propertyUsesTLC) {
		return true
	}
	for _, fair := range m.fairness {
		if m.expressionUsesTLC(fair.action, map[string]bool{}) {
			return true
		}
	}
	return false
}

func (m *model) propertyUsesTLC(property temporalProperty) bool {
	if m.expressionUsesTLC(property.predicate, map[string]bool{}) || m.expressionUsesTLC(property.trigger, map[string]bool{}) {
		return true
	}
	for _, expr := range property.sequence {
		if m.expressionUsesTLC(expr, map[string]bool{}) {
			return true
		}
	}
	return false
}

func (m *model) expressionUsesTLC(expr *parser.Expression, seen map[string]bool) bool {
	if expr == nil {
		return false
	}
	if expr.Kind == "Apply" && tlcOperatorName(expr) == "TLCGet" {
		return true
	}
	if m.tlcDefinitionUses(expr, seen) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = m.expressionUsesTLC(child, seen)
		}
	})
	return found
}

func (m *model) tlcDefinitionUses(expr *parser.Expression, seen map[string]bool) bool {
	if expr.Kind != "Apply" && expr.Kind != "Identifier" {
		return false
	}
	d, ok := m.defs[expr.Text]
	if !ok || seen[expr.Text] {
		return false
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return m.expressionUsesTLC(d.Body, seen)
}

func (s *search) checkLivenessWithLevels() (Result, error) {
	b := levelGraphBuilder{source: s, product: search{model: s.model, result: s.result, fairChecks: s.fairChecks}, indices: map[levelNode]int{}, levels: map[uintptr]tlcGraphLevel{}}
	b.prepareCachedValues()
	for _, i := range s.initial {
		index := b.add(levelNode{base: i, incoming: 1}, -1, "", nil)
		b.product.initial = append(b.product.initial, index)
	}
	if err := b.expand(); err != nil {
		return s.result, err
	}
	s.model.tlcRuntime.levelGraph = b.levels
	defer func() { s.model.tlcRuntime.levelGraph = nil }()
	result, err := b.product.checkLivenessGraph()
	if err == nil {
		err = b.product.trimLevelLoop(&result)
	}
	return result, err
}

func (b *levelGraphBuilder) add(node levelNode, parent int, action string, source *parser.Expression) int {
	if index, ok := b.indices[node]; ok {
		return index
	}
	index := len(b.product.nodes)
	original := b.source.nodes[node.base]
	state := maps.Clone(original.state)
	b.levels[reflect.ValueOf(state).Pointer()] = tlcGraphLevel{incoming: node.incoming, base: original.depth, index: node.base}
	b.indices[node] = index
	b.queue = append(b.queue, node)
	b.product.nodes = append(b.product.nodes, stateNode{state: state, parent: parent, action: action, source: source, depth: original.depth})
	b.product.edges = append(b.product.edges, nil)
	b.copyCachedValues(node.base, -1, index, -1)
	return index
}

func (b *levelGraphBuilder) expand() error {
	for head := 0; head < len(b.queue); head++ {
		if err := b.source.model.ctx.Err(); err != nil {
			return err
		}
		if err := b.expandEdges(head, false); err != nil {
			return err
		}
		if err := b.expandEdges(head, true); err != nil {
			return err
		}
	}
	return nil
}

func (b *levelGraphBuilder) expandEdges(head int, stuttering bool) error {
	state := b.queue[head]
	for choice, edge := range b.source.edges[state.base] {
		if (edge.action == "Stuttering") != stuttering {
			continue
		}
		level := b.source.nodes[state.base].depth + 1
		if edge.to == state.base {
			level--
		}
		next := levelNode{base: edge.to, incoming: level}
		index := b.add(next, head, edge.action, edge.source)
		concrete, err := b.levelEdgeSuccessor(edge)
		if err != nil {
			return err
		}
		productChoice := len(b.product.edges[head])
		b.product.edges[head] = append(b.product.edges[head], graphEdge{to: index, action: edge.action, source: edge.source, next: concrete})
		b.copyCachedValues(state.base, choice, head, productChoice)
	}
	return nil
}

func (b *levelGraphBuilder) prepareCachedValues() {
	if len(b.source.ltlValues) == 0 {
		return
	}
	b.values = map[int]map[int]map[*ltlFormula]bool{}
	b.product.ltlValues = map[ltlValueKey]bool{}
	for key, value := range b.source.ltlValues {
		if b.values[key.base] == nil {
			b.values[key.base] = map[int]map[*ltlFormula]bool{}
		}
		if b.values[key.base][key.choice] == nil {
			b.values[key.base][key.choice] = map[*ltlFormula]bool{}
		}
		b.values[key.base][key.choice][key.formula] = value
	}
}

func (b *levelGraphBuilder) copyCachedValues(source, choice, target, targetChoice int) {
	for formula, value := range b.values[source][choice] {
		b.product.ltlValues[ltlValueKey{formula: formula, base: target, choice: targetChoice}] = value
	}
}

func (b *levelGraphBuilder) levelEdgeSuccessor(edge graphEdge) (evaluator.State, error) {
	if edge.next == nil {
		return nil, nil
	}
	equal, err := b.source.sameConcreteState(edge.next, b.source.nodes[edge.to].state)
	if equal || err != nil {
		return nil, err
	}
	return edge.next, nil
}
