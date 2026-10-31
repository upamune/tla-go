package checker

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

type reducedTableau struct {
	alternatives []reducedTableauAlternative
	next         [][]int
	initial      []int
	indices      map[string]int
}

type reducedTableauAlternative struct {
	ltlAlternative
	conditions []*ltlFormula
}

// The live graph identifies nodes by the reduced state and tableau state.
// Java regenerates successors from the concrete state if that pair first
// appears after the reduced state was explored.
type reducedTableauProduct struct {
	source  *search
	graph   search
	tableau reducedTableau
	nodes   []reducedTableauProductNode
	byBase  [][]int
	indices map[string]int
	done    []bool
}

type reducedTableauProductNode struct {
	base, tableau int
}

func (s *search) reducedLeadsTo(property temporalProperty) temporalProperty {
	c := newLTLCompiler(s.model, property.locals)
	trigger := c.intern(ltlAtom, nil, nil, property.trigger, false)
	consequent := c.intern(ltlAtom, nil, nil, property.predicate, true)
	always := c.intern(ltlAlways, consequent, nil, nil, false)
	and := c.intern(ltlAnd, trigger, always, nil, false)
	property.general = c.intern(ltlEventually, and, nil, nil, false)
	property.eventualities = c.eventuality
	property.mode = generalTemporal
	return property
}

func (s *search) supportsReducedTableau(property temporalProperty) bool {
	return s.model.cfg.Symmetry != "" && s.model.cfg.View == "" &&
		!s.model.environmentIsImpure() && s.stableFingerprintStates() && s.stableRecurrenceSuccessors() &&
		!containsLTLAction(property.general) && s.pureReducedFormula(property.general) && !recurrenceOnly(property.general)
}

func (s *search) checkReducedTableau(property temporalProperty) (bool, error) {
	b := reducedTableauProduct{source: s, graph: search{model: s.model, result: s.result}, indices: map[string]int{}, byBase: make([][]int, len(s.nodes)), done: make([]bool, len(s.nodes))}
	if err := b.tableau.build(s.model, property); err != nil {
		return false, err
	}
	if err := b.build(); err != nil {
		return false, err
	}
	b.debugGraph()
	f := fairSearch{search: &b.graph, property: property, bad: make([]bool, len(b.nodes)), trigger: make([]bool, len(b.nodes)), enabled: make([][]int8, len(s.model.fairness)), acceptance: b.acceptance(property), compactWitnesses: true}
	for i := range f.enabled {
		if !s.model.fairness[i].recurrence {
			f.enabled[i] = make([]int8, len(b.nodes))
		}
	}
	allowed := f.reachableStates()
	found, err := f.findReducedTableauCycle(allowed)
	if found {
		s.result = b.graph.result
		s.reducedTableauTrace = true
	}
	return found, err
}

func (b *reducedTableauProduct) debugGraph() {
	if b.source.model.debug == nil {
		return
	}
	for index, node := range b.nodes {
		edges := make([]debugEdge, 0, len(b.graph.edges[index]))
		for _, edge := range b.graph.edges[index] {
			edges = append(edges, debugEdge{To: edge.to, Action: edge.action})
		}
		b.source.debugEvent(struct {
			Event   string      `json:"event"`
			Node    int         `json:"node"`
			Base    int         `json:"base"`
			Tableau int         `json:"tableau"`
			Edges   []debugEdge `json:"edges"`
		}{"tableau-node", index, node.base, node.tableau, edges})
	}
}

func (f *fairSearch) findReducedTableauCycle(allowed []bool) (bool, error) {
	order := make([][]int, len(f.search.nodes))
	for node, edges := range f.search.edges {
		order[node] = make([]int, len(edges))
		for i := range edges {
			order[node][i] = i
		}
	}
	fallback := false
	found, err := f.search.visitJavaLiveComponents(order, func(root int, component []int) (bool, error) {
		valid, pruned, err := f.checkComponent(component)
		f.debugComponent(component, valid, pruned != nil)
		if err != nil {
			return false, err
		}
		if pruned != nil {
			fallback = true
			return true, nil
		}
		if !valid {
			return false, nil
		}
		for i, node := range component {
			if node == root {
				component[0], component[i] = component[i], component[0]
				break
			}
		}
		if f.supportsJavaTableauCycle() {
			return true, f.recordJavaTableauCycle(root, component, order)
		}
		return true, f.recordCycle(component)
	})
	if fallback {
		return f.findFairCycle(allowed)
	}
	return found, err
}

func (t *reducedTableau) build(m *model, property temporalProperty) error {
	t.indices = map[string]int{}
	expander := ltlExpander{model: m, tableau: true, eventuality: property.eventualities}
	initial, err := expander.alternatives([]*ltlFormula{property.general})
	if err != nil {
		return err
	}
	for i, alternative := range initial {
		t.initial = append(t.initial, t.add(alternative, expander.tableauConditions[i]))
	}
	for i := 0; i < len(t.alternatives); i++ {
		alternatives, err := expander.alternatives(t.alternatives[i].next)
		if err != nil {
			return err
		}
		for index, alternative := range alternatives {
			next := t.add(alternative, expander.tableauConditions[index])
			t.next[i] = append(t.next[i], next)
		}
	}
	return nil
}

func (t *reducedTableau) add(alternative ltlAlternative, conditions []*ltlFormula) int {
	var key strings.Builder
	key.WriteString(ltlProductKey(0, 0, alternative))
	for _, formula := range conditions {
		key.WriteByte(':')
		key.WriteString(strconv.Itoa(formula.id))
	}
	if index, ok := t.indices[key.String()]; ok {
		return index
	}
	index := len(t.alternatives)
	t.indices[key.String()] = index
	t.alternatives = append(t.alternatives, reducedTableauAlternative{ltlAlternative: alternative, conditions: conditions})
	t.next = append(t.next, nil)
	return index
}

