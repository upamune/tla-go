package main

import (
	"context"
	"maps"
	"slices"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/syntax/parser"
)

// javaStandardModules are the modules Java TLC's TLAMonolithCreator keeps
// in the EXTENDS list instead of embedding (tla2sany StandardModules).
var javaStandardModules = map[string]bool{
	"FiniteSets": true, "Sequences": true, "Bags": true, "Naturals": true, "Integers": true,
	"Reals": true, "RealTime": true, "Randomization": true, "TLC": true,
}

// standardModuleExtends lists the EXTENDS clauses of the standard modules,
// and standardModuleInstances their LOCAL INSTANCE modules.
var (
	standardModuleExtends = map[string][]string{
		"Integers": {"Naturals"},
		"Reals":    {"Integers"},
		"RealTime": {"Reals"},
		"Bags":     {"TLC"},
	}
	standardModuleInstances = map[string][]string{
		"TLC":           {"Naturals", "Sequences", "FiniteSets"},
		"Sequences":     {"Naturals"},
		"FiniteSets":    {"Naturals", "Sequences"},
		"Bags":          {"Naturals"},
		"Randomization": {"Naturals", "FiniteSets"},
	}
)

// specTEModules describes the modules a specification depends on, as
// Java TLC derives them from SANY: the transitive EXTENDS of the root and of
// instantiated modules, and the user modules SANY parses after the root.
type specTEModules struct {
	extends  []string
	embedded []specTEEmbedded
}

type specTEEmbedded struct{ name, source string }

type specTEModuleWalk struct {
	loader  checker.ModuleLoader
	parsed  map[string]*parser.Module
	sources map[string]string
}

func discoverSpecTEModules(root *parser.Module, loader checker.ModuleLoader) specTEModules {
	walk := specTEModuleWalk{loader: loader, parsed: map[string]*parser.Module{root.Name: root}, sources: map[string]string{}}
	order := walk.parseOrder(root)
	var extends []string
	walk.collectExtends(root, &extends, map[string]bool{})
	// Java collects the names from a HashSet of SANY module nodes, whose
	// order follows node identifiers. Modules SANY processes later come first
	// in the tested cases, which matters when names share a hash bucket.
	rank := map[string]int{}
	for i, name := range walk.processingOrder(root) {
		rank[name] = i
	}
	slices.SortStableFunc(extends, func(a, b string) int { return rank[b] - rank[a] })
	var result specTEModules
	result.extends = extends
	// TLAMonolithCreator embeds the non-standard modules in reverse parse order.
	for _, name := range slices.Backward(order) {
		if source, ok := walk.sources[name]; ok && !javaStandardModules[name] {
			result.embedded = append(result.embedded, specTEEmbedded{name, source})
		}
	}
	return result
}

// parseOrder replays SANY's SpecObj.loadSpec, which logs the files it
// parses: each round resolves the first unresolved EXTENDS name found depth
// first from the root, and only when none is left the first unresolved
// INSTANCE name. It returns the modules after the root in parse order.
func (w *specTEModuleWalk) parseOrder(root *parser.Module) []string {
	load := sanyLoad{walk: w, external: map[string]*sanyNode{}}
	rootNode := newSANYNode(root, map[string]*sanyNode{})
	load.external[root.Name] = rootNode
	for {
		owner, name := rootNode.nextUnresolved(false, map[*sanyNode]bool{})
		if owner == nil {
			owner, name = rootNode.nextUnresolved(true, map[*sanyNode]bool{})
		}
		if owner == nil {
			return load.order
		}
		owner.resolved[name] = load.module(name)
	}
}

// sanyNode is a module in SANY's module relationships: its EXTENDS and
// INSTANCE names, its inner modules, and the names its context resolves.
// The context of an inner module starts with the inner modules defined
// before it, and an INSTANCE of a module's own inner module needs no file.
type sanyNode struct {
	extends, instances []string
	inner              []*sanyNode
	internal           map[string]bool
	resolved           map[string]*sanyNode
}

func newSANYNode(module *parser.Module, context map[string]*sanyNode) *sanyNode {
	node := &sanyNode{extends: module.Extends, internal: map[string]bool{}, resolved: maps.Clone(context)}
	for _, instance := range module.Instances {
		node.instances = append(node.instances, instance.Module)
	}
	peers := maps.Clone(context)
	for i := range module.Modules {
		inner := newSANYNode(&module.Modules[i], peers)
		node.inner = append(node.inner, inner)
		node.internal[module.Modules[i].Name] = true
		peers[module.Modules[i].Name] = inner
	}
	return node
}

