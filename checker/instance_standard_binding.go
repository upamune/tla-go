package checker

import (
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

func instanceStandardNames(module *parser.Module) map[string]string {
	names := map[string]string{}
	for _, name := range module.Extends {
		addInstanceStandardNames(names, name)
	}
	for _, declaration := range module.Constants {
		delete(names, declaration.Name)
	}
	for _, declaration := range module.Variables {
		delete(names, declaration.Name)
	}
	for _, definition := range module.Definitions {
		if !definition.BuiltinInstance {
			delete(names, definition.Name)
		}
	}
	return names
}

func addInstanceStandardNames(names map[string]string, module string) {
	if standardHasNaturals(module) {
		for name := range arithmeticOperators {
			if name != "-." || module == "Integers" {
				names[name] = "Naturals"
			}
		}
		names["Nat"] = "Naturals"
	}
	if module == "Integers" {
		names["Int"] = "Naturals"
	}
	addNamedStandardOperators(names, module)
}

func addNamedStandardOperators(names map[string]string, module string) {
	switch module {
	case "TLC":
		addStandardMap(names, module, tlcOperators)
		names["Any"], names["JavaTime"] = module, module
	case "TLCExt":
		addStandardMap(names, module, tlcextOperators)
	case "Sequences":
		addStandardMap(names, module, sequenceOperators)
	case "FiniteSets":
		names["Cardinality"], names["IsFiniteSet"] = module, module
	case "Json":
		addStandardMap(names, module, jsonOperators)
	case "Randomization":
		addStandardMap(names, module, randomOperators)
	case "Bags":
		addInstanceBagNames(names)
	}
}

func addStandardMap(names map[string]string, module string, operators map[string]int) {
	for name := range operators {
		names[name] = module
	}
}

func addInstanceBagNames(names map[string]string) {
	addStandardMap(names, "Bags", bagOperators)
	names["EmptyBag"] = "Bags"
}

func markInstanceStandardDefinition(definition *parser.Definition, names map[string]string) {
	if len(names) == 0 {
		return
	}
	markInstanceStandardExpression(definition.Body, names)
	for _, bound := range definition.Bounds {
		markInstanceStandardExpression(bound.Set, names)
	}
}

func markInstanceStandardExpression(expr *parser.Expression, names map[string]string) {
	if expr == nil {
		return
	}
	if !expr.BuiltinNatural && !expr.BuiltinTLC && expr.BuiltinStandard == "" && slices.Contains([]string{"Apply", "Identifier"}, expr.Kind) {
		markInstanceStandardHead(expr, names[expr.Text])
	}
	walkChildren(expr, func(child *parser.Expression) { markInstanceStandardExpression(child, names) })
}

func markInstanceStandardHead(expr *parser.Expression, module string) {
	switch module {
	case "Naturals":
		expr.BuiltinNatural = true
		expr.Text = "!naturals!" + expr.Text
	case "TLC":
		expr.BuiltinTLC = true
		expr.Text = "!tlc!" + expr.Text
	case "FiniteSets", "Sequences", "Bags", "Json", "Randomization", "TLCExt":
		expr.BuiltinStandard = module
	}
}
