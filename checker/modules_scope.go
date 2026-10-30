package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func (r *extensionResolver) validateScope(module *parser.Module) error {
	units := r.scopeUnits(module)
	standards := scopeStandards(units)
	merged := mergeExtensions(module, units, standards)
	if err := validateResolved(merged); err != nil {
		return fmt.Errorf("module %s: %w", module.Name, err)
	}
	return nil
}

func (r *extensionResolver) scopeUnits(module *parser.Module) []*parser.Module {
	seen := map[string]bool{}
	var units []*parser.Module
	var visit func(*parser.Module)
	visit = func(current *parser.Module) {
		if seen[current.Name] {
			return
		}
		seen[current.Name] = true
		for _, name := range current.Extends {
			if dependency, ok := r.byName[name]; ok {
				visit(dependency)
			}
		}
		units = append(units, current)
	}
	visit(module)
	return units
}

func scopeStandards(units []*parser.Module) []string {
	seen := map[string]bool{}
	var result []string
	for _, unit := range units {
		for _, name := range unit.Extends {
			if supportedModule(name) && !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	}
	return result
}
