package checker

import "github.com/upamune/tla-go/syntax/parser"

// The renamer supports operator parameters and ordinary expressions. Refuse
// binders, LET, and selectors until their lexical scope is preserved too.
func safeExtensionUnit(unit *parser.Module) bool {
	return safeExtensionDefinitions(unit.Definitions) && safeExtensionAssumptions(unit.Assumptions) && safeExtensionInstances(unit.Instances)
}

func safeExtensionDefinitions(definitions []parser.Definition) bool {
	for _, d := range definitions {
		if len(d.Bounds) != 0 || !safeExtensionExpression(d.Body) {
			return false
		}
	}
	return true
}

func safeExtensionAssumptions(assumptions []parser.Assumption) bool {
	for _, a := range assumptions {
		if !safeExtensionExpression(a.Expression) {
			return false
		}
	}
	return true
}

func safeExtensionInstances(instances []parser.Instance) bool {
	for _, inst := range instances {
		if len(inst.Params) != 0 {
			return false
		}
		for _, sub := range inst.Substitutions {
			if !safeExtensionExpression(sub.Expression) {
				return false
			}
		}
	}
	return true
}

func safeExtensionExpression(expr *parser.Expression) bool {
	if expr == nil {
		return true
	}
	if len(expr.Bounds) != 0 || len(expr.Definitions) != 0 || len(expr.Instances) != 0 || len(expr.Names) != 0 || len(expr.Selectors) != 0 {
		return false
	}
	safe := true
	walkChildren(expr, func(child *parser.Expression) {
		if !safeExtensionExpression(child) {
			safe = false
		}
	})
	return safe
}
