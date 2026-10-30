package checker

import (
	"maps"

	"github.com/upamune/tla-go/syntax/parser"
)

func collectStateArgumentShadows(expr *parser.Expression, defs map[string]parser.Definition, shadows map[string]bool) {
	for _, bound := range expr.Bounds {
		markStateShadows(bound.Names, defs, shadows)
	}
	if expr.Kind == "Lambda" || expr.Kind == "UnboundedQuantifier" {
		markStateShadows(expr.Names, defs, shadows)
	}
	for _, def := range expr.Definitions {
		for _, param := range def.Params {
			if _, ok := defs[param.Name]; ok {
				shadows[param.Name] = true
			}
		}
	}
	walkChildren(expr, func(child *parser.Expression) { collectStateArgumentShadows(child, defs, shadows) })
}

func markStateShadows(names []string, defs map[string]parser.Definition, shadows map[string]bool) {
	for _, name := range names {
		if _, ok := defs[name]; ok {
			shadows[name] = true
		}
	}
}

func stateProbeLet(expr *parser.Expression, scope map[string]bool, defs map[string]parser.Definition, seen map[string]bool) bool {
	localDefs := maps.Clone(defs)
	localScope := maps.Clone(scope)
	if localScope == nil {
		localScope = map[string]bool{}
	}
	for _, def := range expr.Definitions {
		localDefs[def.Name] = def
		delete(localScope, def.Name)
	}
	for _, def := range expr.Definitions {
		if stateProbePrimed(def.Body, stateProbeDefinitionScope(def, localScope), localDefs) {
			return true
		}
	}
	localScope = stateProbeScope(expr, localScope)
	found := false
	walkTemporalChildren(expr, func(child *parser.Expression) {
		if !found {
			found = stateProbeUsed(child, localScope, localDefs, seen)
		}
	})
	return found
}

func stateProbeDefinitionScope(def parser.Definition, scope map[string]bool) map[string]bool {
	local := maps.Clone(scope)
	if local == nil {
		local = map[string]bool{}
	}
	for _, param := range def.Params {
		local[param.Name] = true
	}
	for _, bound := range def.Bounds {
		for _, name := range bound.Names {
			local[name] = true
		}
	}
	return local
}

func stateProbeScope(expr *parser.Expression, scope map[string]bool) map[string]bool {
	if len(expr.Bounds) == 0 && expr.Kind != "Lambda" && expr.Kind != "UnboundedQuantifier" {
		return scope
	}
	local := maps.Clone(scope)
	if local == nil {
		local = map[string]bool{}
	}
	addStateLocalNames(local, expr)
	addStateBoundNames(local, expr.Bounds)
	return local
}

func addStateBoundNames(scope map[string]bool, bounds []parser.Bound) {
	for _, bound := range bounds {
		for _, name := range bound.Names {
			scope[name] = true
		}
	}
}

func stateArgumentTemporal(expr *parser.Expression, defs map[string]parser.Definition, seen map[string]bool) bool {
	if isStateTemporalHead(expr) {
		return true
	}
	local := defs
	if len(expr.Definitions) > 0 {
		local = maps.Clone(defs)
		for _, def := range expr.Definitions {
			local[def.Name] = def
		}
	}
	if expanded, ok := stateProbeCall(expr, nil, local, seen); ok {
		seen[expr.Text] = true
		defer delete(seen, expr.Text)
		return stateArgumentTemporal(expanded, local, seen)
	}
	found := false
	walkTemporalChildren(expr, func(child *parser.Expression) {
		if !found {
			found = stateArgumentTemporal(child, local, seen)
		}
	})
	return found
}

func isStateTemporalHead(expr *parser.Expression) bool {
	switch expr.Kind {
	case "Always", "Eventually", "WeakFairness", "StrongFairness":
		return true
	case "Apply":
		return expr.Text == "~>" || expr.Text == "-+->"
	}
	return false
}

func stateProbePrimed(expr *parser.Expression, scope map[string]bool, defs map[string]parser.Definition) bool {
	if expr.Kind == "Prime" && stateProbeUsed(expr.Args[0], scope, defs, map[string]bool{}) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = stateProbePrimed(child, scope, defs)
		}
	})
	return found
}
