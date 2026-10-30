package checker

import (
	"fmt"
	"slices"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

// Standard operators participate in configuration substitutions even when
// they are implemented directly by the evaluator instead of as definitions.
func configStandardOperator(module *parser.Module, name string) (parser.Definition, bool) {
	r := resolver{}
	for _, extended := range module.Extends {
		r.includeModule(extended)
	}
	sym, ok := r.lookupStandard(name)
	if !ok || sym.arity == 0 {
		return parser.Definition{}, false
	}
	definition := configConstantDefinition(parser.Declaration{Name: name, Arity: sym.arity})
	for i, arity := range sym.params {
		definition.Params[i].Arity = arity
	}
	return definition, true
}

// Represent overridden standard values as definitions so all existing name
// resolution, lexical scope and model evaluation paths use the replacement.
func addScopedStandardDefinitions(module *parser.Module, cfg config.Config) error {
	added := map[string]int{}
	for _, sub := range cfg.Substitutions {
		if !isScopedStandardValue(module, sub) {
			continue
		}
		index, ok := added[sub.Name]
		if !ok {
			var err error
			index, err = configStandardIndex(module, sub.Name)
			if err != nil {
				return err
			}
			added[sub.Name] = index
		}
		module.Definitions[index].Origins = append(module.Definitions[index].Origins, parser.DefinitionOrigin{Module: sub.Module, Name: sub.Name})
	}
	return nil
}

func configStandardIndex(module *parser.Module, name string) (int, error) {
	for i, definition := range module.Definitions {
		if definition.Name == name {
			if definition.ConfigStandard {
				return i, nil
			}
			return 0, fmt.Errorf("standard value %s conflicts with a module definition", name)
		}
	}
	index := len(module.Definitions)
	module.Definitions = append(module.Definitions, parser.Definition{Kind: "Operator", Name: name, ConfigStandard: true, Body: &parser.Expression{Kind: "Identifier", Text: "TRUE"}})
	return index, nil
}

func isScopedStandardValue(module *parser.Module, sub config.Substitution) bool {
	if ignoredScopedSubstitution(module, sub) {
		return false
	}
	switch sub.Module {
	case "Naturals":
		return sub.Name == "Nat" && hasNaturalModule(module.Extends)
	case "Integers":
		return (sub.Name == "Nat" || sub.Name == "Int") && slices.Contains(module.Extends, "Integers")
	}
	return false
}

func hasNaturalModule(modules []string) bool {
	return slices.Contains(modules, "Naturals") || slices.Contains(modules, "Integers") || slices.Contains(modules, "Sequences")
}
