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
	if item.label && !g.initial {
		if g.actionExistentialCanSplit(item.expr) {
			g.source = item.expr.Args[0]
		} else {
			item.label = false
		}
	}
	more, err := g.witnesses(item, rest, partial, action)
	g.source = previous
	if err == nil && !more {
		g.addInitialFailureSource(item.expr, false)
	}
	return more, err
}

// witnesses evaluates the bounds of an existential before binding them, as
// TLC's ContextEnumerator does.
func (g *generator) witnesses(item work, rest []work, partial evaluator.State, action string) (bool, error) {
	domains, err := g.boundDomains(item.expr.Bounds, partial, item.locals)
	if err != nil {
		return false, err
	}
	more, err := g.enumerateWitnesses(item, domains, rest, partial, item.locals, action)
	if err != nil && item.label && g.constantExistentialBound(item.expr) {
		g.incompleteAction(err, action, item.expr.Args[0].Location.Range, simpleDomainMultiple(domains[0]))
	}
	return more, err
}

func (g *generator) boundDomains(bounds []parser.Bound, partial evaluator.State, locals map[string]value.Value) ([]value.Value, error) {
	current, next := g.context(partial)
	domains := make([]value.Value, len(bounds))
	for i, bound := range bounds {
		if bound.Set == nil {
			return nil, fmt.Errorf("unbounded existential action is unsupported")
		}
		domain, err := g.evalWithLocals(bound.Set, current, next, locals)
		if err != nil {
			return nil, err
		}
		domains[i] = domain
	}
	return domains, nil
}

// enumerateWitnesses binds the bounds in ContextEnumerator order, in which the
// first bound varies fastest.
func (g *generator) enumerateWitnesses(item work, domains []value.Value, rest []work, partial evaluator.State, locals map[string]value.Value, action string) (bool, error) {
	last := len(domains) - 1
	if last < 0 {
		return g.witness(item, rest, partial, locals, action)
	}
	more := true
	err := domains[last].Each(func(v value.Value) (bool, error) {
		bindings, err := bindBound(item.expr.Bounds[last], v, locals, "tuple binding has wrong arity")
		if err != nil {
			return false, err
		}
		more, err = g.enumerateWitnesses(item, domains[:last], rest, partial, bindings, action)
		return more, err
	})
	return more, err
}

// witness solves the body for one binding, which TLC splits into its own
// action when the existential is labeled.
func (g *generator) witness(item work, rest []work, partial evaluator.State, locals map[string]value.Value, action string) (bool, error) {
	if more, err := g.startAction(item); err != nil || !more {
		return more, err
	}
	return g.solve(prepend(work{expr: item.expr.Args[0], path: item.path, label: item.label, locals: locals}, rest), partial, action)
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
