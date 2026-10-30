package checker

import "github.com/upamune/tla-go/syntax/parser"

func addInstanceProofFacts(result, module *parser.Module, generated map[string]string) {
	facts := proofFactNames(module)
	for _, definition := range module.Definitions {
		if facts[definition.Name] {
			result.ImportedProofFacts = append(result.ImportedProofFacts, generated[definition.Name])
		}
	}
}

func renameLocalProofFacts(facts []string, replacements map[string]*parser.Expression) []string {
	result := append([]string(nil), facts...)
	for i, name := range result {
		if replacement, ok := replacements[name]; ok {
			result[i] = replacement.Text
		}
	}
	return result
}
