package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func (s *structuredProofScope) defineStep(step parser.ProofStep) error {
	for _, def := range step.Definitions {
		if _, exists := s.symbols[def.Name]; exists || s.resolver.newProofConflicts(def.Name) {
			return fmt.Errorf("proof definition %s conflicts with an existing symbol", def.Name)
		}
		def.Location.Line += s.lineOffset
		body, err := s.resolveExpression(def.Body)
		if err != nil {
			return err
		}
		def.Body = body
		if err := s.resolver.definition(def, s.symbols); err != nil {
			return err
		}
		s.symbols[def.Name] = definitionSymbol(def)
		s.definitions[def.Name] = def
		s.localDefs[def.Name] = true
	}
	return s.proof(step.Proof)
}

func (s *structuredProofScope) by(proof *parser.Proof) error {
	for _, fact := range proof.Facts {
		if err := s.proofFact(fact); err != nil {
			return err
		}
	}
	for _, def := range proof.Defs {
		if err := s.proofDefinition(def); err != nil {
			return err
		}
	}
	return nil
}

func (s *structuredProofScope) proofFact(fact *parser.Expression) error {
	if fact.Kind == "ProofStepReference" {
		if !s.facts[fact.Text] {
			return fmt.Errorf("unknown proof step %s", fact.Text)
		}
		return nil
	}
	resolved, err := s.resolveExpression(fact)
	if err != nil {
		return err
	}
	return s.resolver.theoremProofFact(resolved, s.symbols)
}

func (s *structuredProofScope) proofDefinition(def *parser.Expression) error {
	if def.Kind != "Identifier" && def.Kind != "OperatorReference" {
		return fmt.Errorf("DEF entry must name a defined operator")
	}
	if s.localDefs[def.Text] {
		return nil
	}
	if _, local := s.symbols[def.Text]; local {
		return fmt.Errorf("%s is not a proof definition", def.Text)
	}
	if _, ok := s.definitions[def.Text]; ok && s.resolver.declaredBefore(def.Text) {
		return nil
	}
	if _, ok := s.resolver.lookupStandard(def.Text); ok {
		return nil
	}
	return fmt.Errorf("undefined proof definition %s", def.Text)
}
