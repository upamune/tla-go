package main

import (
	"context"
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
// and standardModuleDependencies also their LOCAL INSTANCE modules.
var (
	standardModuleExtends = map[string][]string{
		"Integers": {"Naturals"},
		"Reals":    {"Integers"},
		"RealTime": {"Reals"},
		"Bags":     {"TLC"},
	}
	standardModuleDependencies = map[string][]string{
		"TLC":           {"Naturals", "Sequences", "FiniteSets"},
		"Integers":      {"Naturals"},
		"Reals":         {"Integers"},
		"Sequences":     {"Naturals"},
		"FiniteSets":    {"Naturals", "Sequences"},
		"Bags":          {"TLC", "Naturals"},
		"RealTime":      {"Reals"},
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

// parseOrder visits EXTENDS and INSTANCE dependencies breadth first, the
// order in which SANY logs the files it parses.
func (w *specTEModuleWalk) parseOrder(root *parser.Module) []string {
	seen := map[string]bool{root.Name: true}
	queue := []*parser.Module{root}
	var order []string
	for len(queue) > 0 {
		module := queue[0]
		queue = queue[1:]
		for _, name := range moduleDependencies(module) {
			if seen[name] {
				continue
			}
			seen[name] = true
			order = append(order, name)
			if dependency := w.module(name); dependency != nil {
				queue = append(queue, dependency)
			}
		}
	}
	return order
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
		dependencies := standardModuleDependencies[name]
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
