package checker

import (
	"fmt"
	"maps"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (g *generator) existential(item work, rest []work, partial evaluator.State, action string) (bool, error) {
	previous := g.source
	split := false
	if item.label && !g.initial {
		split = g.actionExistentialCanSplit(item.expr)
		if split {
			g.source = item.expr.Args[0]
		} else {
			item.label = false
		}
	}
	more, err := g.witnesses(item, item.expr.Bounds, rest, partial, item.locals, action)
	g.source = previous
	if err != nil && !split {
		// Java TLC splits an action at a top-level \E into one action per
		// witness, and keeps any other \E on its call stack.
		addConjunctErrorSource(err, item.expr)
	}
	if err == nil && !more {
		g.addInitialFailureSource(item.expr, false)
	}
	return more, err
}

func (g *generator) witnesses(item work, bounds []parser.Bound, rest []work, partial evaluator.State, locals map[string]value.Value, action string) (bool, error) {
	if len(bounds) == 0 {
		return g.solve(prepend(work{expr: item.expr.Args[0], path: item.path, label: item.label, locals: locals}, rest), partial, action)
	}
	bound := bounds[0]
	if bound.Set == nil {
		return false, fmt.Errorf("unbounded existential action is unsupported")
	}
	current, next := g.context(partial)
	domain, err := g.evalWithLocals(bound.Set, current, next, locals)
	if err != nil {
		return false, err
	}
	more := true
	err = domain.Each(func(v value.Value) (bool, error) {
		bindings, err := bindBound(bound, v, locals, "tuple binding has wrong arity")
		if err != nil {
			return false, err
		}
		more, err = g.witnesses(item, bounds[1:], rest, partial, bindings, action)
		return more, err
	})
	if err != nil && item.label && g.constantExistentialBound(item.expr) {
		g.incompleteAction(err, action, item.expr.Args[0].Location.Range, simpleDomainMultiple(domain))
	}
	return more, err
}

func (g *generator) constantExistentialBound(expr *parser.Expression) bool {
	if len(expr.Bounds) != 1 || expr.Bounds[0].Set == nil {
		return false
	}
	return g.actionExistentialCanSplit(expr)
}

func (g *generator) actionExistentialCanSplit(expr *parser.Expression) bool {
	if len(expr.Bounds) == 0 {
		return false
	}
	for _, bound := range expr.Bounds {
		if bound.Set == nil || g.model.readsState(bound.Set, map[string]bool{}) || g.model.hasAction(bound.Set, map[string]bool{}) || g.model.unsafeIncrementalExpression(bound.Set) {
			return false
		}
	}
	return true
}

func simpleDomainMultiple(domain value.Value) bool {
	if domain.Kind() != value.SetKind && domain.Kind() != value.IntervalKind {
		return false
	}
	size, err := domain.Cardinality()
	return err == nil && size > 1
}

func bindBound(bound parser.Bound, v value.Value, locals map[string]value.Value, arityError string) (map[string]value.Value, error) {
	values := []value.Value{v}
	if bound.Tuple {
		var err error
		values, err = v.TupleElements()
		if err != nil {
			return nil, err
		}
	}
	if len(values) != len(bound.Names) {
		return nil, fmt.Errorf("%s", arityError)
	}
	bindings := maps.Clone(locals)
	if bindings == nil {
		bindings = map[string]value.Value{}
	}
	for i, name := range bound.Names {
		bindings[name] = values[i]
	}
	return bindings, nil
}