func (b *reducedTableauProduct) build() error {
	for _, base := range b.source.initial {
		if err := b.addInitial(base); err != nil {
			return err
		}
	}
	order, ok := b.source.javaLiveOrder()
	if !ok {
		return fmt.Errorf("cannot order reduced tableau successors")
	}
	for base := range b.source.nodes {
		if err := b.expandBase(base, order[base]); err != nil {
			return err
		}
	}
	return nil
}

func (b *reducedTableauProduct) addInitial(base int) error {
	state := b.source.nodes[base].state
	for _, tableau := range b.tableau.initial {
		ok, err := b.consistent(tableau, state)
		if err != nil {
			return err
		}
		if ok {
			index, _ := b.add(base, tableau, state)
			b.graph.initial = append(b.graph.initial, index)
		}
	}
	return nil
}

func (b *reducedTableauProduct) expandBase(base int, order []int) error {
	b.done[base] = true
	// Snapshot the current nodes: newly discovered tableau states for this
	// base are expanded immediately from their concrete incoming state.
	for _, node := range append([]int(nil), b.byBase[base]...) {
		if err := b.expand(node, b.source.nodes[base].state, b.source.edges[base], order); err != nil {
			return err
		}
	}
	return nil
}

func (b *reducedTableauProduct) consistent(tableau int, state evaluator.State) (bool, error) {
	for _, formula := range b.tableau.alternatives[tableau].conditions {
		ok, err := b.source.model.holdsLTLAtom(formula, state, nil, formula.locals)
		if err != nil || ok == formula.negative {
			return false, err
		}
	}
	return true, nil
}

func (b *reducedTableauProduct) add(base, tableau int, state evaluator.State) (int, bool) {
	key := strconv.Itoa(base) + "/" + strconv.Itoa(tableau)
	if index, ok := b.indices[key]; ok {
		return index, false
	}
	index := len(b.nodes)
	b.indices[key] = index
	b.nodes = append(b.nodes, reducedTableauProductNode{base: base, tableau: tableau})
	b.byBase[base] = append(b.byBase[base], index)
	b.graph.nodes = append(b.graph.nodes, stateNode{state: state})
	b.graph.edges = append(b.graph.edges, nil)
	return index, true
}

func (b *reducedTableauProduct) expand(node int, state evaluator.State, edges []graphEdge, order []int) error {
	if err := b.source.model.ctx.Err(); err != nil {
		return err
	}
	b.graph.nodes[node].state = state
	seen := map[int]bool{}
	for _, choice := range order {
		if err := b.expandEdge(node, state, edges[choice], seen); err != nil {
			return err
		}
	}
	return nil
}

func (b *reducedTableauProduct) expandEdge(node int, state evaluator.State, edge graphEdge, seen map[int]bool) error {
	next := b.source.edgeNext(edge)
	if edge.action == "Stuttering" && edge.next == nil {
		next = state
	}
	for _, tableau := range b.tableau.next[b.nodes[node].tableau] {
		ok, err := b.consistent(tableau, next)
		if err != nil {
			return err
		}
		if ok {
			if err := b.link(node, edge, tableau, next, seen); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *reducedTableauProduct) link(node int, edge graphEdge, tableau int, next evaluator.State, seen map[int]bool) error {
	target, fresh := b.add(edge.to, tableau, next)
	if seen[target] {
		return nil
	}
	seen[target] = true
	b.graph.edges[node] = append(b.graph.edges[node], graphEdge{to: target, action: edge.action, source: edge.source, next: next})
	if fresh && b.done[edge.to] {
		if err := b.expandRegenerated(target, next); err != nil {
			return err
		}
	}
	return nil
}

func (b *reducedTableauProduct) expandRegenerated(node int, state evaluator.State) error {
	if b.source.model.debug != nil {
		b.source.debugEvent(struct {
			Event string `json:"event"`
			Node  int    `json:"node"`
		}{"tableau-regenerated", node})
	}
	base := b.nodes[node].base
	// TLC explores stuttering first when it regenerates an already-done node.
	edges := []graphEdge{{to: base, action: "Stuttering", next: state}}
	expr, err := b.source.model.definition(b.source.model.cfg.Next)
	if err != nil {
		return err
	}
	gen := generator{model: b.source.model, current: state, emit: func(next evaluator.State, action string, source *parser.Expression) (bool, error) {
		ok, err := b.source.constraints(b.source.model.cfg.Constraints, next, nil)
		if err != nil || !ok {
			return err == nil, err
		}
		ok, err = b.source.constraints(b.source.model.cfg.ActionConstraints, state, next)
		if err != nil || !ok {
			return err == nil, err
		}
		key, err := b.source.model.stateKey(next)
		if err != nil {
			return false, err
		}
		if target, ok := b.source.indices[key]; ok {
			edges = append(edges, graphEdge{to: target, action: action, source: source, next: next})
		}
		return true, nil
	}}
	if _, err := gen.solve(&workList{item: work{expr: expr, label: true}}, b.source.model.partialState(), b.source.model.cfg.Next); err != nil {
		return err
	}
	order := make([]int, len(edges))
	for i := range order {
		order[i] = i
	}
	return b.expand(node, state, edges, order)
}

func (b *reducedTableauProduct) acceptance(property temporalProperty) [][]bool {
	result := make([][]bool, len(property.eventualities))
	for i := range result {
		result[i] = make([]bool, len(b.nodes))
		for j, node := range b.nodes {
			result[i][j] = b.tableau.alternatives[node.tableau].accepting[i]
		}
	}
	return result
}
