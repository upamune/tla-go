package checker

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/upamune/tla-go/syntax/parser"
)

type extensionResolver struct {
	ctx           context.Context
	loader        ModuleLoader
	visiting      map[string]bool
	loaded        map[string]bool
	units         []*parser.Module
	standard      []string
	stdSeen       map[string]bool
	byName        map[string]*parser.Module
	root          string
	session       *instanceSession
	nestedExports map[string][]nestedModuleBinding
}

func resolveExtensions(ctx context.Context, module *parser.Module, loader ModuleLoader) (*parser.Module, error) {
	session := newInstanceSession(module.Name)
	return resolveExtensionsScoped(ctx, module, loader, session)
}

func resolveExtensionsScoped(ctx context.Context, module *parser.Module, loader ModuleLoader, session *instanceSession) (*parser.Module, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateRecursiveModule(module); err != nil {
		return nil, err
	}
	if !hasExternalExtensions(module.Extends) {
		return module, nil
	}
	r := extensionResolver{ctx: ctx, loader: loader, visiting: map[string]bool{}, loaded: map[string]bool{}, stdSeen: map[string]bool{}, byName: map[string]*parser.Module{}, root: module.Name, session: session, nestedExports: map[string][]nestedModuleBinding{}}
	if err := r.collect(module); err != nil {
		return nil, err
	}
	if len(r.units) == 1 {
		return module, nil
	}
	result := mergeExtensions(module, r.units, r.standard)
	r.session.inherited[result] = r.inheritedNested(r.units)
	r.session.declaredImports[result] = append([]string(nil), module.Extends...)
	return result, nil
}

func hasExternalExtensions(names []string) bool {
	for _, name := range names {
		if !supportedModule(name) {
			return true
		}
	}
	return false
}

func (r *extensionResolver) collect(module *parser.Module) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if err := validateRecursiveModule(module); err != nil {
		return err
	}
	if r.visiting[module.Name] {
		return moduleCycleError(module.Name)
	}
	if r.loaded[module.Name] {
		return nil
	}
	r.visiting[module.Name] = true
	defer delete(r.visiting, module.Name)
	for _, name := range module.Extends {
		if err := r.collectExtension(name); err != nil {
			return err
		}
	}
	module, err := r.prepareCollectedModule(module)
	if err != nil {
		return err
	}
	r.loaded[module.Name] = true
	r.byName[module.Name] = module
	r.units = append(r.units, module)
	return nil
}

func (r *extensionResolver) prepareCollectedModule(module *parser.Module) (*parser.Module, error) {
	if module.Name == r.root {
		return module, nil
	}
	return r.prepareExtension(module)
}

func (r *extensionResolver) prepareExtension(module *parser.Module) (*parser.Module, error) {
	original := module
	module, err := r.expandExtensionInstances(module)
	if err != nil {
		return nil, err
	}
	module, err = r.resolveExtensionSubexpressions(module)
	if err != nil {
		return nil, err
	}
	if err := r.validateScope(module); err != nil {
		return nil, err
	}
	isolated, err := isolateLocalDefinitions(module)
	if err != nil {
		return nil, err
	}
	r.recordNestedModules(original, module, isolated)
	return isolated, nil
}

func (r *extensionResolver) collectExtension(name string) error {
	if supportedModule(name) {
		if !r.stdSeen[name] {
			r.stdSeen[name] = true
			r.standard = append(r.standard, name)
		}
		return nil
	}
	if r.visiting[name] {
		return moduleCycleError(name)
	}
	if r.loaded[name] {
		return nil
	}
	if r.session.active[name] {
		return moduleCycleError(name)
	}
	r.session.active[name] = true
	defer delete(r.session.active, name)
	module, err := r.load(name)
	if err != nil {
		return err
	}
	return r.collect(module)
}

func (r *extensionResolver) load(name string) (*parser.Module, error) {
	source, err := r.extensionSource(name)
	if err != nil {
		return nil, err
	}
	module, err := parser.ParseModule(source)
	if err != nil {
		return nil, &SyntaxError{Err: fmt.Errorf("module %s: %w", name, err)}
	}
	if module.Name != name {
		return nil, moduleNameError(name, module.Name)
	}
	r.session.sourceUnits[name] = module
	if err := r.session.recordSource(name, source); err != nil {
		return nil, err
	}
	return module, nil
}

func (r *extensionResolver) extensionSource(name string) (string, error) {
	var missing error
	if r.loader != nil {
		source, err := r.loader(r.ctx, name)
		if err == nil {
			return source, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", moduleLoadError("module", name, err)
		}
		missing = err
	}
	if source, ok := embeddedStandardSource(name); ok {
		return source, nil
	}
	if r.loader == nil {
		return "", fmt.Errorf("module %s requires a module loader", name)
	}
	return "", moduleLoadError("module", name, missing)
}
