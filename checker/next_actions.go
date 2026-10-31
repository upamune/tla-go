package checker

import (
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// noNextActions reports a next-state relation that TLC's Tool.getActions
// splits into no actions: each disjunct is an existential with constant
// bounds, one of which is empty. Other forms are assumed to have actions.
func (m *model) noNextActions(expr *parser.Expression) bool {
	return m.withoutActions(expr, map[string]bool{})
}

func (m *model) withoutActions(expr *parser.Expression, seen map[string]bool) bool {
	switch expr.Kind {
	case "Or":
		for _, arg := range expr.Args {
			if !m.withoutActions(arg, seen) {
				return false
			}
		}
		return true
	case "Label":
		return m.withoutActions(expr.Args[0], seen)
	case "Exists":
		return m.emptyActionExistential(expr)
	case "Identifier":
		return m.definitionWithoutActions(expr.Text, seen)
	}
	return false
}

func (m *model) definitionWithoutActions(name string, seen map[string]bool) bool {
	d, ok := m.defs[name]
	if !ok || seen[name] || d.Kind != "Operator" || len(d.Params) > 0 || d.InstanceSubstituted {
		return false
	}
	seen[name] = true
	return m.withoutActions(d.Body, seen)
}

// emptyActionExistential follows TLC's ContextEnumerator, which yields no
// binding when a bound set is empty. TLC keeps an existential whose bounds
// fail to evaluate as one action.
func (m *model) emptyActionExistential(expr *parser.Expression) bool {
	g := generator{model: m}
	if !g.actionExistentialCanSplit(expr) {
		return false
	}
	domains, err := g.boundDomains(expr.Bounds, evaluator.State{}, nil)
	if err != nil {
		return false
	}
	for _, domain := range domains {
		empty := true
		err := domain.Each(func(value.Value) (bool, error) {
			empty = false
			return false, nil
		})
		if err == nil && empty {
			return true
		}
	}
	return false
}
