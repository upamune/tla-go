package checker

import (
	"context"
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func resolveInstances(ctx context.Context, module *parser.Module, loader ModuleLoader) (*parser.Module, error) {
	session := newInstanceSession(module.Name)
	return resolveInstancesScoped(ctx, module, loader, session)
}

func resolveInstancesScoped(ctx context.Context, module *parser.Module, loader ModuleLoader, session *instanceSession) (*parser.Module, error) {
	if !needsInstanceResolution(module, session) {
		return module, nil
	}
	r := newInstanceResolver(ctx, module, loader, session)
	if err := r.registerNested(); err != nil {
		return nil, err
	}
	if err := r.validateInstanceNames(); err != nil {
		return nil, err
	}
	result := *module
	result.Definitions = append([]parser.Definition(nil), module.Definitions...)
	result.Instances, result.Modules = nil, nil
	for index, instance := range module.Instances {
		if err := r.instantiate(&result, instance, index); err != nil {
			return nil, err
		}
	}
	if err := r.expandLetInstances(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func needsInstanceResolution(module *parser.Module, session *instanceSession) bool {
	return len(module.Instances) > 0 || len(module.Modules) > 0 || len(session.inherited[module]) >= 2 || moduleHasLetInstances(module)
}

func newInstanceResolver(ctx context.Context, module *parser.Module, loader ModuleLoader, session *instanceSession) instanceResolver {
	r := instanceResolver{ctx: ctx, root: module, loader: loader, nested: map[string]nestedModuleBinding{}, cache: map[string]*parser.Module{}, session: session, usedModules: map[string]bool{}}
	for _, name := range append(append([]string(nil), module.Extends...), session.declaredImports[module]...) {
		r.usedModules[name] = true
	}
	return r
}

type instanceSession struct {
	nextInstance    int
	active          map[string]bool
	inherited       map[*parser.Module][]nestedModuleBinding
	declaredImports map[*parser.Module][]string
	sourceUnits     map[string]*parser.Module
}

func newInstanceSession(root string) *instanceSession {
	return &instanceSession{active: map[string]bool{root: true}, inherited: map[*parser.Module][]nestedModuleBinding{}, declaredImports: map[*parser.Module][]string{}, sourceUnits: map[string]*parser.Module{}}
}

type nestedModuleBinding struct {
	module    *parser.Module
	parent    *parser.Module
	inherited bool
}

type instanceResolver struct {
	ctx         context.Context
	root        *parser.Module
	loader      ModuleLoader
	nested      map[string]nestedModuleBinding
	cache       map[string]*parser.Module
	session     *instanceSession
	usedModules map[string]bool
}

func (r *instanceResolver) registerNested() error {
	if err := r.registerInheritedNested(); err != nil {
		return err
	}
	for i := range r.root.Modules {
		module := &r.root.Modules[i]
		if _, ok := r.nested[module.Name]; ok || module.Name == r.root.Name || r.usedModules[module.Name] {
			return fmt.Errorf("duplicate nested module %s", module.Name)
		}
		prepared, err := r.prepareNestedModule(module)
		if err != nil {
			return err
		}
		r.nested[module.Name] = nestedModuleBinding{module: prepared, parent: r.root}
	}
	return nil
}

func (r *instanceResolver) instanceModule(instance parser.Instance) (*parser.Module, error) {
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	if binding, ok := r.nested[instance.Module]; ok {
		if !binding.inherited && binding.module.Location.Range.Start.Line >= instance.Location.Line {
			return nil, fmt.Errorf("nested module %s is not yet defined", instance.Module)
		}
		return binding.module, nil
	}
	if module, ok := r.cache[instance.Module]; ok {
		return module, nil
	}
	module, err := r.loadInstanceModule(instance.Module)
	if err != nil {
		return nil, err
	}
	r.cache[instance.Module] = module
	return module, nil
}

func (r *instanceResolver) loadInstanceModule(name string) (*parser.Module, error) {
	if r.session.active[name] {
		return nil, fmt.Errorf("cyclic instance dependency involving %s", name)
	}
	if r.loader == nil {
		return nil, fmt.Errorf("instance module %s requires a module loader", name)
	}
	source, err := r.loader(r.ctx, name)
	if err != nil {
		return nil, fmt.Errorf("load instance module %s: %w", name, err)
	}
	module, err := parser.ParseModule(source)
	if err != nil {
		return nil, &SyntaxError{Err: fmt.Errorf("module %s: %w", name, err)}
	}
	if module.Name != name {
		return nil, fmt.Errorf("module %s declares itself as %s", name, module.Name)
	}
	r.session.sourceUnits[name] = module
	return r.prepareInstanceModule(name, module)
}

func (r *instanceResolver) prepareInstanceModule(name string, module *parser.Module) (*parser.Module, error) {
	r.session.active[name] = true
	defer delete(r.session.active, name)
	module, err := resolveExtensionsScoped(r.ctx, module, r.loader, r.session)
	if err != nil {
		return nil, err
	}
	module, err = resolveInstancesScoped(r.ctx, module, r.loader, r.session)
	if err != nil {
		return nil, err
	}
	module, err = resolveSubexpressions(module)
	if err != nil {
		return nil, err
	}
	if err := r.validateUnit(module, nil); err != nil {
		return nil, err
	}
	return module, nil
}

func (r *instanceResolver) validateUnit(module, parent *parser.Module) error {
	if len(module.Instances) > 0 || len(module.Modules) > 0 || hasExternalExtensions(module.Extends) {
		return fmt.Errorf("instance module %s has unsupported nested or external dependencies", module.Name)
	}
	if !standardSubset(module.Extends, r.root.Extends) {
		return fmt.Errorf("instance module %s requires standard modules not extended by the root", module.Name)
	}
	check := *module
	if parent != nil {
		check = nestedScope(module, parent)
	}
	if err := validateResolved(&check); err != nil {
		return fmt.Errorf("instance module %s: %w", module.Name, err)
	}
	return nil
}

func standardSubset(names, available []string) bool {
	for _, name := range names {
		found := false
		for _, candidate := range available {
			found = found || name == candidate
		}
		if !found {
			return false
		}
	}
	return true
}
