package checker

import "github.com/upamune/tla-go/syntax/parser"

func instanceDefinitionLevelBounded(module *parser.Module, def parser.Definition, name string, level int, stateVariable bool, defs map[string]parser.Definition) bool {
	if !instanceDefinitionDependsOnVariable(def, name, defs) {
		return false
	}
	scope := newMetadataScope(module)
	scope.instanceProbe = true
	probe := level + 1
	if stateVariable {
		// A state-variable substitution is constrained only if the export
		// actually imposes a level limit on that variable.
		probe = 4
	}
	scope.values[name] = probe
	_, err := scope.definition(scope.defs[def.Name], nil, map[*metadataDefinition]bool{})
	// Unknown metadata forms keep the conservative existing behavior.
	return err != nil
}
