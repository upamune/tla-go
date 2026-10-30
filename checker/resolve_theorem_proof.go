package checker

import "github.com/upamune/tla-go/syntax/parser"

func (r *resolver) validateTheoremProofs(module *parser.Module) error {
	if len(module.Theorems) == 0 {
		return nil
	}
	definitions := make(map[string]bool, len(module.Definitions))
	for _, def := range module.Definitions {
		definitions[def.Name] = true
	}
	for _, theorem := range module.Theorems {
		if theorem.Proof == nil || theorem.Proof.Kind != "By" {
			continue
		}
		if err := r.validateTheoremBy(theorem, definitions); err != nil {
			return &SemanticError{Err: err}
		}
	}
	return nil
}

func (r *resolver) validateTheoremBy(theorem parser.Theorem, definitions map[string]bool) error {
	proofResolver := *r
	proofResolver.proofScope = true
	proofResolver.position = theorem.Location.Position
	var scope map[string]symbol
	if theorem.Statement.Kind == "AssumeProve" {
		var err error
		scope, err = proofResolver.assumeProveScope(theorem.Statement, nil)
		if err != nil {
			return err
		}
	}
	for _, fact := range theorem.Proof.Facts {
		if err := proofResolver.theoremProofFact(fact, scope); err != nil {
			return err
		}
	}
	for _, def := range theorem.Proof.Defs {
		if err := proofResolver.validateDirectiveDefinition(def, definitions); err != nil {
			return err
		}
	}
	return nil
}

func (r *resolver) theoremProofFact(fact *parser.Expression, scope map[string]symbol) error {
	if fact.Kind == "Identifier" && r.assumeProve[fact.Text] {
		if _, shadowed := scope[fact.Text]; !shadowed {
			return r.checkName(fact.Text, 0, scope)
		}
	}
	return r.expression(fact, scope)
}
