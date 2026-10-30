package checker

import (
	"maps"

	"github.com/upamune/tla-go/syntax/parser"
)

// A parameterized instance imposes state-level limits only on exports that
// depend on the variable substituted with that parameter.
func stateInstanceTargets(module *parser.Module, instance parser.Instance, replacements map[string]*parser.Expression) map[string][]bool {
	defs := make(map[string]parser.Definition, len(module.Definitions))
	for _, def := range module.Definitions {
		defs[def.Name] = def
	}
	variables := instanceVariableParameters(module.Variables, instance.Params, replacements)
	targets := make(map[string][]bool)
	for _, def := range module.Definitions {
		if def.Local {
			continue
		}
		targets[instance.Name+"!"+def.Name] = stateDefinitionTargets(def, variables, defs, len(instance.Params))
	}
	return targets
}

func instanceVariableParameters(variables, params []parser.Declaration, replacements map[string]*parser.Expression) map[string][]bool {
	result := map[string][]bool{}
	for _, variable := range variables {
		names := map[string]bool{}
		collectArgumentNames(replacements[variable.Name], names)
		limits := make([]bool, len(params))
		for i, param := range params {
			limits[i] = names[param.Name]
		}
		if hasStateLimit(limits) {
			result[variable.Name] = limits
		}
	}
	return result
}

func stateDefinitionTargets(def parser.Definition, variables map[string][]bool, defs map[string]parser.Definition, paramCount int) []bool {
	limits := make([]bool, paramCount)
	for name, variableLimits := range variables {
		if instanceDefinitionDependsOnVariable(def, name, defs) {
			for i, required := range variableLimits {
				limits[i] = limits[i] || required
			}
		}
	}
	return limits
}

func instanceDefinitionDependsOnVariable(def parser.Definition, variable string, defs map[string]parser.Definition) bool {
	return instanceDefinitionDependsOnVariableSeen(def, variable, defs, map[string]bool{})
}

func instanceDefinitionDependsOnVariableSeen(def parser.Definition, variable string, defs map[string]parser.Definition, seen map[string]bool) bool {
	if instanceVariableUsed(def.Body, variable, defs, seen) {
		return true
	}
	for _, bound := range def.Bounds {
		if bound.Set != nil && instanceVariableUsed(bound.Set, variable, defs, seen) {
			return true
		}
	}
	return false
}

func instanceVariableUsed(expr *parser.Expression, variable string, defs map[string]parser.Definition, seen map[string]bool) bool {
	if expr == nil {
		return false
	}
	if expr.Kind == "Identifier" && expr.Text == variable {
		return true
	}
	local, primed := instanceVariableLocalDefinitions(expr.Definitions, variable, defs)
	if primed {
		return true
	}
	if instanceDefinitionUsesVariable(expr, variable, local, seen) {
		return true
	}
	found := false
	walkTemporalChildren(expr, func(child *parser.Expression) {
		if !found {
			found = instanceVariableUsed(child, variable, local, seen)
		}
	})
	return found
}

func instanceVariableLocalDefinitions(definitions []parser.Definition, variable string, defs map[string]parser.Definition) (map[string]parser.Definition, bool) {
	if len(definitions) == 0 {
		return defs, false
	}
	local := maps.Clone(defs)
	for _, def := range definitions {
		local[def.Name] = def
	}
	for _, def := range definitions {
		if instanceVariablePrimed(def.Body, variable, local) {
			return local, true
		}
	}
	return local, false
}

// A local definition can impose a level constraint even when its value is
// unused: substituting an action into a primed expression is invalid.
func instanceVariablePrimed(expr *parser.Expression, variable string, defs map[string]parser.Definition) bool {
	if expr.Kind == "Prime" && instanceVariableUsed(expr.Args[0], variable, defs, map[string]bool{}) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = instanceVariablePrimed(child, variable, defs)
		}
	})
	return found
}

func instanceDefinitionUsesVariable(expr *parser.Expression, variable string, defs map[string]parser.Definition, seen map[string]bool) bool {
	if expr.Kind != "Identifier" && expr.Kind != "Apply" && expr.Kind != "OperatorReference" {
		return false
	}
	def, ok := defs[expr.Text]
	if !ok || seen[expr.Text] {
		return false
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return instanceDefinitionDependsOnVariableSeen(def, variable, defs, seen)
}
