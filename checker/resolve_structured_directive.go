package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func (s *structuredProofScope) directive(directive *parser.ProofDirective) error {
	if directive == nil {
		return fmt.Errorf("missing proof directive")
	}
	for _, fact := range directive.Facts {
		if directive.Kind == "HIDE" && !s.hideableFact(fact) {
			return fmt.Errorf("HIDE requires a named theorem, assumption, or proof step")
		}
		if err := s.proofFact(fact); err != nil {
			return err
		}
	}
	for _, definition := range directive.Defs {
		if err := s.proofDefinition(definition); err != nil {
			return err
		}
	}
	return nil
}

func (s *structuredProofScope) hideableFact(fact *parser.Expression) bool {
	if fact.Kind == "ModuleFact" {
		return true
	}
	if fact.Kind == "ProofStepReference" {
		return s.facts[fact.Text]
	}
	if fact.Kind != "Identifier" || !s.namedFacts[fact.Text] {
		return false
	}
	_, shadowed := s.symbols[fact.Text]
	return !shadowed
}
