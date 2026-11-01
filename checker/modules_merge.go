package checker

import "github.com/upamune/tla-go/syntax/parser"

func mergeExtensions(root *parser.Module, units []*parser.Module, standard []string) *parser.Module {
	result := *root
	result.Extends = standard
	result.Constants = nil
	result.Variables = nil
	result.SourceDeclarations = nil
	result.Definitions = nil
	result.Assumptions = nil
	result.ProofDirectives = nil
	result.ImportedProofFacts = nil
	result.ProofModules = append([]string(nil), root.ProofModules...)
	result.InstanceStandardModules = nil
	result.RecursiveDeclarations = nil
	result.Theorems = append([]parser.Theorem(nil), root.Theorems...)
	offset := 0
	for _, unit := range units {
		if unit.Name == root.Name {
			shiftTheoremLocations(result.Theorems, offset)
		}
		mergeExtensionUnit(&result, unit, offset)
		offset += extensionLastLine(unit) + 1
	}
	return &result
}

func shiftTheoremLocations(theorems []parser.Theorem, offset int) {
	for i := range theorems {
		theorems[i].Location.Line += offset
	}
}

func mergeExtensionUnit(result, unit *parser.Module, offset int) {
	addProofModules(result, unit.Name)
	addProofModules(result, unit.ProofModules...)
	addInstanceStandardModules(result, unit.InstanceStandardModules...)
	for _, declaration := range unit.Constants {
		rememberSourceDeclaration(result, unit, declaration)
		declaration.Location.Line += offset
		result.Constants = append(result.Constants, declaration)
	}
	for _, declaration := range unit.Variables {
		rememberSourceDeclaration(result, unit, declaration)
		declaration.Location.Line += offset
		result.Variables = append(result.Variables, declaration)
	}
	for _, definition := range unit.Definitions {
		definition.Location.Line += offset
		result.Definitions = append(result.Definitions, definition)
	}
	for _, assumption := range unit.Assumptions {
		assumption.Location.Line += offset
		result.Assumptions = append(result.Assumptions, assumption)
	}
	for _, directive := range unit.ProofDirectives {
		directive.Location.Line += offset
		result.ProofDirectives = append(result.ProofDirectives, directive)
	}
	result.ImportedProofFacts = append(result.ImportedProofFacts, unit.ImportedProofFacts...)
	for _, theorem := range unit.Theorems {
		if theorem.Name != "" {
			result.ImportedProofFacts = append(result.ImportedProofFacts, theorem.Name)
		}
	}
	result.RecursiveDeclarations = append(result.RecursiveDeclarations, unit.RecursiveDeclarations...)
}

func rememberSourceDeclaration(result, unit *parser.Module, declaration parser.Declaration) {
	if result.SourceDeclarations == nil {
		result.SourceDeclarations = make(map[string]parser.Location)
	}
	if origin, ok := unit.SourceDeclarations[declaration.Name]; ok {
		result.SourceDeclarations[declaration.Name] = origin
		return
	}
	if declaration.Location.Range.Module == "" && len(unit.SourceTokens) == 0 {
		// Preserve unknown provenance even if the merged root has source tokens.
		result.SourceDeclarations[declaration.Name] = parser.Location{}
		return
	}
	result.SourceDeclarations[declaration.Name] = declarationSourceLocation(unit.Name, declaration.Location)
}

func declarationSourceLocation(module string, location parser.Location) parser.Location {
	if location.Range.Module == "" && location.Line != 0 {
		location.Range.Module = module
		location.Range.Start = location.Position
	}
	return location
}

func extensionLastLine(module *parser.Module) int {
	line := module.Location.Range.End.Line
	for _, definition := range module.Definitions {
		line = max(line, definition.Location.Line)
	}
	for _, declaration := range module.Constants {
		line = max(line, declaration.Location.Line)
	}
	for _, declaration := range module.Variables {
		line = max(line, declaration.Location.Line)
	}
	for _, assumption := range module.Assumptions {
		line = max(line, assumption.Location.Line)
	}
	return line
}
