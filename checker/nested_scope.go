package checker

import "github.com/upamune/tla-go/syntax/parser"

// Nested modules may refer to enclosing declarations that precede them.
func nestedScope(module, parent *parser.Module) parser.Module {
	result := *module
	result.Extends = append(append([]string(nil), module.Extends...), parent.Extends...)
	line := module.Location.Range.Start.Line
	result.Constants = append(declarationsBefore(parent.Constants, parent.Name, line), module.Constants...)
	result.Variables = append(declarationsBefore(parent.Variables, parent.Name, line), module.Variables...)
	result.Definitions = append(definitionsBefore(parent.Definitions, parent.Name, line), module.Definitions...)
	return result
}

func declarationsBefore(declarations []parser.Declaration, parent string, line int) []parser.Declaration {
	var result []parser.Declaration
	for _, declaration := range declarations {
		if visibleBeforeNested(declaration.Location, parent, line) {
			result = append(result, declaration)
		}
	}
	return result
}

func definitionsBefore(definitions []parser.Definition, parent string, line int) []parser.Definition {
	var result []parser.Definition
	for _, definition := range definitions {
		if visibleBeforeNested(definition.Location, parent, line) {
			result = append(result, definition)
		}
	}
	return result
}

func visibleBeforeNested(location parser.Location, parent string, line int) bool {
	if location.Range.Module != "" && location.Range.Module != parent {
		return true
	}
	if location.Range.Start.Line > 0 {
		return location.Range.Start.Line < line
	}
	return location.Line < line
}
