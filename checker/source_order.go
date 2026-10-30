package checker

import "github.com/upamune/tla-go/syntax/parser"

// sourceReferences tracks one module scope within a parsed source unit.
// Nested scopes share their enclosing unit's semantic analysis pass.
type sourceReferences struct {
	unit       string
	extends    []string
	instances  []string
	inner      []*sourceReferences
	allExtends map[string]bool
}

type sourceRelation struct {
	scope *sourceReferences
	name  string
}

type sourceOrder struct {
	units        map[string]*parser.Module
	references   map[string]*sourceReferences
	inner        map[string]*sourceReferences
	relations    map[sourceRelation]bool
	dependencies map[string][]string
}

func rankModuleSources(root *parser.Module, units map[string]*parser.Module, ranks map[string]int) {
	if original := units[root.Name]; original != nil {
		root = original
	}
	order := newSourceOrder(units)
	rootRefs := order.buildReferences(root, root.Name, map[string]bool{})
	order.references[root.Name] = rootRefs
	rankParsedTokens(root.SourceTokens, ranks, true)
	order.rankDependencies(rootRefs, ranks)
	rankStrings([]string{"LAMBDA", "$$InAssume", "nullODN", "nullOpArg", "nullLabelNode"}, ranks)
	order.rankSemanticUnit(root.Name, root, map[string]bool{}, ranks)
	rankStrings(tlcRuntimeStrings, ranks)
}

func newSourceOrder(units map[string]*parser.Module) *sourceOrder {
	return &sourceOrder{units: units, references: map[string]*sourceReferences{}, inner: map[string]*sourceReferences{}, relations: map[sourceRelation]bool{}, dependencies: map[string][]string{}}
}

func (o *sourceOrder) buildReferences(module *parser.Module, unit string, allExtends map[string]bool) *sourceReferences {
	refs := &sourceReferences{unit: unit, extends: module.Extends, allExtends: allExtends}
	for _, name := range refs.extends {
		allExtends[name] = true
	}
	for _, instance := range sourceInstances(module) {
		refs.instances = append(refs.instances, instance.Module)
	}
	for i := range module.Modules {
		child := &module.Modules[i]
		nested := o.buildReferences(child, unit, allExtends)
		o.inner[child.Name] = nested
		refs.inner = append(refs.inner, nested)
	}
	return refs
}

func (o *sourceOrder) unitReferences(name string) *sourceReferences {
	if refs := o.references[name]; refs != nil {
		return refs
	}
	module := sourceUnit(name, o.units)
	if module == nil {
		module = &parser.Module{Name: name}
	}
	refs := o.buildReferences(module, name, map[string]bool{})
	o.references[name] = refs
	return refs
}

func (o *sourceOrder) referenced(name string) *sourceReferences {
	if refs := o.inner[name]; refs != nil {
		return refs
	}
	return o.unitReferences(name)
}

func sourceUnit(name string, units map[string]*parser.Module) *parser.Module {
	if standard := tlcStandardSourceUnits[name]; standard != nil {
		return standard
	}
	return units[name]
}