// nextUnresolved follows SpecObj.findNextUnresolvedExtentionBody, or
// findNextUnresolvedInstantiationBody for instances: an unresolved name of
// the module itself, else one reachable through its resolved extendees,
// resolved instancees, and inner modules, in that order.
func (n *sanyNode) nextUnresolved(instances bool, visited map[*sanyNode]bool) (*sanyNode, string) {
	if visited[n] {
		return nil, ""
	}
	visited[n] = true
	if name, ok := n.unresolved(instances); ok {
		return n, name
	}
	for _, child := range n.children() {
		if owner, name := child.nextUnresolved(instances, visited); owner != nil {
			return owner, name
		}
	}
	return nil, ""
}

func (n *sanyNode) unresolved(instances bool) (string, bool) {
	names := n.extends
	if instances {
		names = n.instances
	}
	for _, name := range names {
		if n.resolved[name] == nil && !(instances && n.internal[name]) {
			return name, true
		}
	}
	return "", false
}

func (n *sanyNode) children() []*sanyNode {
	var children []*sanyNode
	for _, name := range slices.Concat(n.extends, n.instances) {
		if child := n.resolved[name]; child != nil {
			children = append(children, child)
		}
	}
	return append(children, n.inner...)
}

// sanyLoad holds the external modules, one node per name, in parse order.
type sanyLoad struct {
	walk     *specTEModuleWalk
	external map[string]*sanyNode
	order    []string
}

func (l *sanyLoad) module(name string) *sanyNode {
	if node, ok := l.external[name]; ok {
		return node
	}
	node := &sanyNode{extends: standardModuleExtends[name], instances: standardModuleInstances[name], resolved: map[string]*sanyNode{}}
	if module := l.walk.module(name); module != nil {
		node = newSANYNode(module, map[string]*sanyNode{})
	}
	l.external[name] = node
	l.order = append(l.order, name)
	return node
}

// processingOrder lists modules after their dependencies, the order in
// which SANY processes them semantically.
func (w *specTEModuleWalk) processingOrder(root *parser.Module) []string {
	var order []string
	seen := map[string]bool{}
	var visit func(name string)
	visit = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		dependencies := slices.Concat(standardModuleExtends[name], standardModuleInstances[name])
		if module := w.module(name); module != nil {
			dependencies = moduleDependencies(module)
		}
		for _, dependency := range dependencies {
			visit(dependency)
		}
		order = append(order, name)
	}
	visit(root.Name)
	return order
}

func moduleDependencies(module *parser.Module) []string {
	names := slices.Clone(module.Extends)
	for _, instance := range module.Instances {
		names = append(names, instance.Module)
	}
	return names
}

// collectExtends follows MCParser: the transitive EXTENDS of the root and
// of every module it instantiates, directly or indirectly.
func (w *specTEModuleWalk) collectExtends(module *parser.Module, names *[]string, instantiated map[string]bool) {
	w.addExtends(module.Extends, names, map[string]bool{})
	for _, instance := range module.Instances {
		if instantiated[instance.Module] {
			continue
		}
		instantiated[instance.Module] = true
		if dependency := w.module(instance.Module); dependency != nil {
			w.collectExtends(dependency, names, instantiated)
		} else {
			w.addExtends(standardModuleExtends[instance.Module], names, map[string]bool{})
		}
	}
}

func (w *specTEModuleWalk) addExtends(extends []string, names *[]string, seen map[string]bool) {
	for _, name := range extends {
		if seen[name] {
			continue
		}
		seen[name] = true
		if !slices.Contains(*names, name) {
			*names = append(*names, name)
		}
		next := standardModuleExtends[name]
		if dependency := w.module(name); dependency != nil {
			next = dependency.Extends
		}
		w.addExtends(next, names, seen)
	}
}

// module parses a user module found by the loader; standard modules
// without a source file return nil.
func (w *specTEModuleWalk) module(name string) *parser.Module {
	if module, ok := w.parsed[name]; ok {
		return module
	}
	w.parsed[name] = nil
	source, err := w.loader(context.Background(), name)
	if err != nil {
		return nil
	}
	module, err := parser.ParseModule(source)
	if err != nil {
		return nil
	}
	w.parsed[name], w.sources[name] = module, source
	return module
}
