package checker

import (
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

func sourceInstances(module *parser.Module) []parser.Instance {
	instances := append([]parser.Instance(nil), module.Instances...)
	for _, definition := range module.Definitions {
		collectExpressionInstances(definition.Body, &instances)
		for _, bound := range definition.Bounds {
			collectExpressionInstances(bound.Set, &instances)
		}
	}
	for _, assumption := range module.Assumptions {
		collectExpressionInstances(assumption.Expression, &instances)
	}
	slices.SortStableFunc(instances, func(a, b parser.Instance) int {
		if a.Location.Line != b.Location.Line {
			return a.Location.Line - b.Location.Line
		}
		return a.Location.Column - b.Location.Column
	})
	return instances
}

func collectExpressionInstances(expr *parser.Expression, result *[]parser.Instance) {
	if !expressionHasLetInstance(expr) {
		return
	}
	*result = append(*result, expr.Instances...)
	for _, instance := range expr.Instances {
		for _, sub := range instance.Substitutions {
			collectExpressionInstances(sub.Expression, result)
		}
	}
	walkChildren(expr, func(child *parser.Expression) { collectExpressionInstances(child, result) })
}
