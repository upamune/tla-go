package checker

import (
	"maps"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (m *model) expandFairness() error {
	var expanded []fairnessCondition
	for _, fair := range m.fairness {
		err := m.expandBindings(fair.bounds, nil, func(locals map[string]value.Value) error {
			instance := fair
			instance.locals = maps.Clone(locals)
			instance.bounds = nil
			expanded = append(expanded, instance)
			return nil
		})
		if err != nil {
			return err
		}
	}
	m.fairness = expanded
	return nil
}

func (m *model) expandBindings(bounds []parser.Bound, locals map[string]value.Value, visit func(map[string]value.Value) error) error {
	if len(bounds) == 0 {
		return visit(locals)
	}
	bound := bounds[0]
	domain, err := m.program.Evaluate(bound.Set, evaluator.Context{Constants: m.constants, Locals: locals})
	if err != nil {
		return err
	}
	return domain.Each(func(item value.Value) (bool, error) {
		next, err := bindBound(bound, item, locals, "fairness tuple binding has wrong arity")
		if err != nil {
			return false, err
		}
		return true, m.expandBindings(bounds[1:], next, visit)
	})
}
