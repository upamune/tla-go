package checker

import (
	"fmt"
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

func addProofModules(module *parser.Module, names ...string) {
	for _, name := range names {
		if !slices.Contains(module.ProofModules, name) {
			module.ProofModules = append(module.ProofModules, name)
		}
	}
}

func (r *resolver) checkProofModule(name string) error {
	if !r.proofModuleVisible(name) {
		return fmt.Errorf("unknown proof module %s", name)
	}
	return nil
}

func (r *resolver) proofModuleVisible(name string) bool {
	if r.proofModule == nil {
		return false
	}
	if r.proofModule.Name == name {
		return true
	}
	for _, module := range r.proofModule.Extends {
		if proofModuleIncludes(module, name) {
			return true
		}
	}
	for _, module := range r.proofModule.ProofModules {
		if proofModuleIncludes(module, name) {
			return true
		}
	}
	return false
}

// These standard modules refer to other bundled modules through EXTENDS or
// LOCAL INSTANCE in the Java TLA+ Tools standard library.
func proofModuleIncludes(module, name string) bool {
	if module == name {
		return true
	}
	switch module {
	case "Integers", "Sequences":
		return name == "Naturals"
	case "FiniteSets":
		return slices.Contains([]string{"Naturals", "Sequences"}, name)
	case "TLC":
		return slices.Contains([]string{"Naturals", "Sequences", "FiniteSets"}, name)
	case "Bags":
		return slices.Contains([]string{"TLC", "Naturals", "Sequences", "FiniteSets"}, name)
	case "Randomization":
		return slices.Contains([]string{"Naturals", "Sequences", "FiniteSets"}, name)
	}
	return false
}
