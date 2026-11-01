package checker

import (
	"fmt"
	"sort"

	"github.com/upamune/tla-go/syntax/parser"
)

func (r *instanceResolver) prepareNestedModule(module *parser.Module) (*parser.Module, error) {
	module, err := r.extendNestedModule(module)
	if err != nil {
		return nil, err
	}
	module, err = r.expandNestedInstances(module)
	if err != nil {
		return nil, err
	}
	if err := r.validateUnit(module, r.root); err != nil {
		return nil, err
	}
	return module, nil
}

func (r *instanceResolver) extendNestedModule(module *parser.Module) (*parser.Module, error) {
	var dependencies []*parser.Module
	var standards []string
	for _, name := range module.Extends {
		_, nested := r.nested[name]
		if supportedModule(name) && !nested {
			r.usedModules[name] = true
			standards = append(standards, name)
			continue
		}
		dependency, err := r.nestedDependency(module, name)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, dependency)
		standards = append(standards, dependency.Extends...)
	}
	if len(dependencies) == 0 {
		return module, nil
	}
	if len(dependencies) > 1 {
		return nil, fmt.Errorf("multiple nested module extensions are not yet supported")
	}
	units := append(dependencies, module)
	return mergeExtensions(module, units, uniqueStandardModules(standards)), nil
}

func (r *instanceResolver) nestedDependency(module *parser.Module, name string) (*parser.Module, error) {
	binding, ok := r.nested[name]
	if !ok || (!binding.inherited && !nestedPositionBefore(binding.module.Location.Range.Start, module.Location.Range.Start)) {
		return nil, fmt.Errorf("nested module extension %s is not yet supported", name)
	}
	return isolateLocalDefinitions(binding.module)
}

func uniqueStandardModules(names []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, name := range names {
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result
}

func (r *instanceResolver) expandNestedInstances(module *parser.Module) (*parser.Module, error) {
	if len(module.Instances) == 0 && len(module.Modules) == 0 {
		return module, nil
	}
	scope := nestedScope(module, r.root)
	r.session.inherited[&scope] = r.visibleNestedBindings()
	expanded, err := resolveInstancesScoped(r.ctx, &scope, r.loader, r.session)
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

func (r *instanceResolver) visibleNestedBindings() []nestedModuleBinding {
	names := make([]string, 0, len(r.nested))
	for name := range r.nested {
		names = append(names, name)
	}
	sort.Strings(names)
	bindings := make([]nestedModuleBinding, 0, len(names))
	for _, name := range names {
		binding := r.nested[name]
		binding.inherited = true
		bindings = append(bindings, binding)
	}
	return bindings
}
