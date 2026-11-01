package checker

import (
	"fmt"
	"maps"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

// isolateExtensionBindings preserves each extended module's operator bindings
// when independent EXTENDS paths export the same name. It fails closed for
// syntax whose lexical scope is not yet handled by the renaming pass.
func isolateExtensionBindings(units []*parser.Module) []*parser.Module {
	if !hasExtensionBindingCollisions(units) || !canIsolateExtensionBindings(units) {
		return units
	}
	occupied := make(map[string]bool)
	exports := make(map[string]map[string]string)
	out := make([]*parser.Module, 0, len(units))
	for _, unit := range units {
		names := extensionUnitBindings(unit, exports, occupied)
		renamed, ok := renameExtensionUnit(unit, names)
		if !ok {
			return units
		}
		out = append(out, renamed)
		exports[unit.Name] = names
	}
	return out
}

func extensionUnitBindings(unit *parser.Module, exports map[string]map[string]string, occupied map[string]bool) map[string]string {
	scope := inheritedExtensionBindings(unit, exports)
	names := maps.Clone(scope)
	own := make(map[string]bool)
	for _, d := range unit.Definitions {
		if strings.HasPrefix(d.Name, "!") || own[d.Name] {
			continue
		}
		own[d.Name] = true
		if _, inherited := scope[d.Name]; inherited {
			continue
		}
		name := d.Name
		if occupied[name] {
			name = fmt.Sprintf("!extension!%s!%s", unit.Name, name)
		}
		names[d.Name] = name
		occupied[name] = true
	}
	return names
}

func hasExtensionBindingCollisions(units []*parser.Module) bool {
	seen := make(map[string]string)
	for _, unit := range units {
		for _, d := range unit.Definitions {
			if strings.HasPrefix(d.Name, "!") {
				continue
			}
			if previous := seen[d.Name]; previous != "" && previous != unit.Name {
				return true
			}
			seen[d.Name] = unit.Name
		}
	}
	return false
}

func inheritedExtensionBindings(unit *parser.Module, exports map[string]map[string]string) map[string]string {
	result := make(map[string]string)
	for _, name := range unit.Extends {
		for original, renamed := range exports[name] {
			if _, exists := result[original]; !exists {
				result[original] = renamed
			}
		}
	}
	return result
}

func canIsolateExtensionBindings(units []*parser.Module) bool {
	defined := make(map[string]bool)
	for _, unit := range units {
		for _, d := range unit.Definitions {
			defined[d.Name] = true
		}
	}
	for _, unit := range units {
		if unsupportedExtensionUnit(unit) || collidesWithDeclaration(unit, defined) {
			return false
		}
	}
	return true
}

func unsupportedExtensionUnit(unit *parser.Module) bool {
	// Proof references and nested module bodies need their own lexical scopes.
	if len(unit.Theorems) != 0 || len(unit.ProofDirectives) != 0 || len(unit.Modules) != 0 {
		return true
	}
	for _, a := range unit.Assumptions {
		if a.Name != "" {
			return true
		}
	}
	return false
}

func collidesWithDeclaration(unit *parser.Module, defined map[string]bool) bool {
	for _, d := range unit.Constants {
		if defined[d.Name] {
			return true
		}
	}
	for _, d := range unit.Variables {
		if defined[d.Name] {
			return true
		}
	}
	return false
}

func renameExtensionUnit(unit *parser.Module, names map[string]string) (*parser.Module, bool) {
	if !hasRenamedExtensionBindings(names) {
		return unit, true
	}
	if !safeExtensionUnit(unit) {
		return nil, false
	}
	result := *unit
	result.Definitions = make([]parser.Definition, len(unit.Definitions))
	for i, d := range unit.Definitions {
		result.Definitions[i] = renameExtensionDefinition(d, names)
	}
	result.RecursiveDeclarations = renameExtensionDeclarations(unit.RecursiveDeclarations, names)
	result.Assumptions = renameExtensionAssumptions(unit.Assumptions, names)
	result.Instances = renameExtensionInstances(unit.Instances, names)
	return &result, true
}

func hasRenamedExtensionBindings(names map[string]string) bool {
	for old, name := range names {
		if old != name {
			return true
		}
	}
	return false
}

func renameExtensionDefinition(d parser.Definition, names map[string]string) parser.Definition {
	scope := maps.Clone(names)
	for _, p := range d.Params {
		delete(scope, p.Name)
	}
	renamer := alphaRenamer{}
	d.Body = renamer.expression(d.Body, scope)
	if name, ok := names[d.Name]; ok {
		d.Local = d.Local || name != d.Name
		d.Name = name
	}
	return d
}

func renameExtensionDeclarations(input []parser.Declaration, names map[string]string) []parser.Declaration {
	out := append([]parser.Declaration(nil), input...)
	for i := range out {
		if name, ok := names[out[i].Name]; ok {
			out[i].Name = name
		}
	}
	return out
}

func renameExtensionAssumptions(input []parser.Assumption, names map[string]string) []parser.Assumption {
	out := append([]parser.Assumption(nil), input...)
	renamer := alphaRenamer{}
	for i := range out {
		out[i].Expression = renamer.expression(out[i].Expression, names)
	}
	return out
}

func renameExtensionInstances(input []parser.Instance, names map[string]string) []parser.Instance {
	out := append([]parser.Instance(nil), input...)
	renamer := alphaRenamer{}
	for i := range out {
		substitutions := append([]parser.Substitution(nil), out[i].Substitutions...)
		for j := range substitutions {
			substitutions[j].Expression = renamer.expression(substitutions[j].Expression, names)
		}
		out[i].Substitutions = substitutions
	}
	return out
}
