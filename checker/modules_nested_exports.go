package checker

import "github.com/upamune/tla-go/syntax/parser"

func (r *extensionResolver) inheritedNested(units []*parser.Module) []nestedModuleBinding {
	var result []nestedModuleBinding
	for _, unit := range units {
		result = append(result, r.nestedExports[unit.Name]...)
	}
	return result
}

func (r *extensionResolver) recordNestedModules(original, expanded, isolated *parser.Module) {
	if len(original.Modules) == 0 {
		return
	}
	units := r.scopeUnits(isolated)
	parent := mergeExtensions(isolated, units, scopeStandards(units))
	replacements := localReplacements(expanded)
	for i := range original.Modules {
		nested := rewriteNestedParentLocals(&original.Modules[i], replacements)
		r.nestedExports[original.Name] = append(r.nestedExports[original.Name], nestedModuleBinding{module: nested, parent: parent, inherited: true})
	}
}

func rewriteNestedParentLocals(module *parser.Module, replacements map[string]*parser.Expression) *parser.Module {
	if len(replacements) == 0 {
		return module
	}
	renamer := alphaRenamer{}
	definitions := make([]parser.Definition, len(module.Definitions))
	for i, definition := range module.Definitions {
		definitions[i] = renamer.definition(definition, nil)
	}
	assumptions := append([]parser.Assumption(nil), module.Assumptions...)
	for i := range assumptions {
		assumptions[i].Expression = renamer.expression(assumptions[i].Expression, nil)
	}
	result := *module
	result.Definitions = renameLocalDefinitions(definitions, replacements)
	result.Assumptions = renameLocalAssumptions(assumptions, replacements)
	return &result
}
