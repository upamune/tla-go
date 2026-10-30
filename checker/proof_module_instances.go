package checker

import "github.com/upamune/tla-go/syntax/parser"

func parentProofInstances(module *parser.Module) []parser.Instance {
	instances := append([]parser.Instance(nil), module.Instances...)
	for _, definition := range module.Definitions {
		collectProofInstances(definition.Body, &instances)
		for _, bound := range definition.Bounds {
			collectProofInstances(bound.Set, &instances)
		}
	}
	for _, assumption := range module.Assumptions {
		collectProofInstances(assumption.Expression, &instances)
	}
	for _, theorem := range module.Theorems {
		if theorem.Name == "" {
			collectProofInstances(theorem.Statement, &instances)
		}
	}
	return instances
}

func collectProofInstances(expr *parser.Expression, instances *[]parser.Instance) {
	if expr == nil {
		return
	}
	*instances = append(*instances, expr.Instances...)
	for _, instance := range expr.Instances {
		for _, sub := range instance.Substitutions {
			collectProofInstances(sub.Expression, instances)
		}
	}
	walkChildren(expr, func(child *parser.Expression) { collectProofInstances(child, instances) })
}
