package checker

import (
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

var errSequenceNotRecognized = errors.New("not an ordered eventuality")

// prepareOrderedSequence expands finite recursive temporal definitions into
// state predicates that must occur in order on the same behavior.
func (m *model) prepareOrderedSequence(name string, expr *parser.Expression, bounds []parser.Bound) (bool, error) {
	if !m.sequenceCandidate(expr) {
		return false, nil
	}
	if err := m.validateSequenceBounds(bounds); err != nil {
		return true, err
	}
	var properties []temporalProperty
	err := m.expandBindings(bounds, nil, func(locals map[string]value.Value) error {
		p, ok := m.sequenceProperty(name, expr, locals)
		if !ok {
			return errSequenceNotRecognized
		}
		properties = append(properties, p)
		return nil
	})
	if errors.Is(err, errSequenceNotRecognized) || (err == nil && len(properties) == 0) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	m.properties = append(m.properties, properties...)
	m.livenessOrder = append(m.livenessOrder, properties...)
	return true, nil
}

func (m *model) sequenceCandidate(expr *parser.Expression) bool {
	switch expr.Kind {
	case "Eventually", "Identifier", "Apply", "If":
		return m.hasTemporal(expr, map[string]bool{})
	default:
		return false
	}
}

func (m *model) validateSequenceBounds(bounds []parser.Bound) error {
	for _, bound := range bounds {
		if bound.Set == nil || m.hasAction(bound.Set, map[string]bool{}) || m.readsState(bound.Set, map[string]bool{}) {
			return fmt.Errorf("temporal quantifier domain must be a constant set")
		}
	}
	return nil
}

func (m *model) sequenceProperty(name string, expr *parser.Expression, locals map[string]value.Value) (temporalProperty, bool) {
	sequence, ok := m.compileSequence(expr, locals, map[string]bool{}, 0)
	if !ok || len(sequence) == 0 {
		return temporalProperty{}, false
	}
	p := temporalProperty{name: name, source: expr, locals: maps.Clone(locals), sequence: sequence, predicate: sequence[0], mode: eventually}
	if len(sequence) > 1 {
		p.trigger, p.predicate, p.mode = sequence[0], sequence[1], orderedEventually
	}
	return p, true
}

func (m *model) compileSequence(expr *parser.Expression, locals map[string]value.Value, active map[string]bool, depth int) ([]*parser.Expression, bool) {
	if depth >= 256 {
		return nil, false
	}
	switch expr.Kind {
	case "Eventually":
		return m.compileSequenceEventually(expr, locals, active, depth)
	case "Identifier", "Apply":
		return m.compileSequenceCall(expr, locals, active, depth)
	case "If":
		return m.compileSequenceIf(expr, locals, active, depth)
	default:
		return nil, false
	}
}

func (m *model) compileSequenceEventually(expr *parser.Expression, locals map[string]value.Value, active map[string]bool, depth int) ([]*parser.Expression, bool) {
	if len(expr.Args) != 1 {
		return nil, false
	}
	inner := expr.Args[0]
	if m.isStateExpression(inner) {
		return []*parser.Expression{inner}, true
	}
	if inner.Kind != "And" {
		return nil, false
	}
	states, tail, ok := m.sequenceParts(inner.Args)
	if !ok {
		return nil, false
	}
	suffix, ok := m.compileSequence(tail, locals, active, depth+1)
	if !ok {
		return nil, false
	}
	first := states[0]
	if len(states) > 1 {
		first = &parser.Expression{Kind: "And", Args: states}
	}
	return append([]*parser.Expression{first}, suffix...), true
}

func (m *model) sequenceParts(parts []*parser.Expression) ([]*parser.Expression, *parser.Expression, bool) {
	var states []*parser.Expression
	var tail *parser.Expression
	for _, part := range parts {
		if m.isStateExpression(part) {
			states = append(states, part)
		} else if tail == nil {
			tail = part
		} else {
			return nil, nil, false
		}
	}
	return states, tail, len(states) > 0 && tail != nil
}

func (m *model) compileSequenceCall(expr *parser.Expression, locals map[string]value.Value, active map[string]bool, depth int) ([]*parser.Expression, bool) {
	definition, ok := m.defs[expr.Text]
	if !ok || definition.Kind != "Operator" || len(definition.Params) != len(expr.Args) {
		return nil, false
	}
	key := m.sequenceCallKey(expr, locals)
	if active[key] {
		return nil, false
	}
	expanded, err := substituteActionCall(definition, expr)
	if err != nil {
		return nil, false
	}
	active[key] = true
	defer delete(active, key)
	return m.compileSequence(expanded, locals, active, depth+1)
}

func (m *model) sequenceCallKey(expr *parser.Expression, locals map[string]value.Value) string {
	var key strings.Builder
	key.WriteString(strconv.Itoa(len(expr.Text)) + ":" + expr.Text)
	for _, arg := range expr.Args {
		if m.readsState(arg, map[string]bool{}) || m.hasAction(arg, map[string]bool{}) || m.hasTemporal(arg, map[string]bool{}) {
			return strconv.Itoa(len(expr.Text)) + ":" + expr.Text
		}
		v, err := m.program.Evaluate(arg, evaluator.Context{Constants: m.constants, Locals: locals})
		if err != nil {
			return strconv.Itoa(len(expr.Text)) + ":" + expr.Text
		}
		part, err := v.Key()
		if err != nil {
			return strconv.Itoa(len(expr.Text)) + ":" + expr.Text
		}
		key.WriteString(strconv.Itoa(len(part)) + ":" + part)
	}
	return key.String()
}

func (m *model) compileSequenceIf(expr *parser.Expression, locals map[string]value.Value, active map[string]bool, depth int) ([]*parser.Expression, bool) {
	if len(expr.Args) != 3 || m.readsState(expr.Args[0], map[string]bool{}) || m.hasAction(expr.Args[0], map[string]bool{}) || m.hasTemporal(expr.Args[0], map[string]bool{}) {
		return nil, false
	}
	v, err := m.program.Evaluate(expr.Args[0], evaluator.Context{Constants: m.constants, Locals: locals})
	if err != nil {
		return nil, false
	}
	condition, ok := v.Bool()
	if !ok {
		return nil, false
	}
	branch := 2
	if condition {
		branch = 1
	}
	return m.compileSequence(expr.Args[branch], locals, active, depth+1)
}
