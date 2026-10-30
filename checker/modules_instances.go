package checker

import "github.com/upamune/tla-go/syntax/parser"

func (r *extensionResolver) expandExtensionInstances(module *parser.Module) (*parser.Module, error) {
	if len(module.Instances) == 0 && len(module.Modules) == 0 {
		return module, nil
	}
	units := r.scopeUnits(module)
	scope := mergeExtensions(module, units, scopeStandards(units))
	r.session.inherited[scope] = r.inheritedNested(units)
	expanded, err := resolveInstancesScoped(r.ctx, scope, r.loader, r.session)
	if err != nil {
		return nil, err
	}
	result := *module
	result.Definitions = append([]parser.Definition(nil), module.Definitions...)
	for _, definition := range expanded.Definitions[len(scope.Definitions):] {
		definition.Location.Range.Module = module.Name
		definition.Location.Range.Start = definition.Location.Position
		result.Definitions = append(result.Definitions, definition)
	}
	result.Instances, result.Modules = nil, nil
	return &result, nil
}
