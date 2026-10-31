package checker

import (
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// LiveCheck1 expands a tableau together with the sampled trace. Only newly
// discovered product nodes become the frontier for the next sampled state.
type simulationLeadsBuilder struct {
	graph         search
	property      temporalProperty
	nodes         []simulationLeadsNode
	indices       map[simulationLeadsKey]int
	ignoreTrigger bool
	ignoreTarget  bool
}

// The negation of P ~> Q is <>(P /\ []~Q). The start phase
// witnesses P, waiting postpones it, and pending maintains ~Q.
type simulationLeadsPhase uint8

const (
	leadsStart simulationLeadsPhase = iota
	leadsWaiting
	leadsPending
)

type (
	simulationLeadsNode struct{ phase simulationLeadsPhase }
	simulationLeadsKey  struct {
		state string
		phase simulationLeadsPhase
	}
)

func (r *simulationRun) checkSimulationLeadsTo(visited int, property temporalProperty) (*Result, error) {
	constants, err := r.simulationLeadsConstants(property)
	if err != nil {
		return nil, err
	}
	if constants.triggerKnown && constants.trigger {
		return r.checkSimulationLeadsRecurrence(visited, property)
	}
	b := simulationLeadsBuilder{graph: newSearch(r.model), property: property, indices: map[simulationLeadsKey]int{}}
	b.ignoreTrigger = constants.triggerKnown && !constants.trigger
	b.ignoreTarget = b.ignoreTrigger || constants.targetKnown
	b.graph.result = r.search.result
	return b.check(r.search.nodes[:visited], b.ignoreTrigger || constants.targetKnown && constants.target)
}

type simulationLeadsConstants struct {
	triggerKnown, trigger bool
	targetKnown, target   bool
}

func (r *simulationRun) simulationLeadsConstants(property temporalProperty) (simulationLeadsConstants, error) {
	var result simulationLeadsConstants
	var err error
	result.triggerKnown, result.trigger, err = r.simulationLeadsConstant(property.trigger, property)
	if err != nil {
		return result, err
	}
	result.targetKnown, result.target, err = r.simulationLeadsConstant(property.predicate, property)
	return result, err
}

func (b *simulationLeadsBuilder) check(trace []stateNode, ignoreSemanticDifference bool) (*Result, error) {
	if err := b.build(trace); err != nil {
		return nil, err
	}
	f := fairSearch{search: &b.graph, property: temporalProperty{name: b.property.name, mode: generalTemporal}, acceptance: [][]bool{b.acceptance()}}
	found, err := f.findFairCycle(f.reachableStates())
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	if err := b.collapseTrace(); err != nil {
		return nil, err
	}
	if ignoreSemanticDifference {
		// LiveCheck1 ignores constant predicates in tableau consistency checks.
		b.graph.result.Violation.Properties = []string{b.property.name}
		return &b.graph.result, nil
	}
	result, err := b.graph.reportLassoProperties()
	return &result, err
}

func (r *simulationRun) simulationLeadsConstant(expr *parser.Expression, property temporalProperty) (bool, bool, error) {
	if r.model.readsState(expr, map[string]bool{}) {
		return false, false, nil
	}
	value, err := r.model.holdsProperty(expr, nil, property.locals)
	return true, value, err
}

func (r *simulationRun) checkSimulationLeadsRecurrence(visited int, property temporalProperty) (*Result, error) {
	graph, err := r.traceGraph(visited)
	if err != nil {
		return nil, err
	}
	property.mode = often
	found, err := graph.checkSimulationOften(property)
	if err != nil || !found {
		return nil, err
	}
	result, err := graph.reportLassoProperties()
	return &result, err
}

func (b *simulationLeadsBuilder) collapseTrace() error {
	v := b.graph.result.Violation
	trace := make([]TraceStep, 0, len(v.Trace))
	previous := ""
	loop := 0
	for i, step := range v.Trace {
		key, err := b.graph.model.unreducedStateKey(step.State)
		if err != nil {
			return err
		}
		if i == 0 || key != previous {
			trace = append(trace, step)
		}
		if i <= *v.LoopStart {
			loop = len(trace) - 1
		}
		previous = key
	}
	v.Trace, v.LoopStart = trace, &loop
	return nil
}

func (b *simulationLeadsBuilder) build(trace []stateNode) error {
	frontier, err := b.initialFrontier(trace[0])
	if err != nil {
		return err
	}
	for _, state := range trace[1:] {
		frontier, err = b.advanceFrontier(frontier, state)
		if err != nil || len(frontier) == 0 {
			return err
		}
	}
	return nil
}

func (b *simulationLeadsBuilder) initialFrontier(state stateNode) ([]int, error) {
	initial, err := b.values(state.state)
	if err != nil {
		return nil, err
	}
	frontier := make([]int, 0, 2)
	for _, phase := range []simulationLeadsPhase{leadsStart, leadsWaiting} {
		index, created, err := b.add(state, initial, phase)
		if err != nil {
			return nil, err
		}
		if created {
			frontier = append(frontier, index)
		}
	}
	b.graph.initial = append(b.graph.initial, frontier...)
	b.linkInitial(frontier, initial.key)
	return frontier, nil
}

func (b *simulationLeadsBuilder) linkInitial(frontier []int, key string) {
	for _, src := range frontier {
		for _, phase := range simulationLeadsNext(b.nodes[src].phase) {
			if dest, ok := b.indices[simulationLeadsKey{state: key, phase: phase}]; ok {
				b.link(src, dest, graphEdge{action: "Stuttering"})
			}
		}
	}
}

func (b *simulationLeadsBuilder) advanceFrontier(frontier []int, state stateNode) ([]int, error) {
	values, err := b.values(state.state)
	if err != nil {
		return nil, err
	}
	var next []int
	for _, src := range frontier {
		if err := b.expand(src, state, values, &next, false); err != nil {
			return nil, err
		}
	}
	for i := 0; i < len(next); i++ {
		if err := b.expand(next[i], state, values, &next, true); err != nil {
			return nil, err
		}
	}
	return next, nil
}

type simulationLeadsValues struct {
	state                     evaluator.State
	key                       string
	trigger, target           bool
	triggerKnown, targetKnown bool
}

func simulationLeadsNext(phase simulationLeadsPhase) []simulationLeadsPhase {
	if phase == leadsWaiting {
		return []simulationLeadsPhase{leadsStart, leadsWaiting}
	}
	return []simulationLeadsPhase{leadsPending}
}

func (b *simulationLeadsBuilder) values(state evaluator.State) (*simulationLeadsValues, error) {
	key, err := b.graph.model.unreducedStateKey(state)
	return &simulationLeadsValues{state: state, key: key}, err
}

func (b *simulationLeadsBuilder) allowed(v *simulationLeadsValues, phase simulationLeadsPhase) (bool, error) {
	if phase == leadsWaiting {
		return true, nil
	}
	if phase == leadsStart && !b.ignoreTrigger {
		trigger, err := b.trigger(v)
		if err != nil {
			return false, err
		}
		if !trigger {
			return false, nil
		}
	}
	if b.ignoreTarget {
		return true, nil
	}
	return b.notTarget(v)
}

func (b *simulationLeadsBuilder) trigger(v *simulationLeadsValues) (bool, error) {
	if !v.triggerKnown {
		var err error
		v.trigger, err = b.graph.model.holdsProperty(b.property.trigger, v.state, b.property.locals)
		if err != nil {
			return false, err
		}
		v.triggerKnown = true
	}
	return v.trigger, nil
}

func (b *simulationLeadsBuilder) notTarget(v *simulationLeadsValues) (bool, error) {
	if !v.targetKnown {
		var err error
		v.target, err = b.graph.model.holdsProperty(b.property.predicate, v.state, b.property.locals)
		if err != nil {
			return false, err
		}
		v.targetKnown = true
	}
	return !v.target, nil
}

func (b *simulationLeadsBuilder) add(state stateNode, values *simulationLeadsValues, phase simulationLeadsPhase) (int, bool, error) {
	k := simulationLeadsKey{state: values.key, phase: phase}
	if index, ok := b.indices[k]; ok {
		return index, false, nil
	}
	ok, err := b.allowed(values, phase)
	if err != nil || !ok {
		return -1, false, err
	}
	index := len(b.nodes)
	b.indices[k] = index
	b.nodes = append(b.nodes, simulationLeadsNode{phase: phase})
	b.graph.nodes = append(b.graph.nodes, state)
	b.graph.edges = append(b.graph.edges, nil)
	return index, true, nil
}

func (b *simulationLeadsBuilder) expand(src int, state stateNode, values *simulationLeadsValues, frontier *[]int, stutter bool) error {
	for _, phase := range simulationLeadsNext(b.nodes[src].phase) {
		dest, created, err := b.add(state, values, phase)
		if err != nil {
			return err
		}
		if dest < 0 {
			continue
		}
		edge := graphEdge{action: state.action, source: state.source}
		if stutter {
			edge = graphEdge{action: "Stuttering"}
		}
		b.link(src, dest, edge)
		if created {
			*frontier = append(*frontier, dest)
		}
	}
	return nil
}

func (b *simulationLeadsBuilder) link(src, dest int, edge graphEdge) {
	if simulationEdgeExists(b.graph.edges[src], dest) {
		return
	}
	edge.to = dest
	b.graph.edges[src] = append(b.graph.edges[src], edge)
}

func (b *simulationLeadsBuilder) acceptance() []bool {
	result := make([]bool, len(b.nodes))
	for i, node := range b.nodes {
		result[i] = node.phase != leadsWaiting
	}
	return result
}
