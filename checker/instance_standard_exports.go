package checker

import (
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

// Standard definitions inherited by a module are also exported by INSTANCE.
func withInheritedStandardDefinitions(module *parser.Module) *parser.Module {
	inherited := inheritedStandardDefinitions(module.Extends)
	if len(inherited) == 0 {
		return module
	}
	result := *module
	result.Definitions = append([]parser.Definition(nil), module.Definitions...)
	for _, definition := range inherited {
		if !hasModuleDefinition(result.Definitions, definition.Name) {
			result.Definitions = append(result.Definitions, definition)
		}
	}
	return &result
}

func inheritedStandardDefinitions(extends []string) []parser.Definition {
	var inherited []parser.Definition
	if slices.Contains(extends, "Naturals") || slices.Contains(extends, "Integers") {
		inherited = append(inherited, builtinNaturalsModule().Definitions...)
	}
	if slices.Contains(extends, "Integers") {
		inherited = append(inherited, builtinIntegerDefinitions()...)
	}
	if slices.Contains(extends, "TLC") || slices.Contains(extends, "Bags") {
		inherited = append(inherited, builtinTLCModule().Definitions...)
	}
	return inherited
}

func hasModuleDefinition(definitions []parser.Definition, name string) bool {
	for _, definition := range definitions {
		if definition.Name == name {
			return true
		}
	}
	return false
}

func builtinIntegerDefinitions() []parser.Definition {
	parameter := parser.Declaration{Name: "!integer-argument"}
	return []parser.Definition{
		{Kind: "Operator", Name: "Int", BuiltinInstance: true, Body: &parser.Expression{Kind: "Identifier", Text: "!naturals!Int", BuiltinNatural: true}},
		{Kind: "Operator", Name: "-.", BuiltinInstance: true, Params: []parser.Declaration{parameter}, Body: &parser.Expression{Kind: "Apply", Text: "!naturals!-.", BuiltinNatural: true, Args: []*parser.Expression{{Kind: "Identifier", Text: parameter.Name}}}},
	}
}
