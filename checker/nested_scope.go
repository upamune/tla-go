package checker

import "github.com/upamune/tla-go/syntax/parser"

// Nested modules may refer to enclosing declarations that precede them.
func nestedScope(module, parent *parser.Module) parser.Module {
	result := *module
	result.SourceDeclarations = nil
	result.Extends = append(append([]string(nil), module.Extends...), parent.Extends...)
	result.ProofModules = append([]string(nil), module.ProofModules...)
	addProofModules(&result, parent.ProofModules...)
	for _, instance := range parent.Instances {
		addProofModules(&result, instance.Module)
	}
	start := module.Location.Range.Start
	result.Constants = append(declarationsBefore(parent.Constants, parent.Name, start), module.Constants...)
	result.Variables = append(declarationsBefore(parent.Variables, parent.Name, start), module.Variables...)
	for _, declaration := range result.Constants {
		rememberNestedSource(&result, module, parent, declaration)
	}
	for _, declaration := range result.Variables {
		rememberNestedSource(&result, module, parent, declaration)
	}
	result.Definitions = append(definitionsBefore(parent.Definitions, parent.Name, start), module.Definitions...)
	return result
}

func rememberNestedSource(result, module, parent *parser.Module, declaration parser.Declaration) {
	if _, ok := module.SourceDeclarations[declaration.Name]; ok {
		rememberSourceDeclaration(result, module, declaration)
		return
	}
	if _, ok := parent.SourceDeclarations[declaration.Name]; ok {
		rememberSourceDeclaration(result, parent, declaration)
		return
	}
	owner := module
	if declaration.Location.Range.Module == parent.Name || declaration.Location.Range.Module == "" && nestedPositionBefore(declaration.Location.Position, module.Location.Range.Start) {
		owner = parent
	}
	rememberSourceDeclaration(result, owner, declaration)
}

func declarationsBefore(declarations []parser.Declaration, parent string, start parser.Position) []parser.Declaration {
	var result []parser.Declaration
	for _, declaration := range declarations {
		if visibleBeforeNested(declaration.Location, parent, start) {
			result = append(result, declaration)
		}
	}
	return result
}

func definitionsBefore(definitions []parser.Definition, parent string, start parser.Position) []parser.Definition {
	var result []parser.Definition
	for _, definition := range definitions {
		if visibleBeforeNested(definition.Location, parent, start) {
			result = append(result, definition)
		}
	}
	return result
}

func visibleBeforeNested(location parser.Location, parent string, start parser.Position) bool {
	if location.Range.Module != "" && location.Range.Module != parent {
		return true
	}
	position := location.Range.Start
	if position.Line == 0 {
		position = location.Position
	}
	return nestedPositionBefore(position, start)
}

// Fall back to line-only ordering for ASTs without complete source positions.
func nestedPositionBefore(position, start parser.Position) bool {
	if position.Line != start.Line {
		return position.Line < start.Line
	}
	return position.Line != 0 && position.Column != 0 && start.Column != 0 && position.Column < start.Column
}
