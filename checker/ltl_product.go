package checker

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

type ltlProductNode struct {
	base      int
	choice    int
	next      []*ltlFormula
	accepting []bool
}

type ltlProductBuilder struct {
	source               *search
	eventuality          []*ltlFormula
	product              search
	nodes                []ltlProductNode
	indices              map[string]int
	representativeStates bool
}

func (s *search) checkGeneralProperty(property temporalProperty) (bool, error) {
	if len(s.model.fairness) > 0 && s.supportsReducedTableau(property) {
		return s.checkReducedTableau(property)
	}
	source, canonical, err := s.generalPropertyGraph(property)
	if err != nil {
		return false, err
	}
	b := ltlProductBuilder{source: source, eventuality: property.eventualities, product: search{model: s.model, result: s.result, fairChecks: s.fairChecks}, indices: map[string]int{}, representativeStates: canonical}
	if err := b.build(property.general); err != nil {
		return false, err
	}
	f := fairSearch{search: &b.product, property: property, bad: make([]bool, len(b.nodes)), trigger: make([]bool, len(b.nodes)), enabled: make([][]int8, len(s.model.fairness)), acceptance: b.acceptance()}
	for i := range f.enabled {
		if !s.model.fairness[i].recurrence {
			f.enabled[i] = make([]int8, len(b.nodes))
		}
	}
	allowed := f.reachableStates()
	found, err := f.findFairCycle(allowed)
	if found {
		s.result = b.product.result
	}
	return found, err
}

func (s *search) validateGeneralReduction(property temporalProperty) error {
	if !s.model.hasReduction() || s.constantPropertyHolds(property) || s.supportsReducedStateFormula(property) {
		return nil
	}
	for _, edges := range s.edges {
		if err := s.validateGeneralEdges(edges); err != nil {
			return err
		}
	}
	return nil
}

// With no fairness or effects, the tableau can follow state predicates through
// merged nodes. Action checks need TLC's per-tableau successor deduplication.
func (s *search) supportsReducedStateFormula(property temporalProperty) bool {
	if len(s.model.fairness) != 0 || s.model.environmentIsImpure() || containsLTLAction(property.general) {
		return false
	}
	return s.pureReducedFormula(property.general)
}

func (s *search) pureReducedFormula(formula *ltlFormula) bool {
	if formula == nil {
		return true
	}
	if formula.atom != nil && s.model.unsafeIncrementalExpression(formula.atom) {
		return false
	}
	return s.pureReducedFormula(formula.left) && s.pureReducedFormula(formula.right)
}

// These formulas describe the same behavior on every infinite path. Restrict
// this optimization to satisfied properties: upstream can fail while recovering
// a counterexample when a view merges distinct states.
func (s *search) constantPropertyHolds(property temporalProperty) bool {
	if len(s.model.fairness) != 0 || !s.constantTemporalFormula(property.general) {
		return false
	}
	value, err := s.constantTemporalValue(property.general)
	return err == nil && !value // general is the negation of the property
}

func (s *search) constantTemporalFormula(formula *ltlFormula) bool {
	if formula == nil {
		return true
	}
	if formula.kind == ltlAction {
		return false
	}
	if formula.kind == ltlAtom {
		return s.pureConstantAtom(formula.atom, formula.locals)
	}
	return s.constantTemporalFormula(formula.left) && s.constantTemporalFormula(formula.right)
}

// Only evaluate expressions known to be free of effects and random choices.
// More constant-level operators can be added once their behavior is checked.
func (s *search) pureConstantAtom(expr *parser.Expression, locals map[string]value.Value) bool {
	switch expr.Kind {
	case "Number", "String":
		return true
	case "Identifier":
		return s.pureConstantIdentifier(expr.Text, locals)
	case "Apply":
		if !slices.Contains([]string{"=", "#", "/=", "<", "<=", ">", ">=", "+", "-", "*", "~"}, expr.Text) {
			return false
		}
		return s.pureConstantArgs(expr.Args, locals)
	case "And", "Or":
		return s.pureConstantArgs(expr.Args, locals)
	default:
		return false
	}
}

func (s *search) pureConstantIdentifier(name string, locals map[string]value.Value) bool {
	_, local := locals[name]
	_, constant := s.model.constants[name]
	return local || constant || name == "TRUE" || name == "FALSE"
}

func (s *search) pureConstantArgs(args []*parser.Expression, locals map[string]value.Value) bool {
	for _, arg := range args {
		if !s.pureConstantAtom(arg, locals) {
			return false
		}
	}
	return true
}

