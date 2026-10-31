package evaluator

import (
	"fmt"
	"maps"
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (e *evaluator) setComprehension(expr *parser.Expression) (value.Value, error) {
	if expr.Kind == "SetFilter" {
		return e.filter(expr)
	}
	return e.comprehension(expr)
}

func (e *evaluator) filter(expr *parser.Expression) (value.Value, error) {
	if len(expr.Bounds) != 1 || expr.Bounds[0].Set == nil {
		return value.Value{}, fmt.Errorf("invalid set filter bound")
	}
	bound := expr.Bounds[0]
	base, err := e.eval(bound.Set)
	if err != nil {
		return value.Value{}, err
	}
	if !base.IsSet() {
		return value.Value{}, fmt.Errorf("set filter requires a set")
	}
	if base.Kind() == value.SetKind || base.Kind() == value.IntervalKind {
		return e.eagerFilter(base, bound, expr.Args[0])
	}
	captured := e.snapshot()
	return value.Filter(base, func(item value.Value) (bool, error) {
		instance := captured
		child, err := instance.bind(bound, item)
		if err != nil {
			return false, err
		}
		return child.condition(expr.Args[0])
	})
}

func (e *evaluator) eagerFilter(base value.Value, bound parser.Bound, predicate *parser.Expression) (value.Value, error) {
	var elements []value.Value
	err := base.Each(func(item value.Value) (bool, error) {
		frame, err := boundScope(e.scope, bound, item)
		if err != nil {
			return false, err
		}
		saved := *e
		e.scope = frame
		include, err := e.condition(predicate)
		*e = saved
		if err != nil {
			return false, err
		}
		if include {
			elements = append(elements, item)
		}
		return true, nil
	})
	if err != nil {
		return value.Value{}, err
	}
	return e.orderedSet(elements)
}

// A deferred predicate must not observe later state changes or share the
// recursion map between concurrent calls.
func (e *evaluator) snapshot() evaluator {
	result := *e
	result.active = nil
	result.functionCache = nil
	result.scope = snapshotScope(e.scope)
	result.context.Current = maps.Clone(e.context.Current)
	result.context.Next = maps.Clone(e.context.Next)
	result.context.Constants = maps.Clone(e.context.Constants)
	result.context.Locals = maps.Clone(e.context.Locals)
	return result
}

func snapshotScope(s *scope) *scope {
	return snapshotScopeGraph(s, make(map[*scope]*scope))
}

func snapshotScopeGraph(s *scope, seen map[*scope]*scope) *scope {
	if s == nil {
		return nil
	}
	if clone, ok := seen[s]; ok {
		return clone
	}
	clone := &scope{values: maps.Clone(s.values), definitions: s.definitions}
	seen[s] = clone
	clone.parent = snapshotScopeGraph(s.parent, seen)
	clone.bound = snapshotBindings(s.bound, seen)
	if s.operators != nil {
		clone.operators = make(map[string]operatorRef, len(s.operators))
		for name, op := range s.operators {
			op.scope = snapshotScopeGraph(op.scope, seen)
			clone.operators[name] = op
		}
	}
	return clone
}

// A snapshot gives each lazy argument its own empty cache.
func snapshotBindings(bindings []binding, seen map[*scope]*scope) []binding {
	if bindings == nil {
		return nil
	}
	result := slices.Clone(bindings)
	for i, bound := range result {
		if bound.arg != nil {
			result[i].arg = &primeArgument{expression: bound.arg.expression, scope: snapshotScopeGraph(bound.arg.scope, seen)}
		}
	}
	return result
}
