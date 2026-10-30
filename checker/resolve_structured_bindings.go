package checker

import (
	"maps"

	"github.com/upamune/tla-go/syntax/parser"
)

func (s *structuredProofScope) bindingStep(step parser.ProofStep) error {
	local, err := s.proofBindings(step.Bounds)
	if err != nil {
		return err
	}
	if step.Statement != nil {
		child := s.clone()
		child.symbols = local
		if _, err := child.statementScope(step.Statement); err != nil {
			return err
		}
	}
	if err := s.proof(step.Proof); err != nil {
		return err
	}
	s.symbols = local
	return nil
}

func (s *structuredProofScope) proofBindings(bounds []parser.Bound) (map[string]symbol, error) {
	local := maps.Clone(s.symbols)
	for _, bound := range bounds {
		if bound.Set != nil {
			if _, err := s.statementScope(bound.Set); err != nil {
				return nil, err
			}
		}
		bound.Set = nil
		if err := s.resolver.bound(bound, local); err != nil {
			return nil, err
		}
	}
	return local, nil
}

func (s *structuredProofScope) witnessStep(step parser.ProofStep) error {
	for _, argument := range step.Arguments {
		if _, err := s.statementScope(argument); err != nil {
			return err
		}
	}
	return nil
}