func (s *search) constantTemporalValue(formula *ltlFormula) (bool, error) {
	switch formula.kind {
	case ltlTrue:
		return true, nil
	case ltlFalse:
		return false, nil
	case ltlAtom:
		result, err := s.model.holdsLTLAtom(formula, nil, nil, formula.locals)
		if formula.negative {
			result = !result
		}
		return result, err
	case ltlAlways, ltlEventually:
		return s.constantTemporalValue(formula.left)
	case ltlAnd, ltlOr:
		return s.constantTemporalJunction(formula)
	default:
		return false, fmt.Errorf("unsupported constant temporal formula")
	}
}

func (s *search) constantTemporalJunction(formula *ltlFormula) (bool, error) {
	left, err := s.constantTemporalValue(formula.left)
	if err != nil {
		return false, err
	}
	right, err := s.constantTemporalValue(formula.right)
	if err != nil {
		return false, err
	}
	if formula.kind == ltlAnd {
		return left && right, nil
	}
	return left || right, nil
}

func (s *search) validateGeneralEdges(edges []graphEdge) error {
	for _, edge := range edges {
		if edge.next == nil {
			continue
		}
		equal, err := s.sameConcreteState(edge.next, s.nodes[edge.to].state)
		if err != nil {
			return err
		}
		if !equal {
			return fmt.Errorf("general temporal formulas over reduced states with distinct concrete successors are not yet supported")
		}
	}
	return nil
}

func (s *search) sameConcreteState(a, b evaluator.State) (bool, error) {
	for _, name := range s.model.names {
		equal, err := value.Equal(a[name], b[name])
		if err != nil || !equal {
			return false, err
		}
	}
	return true, nil
}

func (b *ltlProductBuilder) build(formula *ltlFormula) error {
	for _, initial := range b.source.initial {
		indices, err := b.addAlternatives(initial, b.source.nodes[initial].state, []*ltlFormula{formula})
		if err != nil {
			return err
		}
		b.product.initial = append(b.product.initial, indices...)
	}
	for index := 0; index < len(b.nodes); index++ {
		if err := b.expand(index); err != nil {
			return err
		}
	}
	return nil
}

func (b *ltlProductBuilder) expand(index int) error {
	node := b.nodes[index]
	edge := b.source.edges[node.base][node.choice]
	current := b.source.edgeNext(edge)
	if b.representativeStates {
		current = b.source.nodes[edge.to].state
	}
	indices, err := b.addAlternatives(edge.to, current, node.next)
	if err != nil {
		return err
	}
	for _, target := range indices {
		choice := len(b.product.edges[index])
		b.product.edges[index] = append(b.product.edges[index], graphEdge{to: target, action: edge.action, source: edge.source, next: edge.next})
		copyFairActionValues(b.source, &b.product, index, choice, node.base, node.choice)
	}
	return nil
}

func (b *ltlProductBuilder) addAlternatives(base int, current evaluator.State, formulas []*ltlFormula) ([]int, error) {
	var indices []int
	for choice, edge := range b.source.edges[base] {
		next := b.source.edgeNext(edge)
		if edge.action == "Stuttering" && edge.to == base && edge.next == nil {
			next = current
		}
		expander := ltlExpander{model: b.source.model, state: current, nextState: next, eventuality: b.eventuality, values: map[int]bool{}, known: map[int]bool{}, cache: b.source.ltlValues, base: base, choice: choice}
		alternatives, err := expander.alternatives(formulas)
		if err != nil {
			return nil, err
		}
		for _, alternative := range alternatives {
			indices = append(indices, b.add(base, choice, alternative))
		}
	}
	return indices, nil
}

func (b *ltlProductBuilder) add(base, choice int, alternative ltlAlternative) int {
	key := ltlProductKey(base, choice, alternative)
	if index, ok := b.indices[key]; ok {
		return index
	}
	index := len(b.nodes)
	b.indices[key] = index
	b.nodes = append(b.nodes, ltlProductNode{base: base, choice: choice, next: alternative.next, accepting: alternative.accepting})
	b.product.nodes = append(b.product.nodes, stateNode{state: b.source.nodes[base].state})
	b.product.edges = append(b.product.edges, nil)
	copyFairStateValues(b.source, &b.product, index, base)
	return index
}

func ltlProductKey(base, choice int, alternative ltlAlternative) string {
	var key strings.Builder
	key.WriteString(strconv.Itoa(base))
	key.WriteByte('/')
	key.WriteString(strconv.Itoa(choice))
	for _, formula := range alternative.next {
		key.WriteByte(':')
		key.WriteString(strconv.Itoa(formula.id))
	}
	key.WriteByte('|')
	for _, accept := range alternative.accepting {
		if accept {
			key.WriteByte('1')
		} else {
			key.WriteByte('0')
		}
	}
	return key.String()
}

func (b *ltlProductBuilder) acceptance() [][]bool {
	result := make([][]bool, len(b.eventuality))
	for i := range result {
		result[i] = make([]bool, len(b.nodes))
		for j, node := range b.nodes {
			result[i][j] = node.accepting[i]
		}
	}
	return result
}
