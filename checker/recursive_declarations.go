package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

// Check source scopes before EXTENDS and INSTANCE expansion can combine or
// rename their definitions. LET declarations belong only to their own LET.
func validateRecursiveModule(module *parser.Module) error {
	if err := validateRecursiveDeclarations(module.RecursiveDeclarations, module.Definitions); err != nil {
		return fmt.Errorf("module %s: %w", module.Name, err)
	}
	for _, d := range module.Definitions {
		if err := validateRecursiveDefinition(d); err != nil {
			return err
		}
	}
	if err := validateRecursiveInstances(module.Instances); err != nil {
		return err
	}
	if err := validateRecursiveAssertions(module); err != nil {
		return err
	}
	for i := range module.Modules {
		if err := validateRecursiveModule(&module.Modules[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateRecursiveAssertions(module *parser.Module) error {
	for _, a := range module.Assumptions {
		if err := validateRecursiveExpression(a.Expression); err != nil {
			return err
		}
	}
	for _, t := range module.Theorems {
		if err := validateRecursiveTheorem(t); err != nil {
			return err
		}
	}
	for i := range module.ProofDirectives {
		if err := validateRecursiveDirective(&module.ProofDirectives[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateRecursiveTheorem(theorem parser.Theorem) error {
	if err := validateRecursiveExpression(theorem.Statement); err != nil {
		return err
	}
	return validateRecursiveProof(theorem.Proof)
}

func validateRecursiveProof(proof *parser.Proof) error {
	if proof == nil {
		return nil
	}
	if err := validateRecursiveExpressions(proof.Facts, proof.Defs); err != nil {
		return err
	}
	for _, step := range proof.Steps {
		if err := validateRecursiveProofStep(step); err != nil {
			return err
		}
	}
	return nil
}

func validateRecursiveProofStep(step parser.ProofStep) error {
	if err := validateRecursiveExpression(step.Statement); err != nil {
		return err
	}
	for _, bound := range step.Bounds {
		if err := validateRecursiveExpression(bound.Set); err != nil {
			return err
		}
	}
	if err := validateRecursiveExpressions(step.Arguments); err != nil {
		return err
	}
	for _, def := range step.Definitions {
		if err := validateRecursiveDefinition(def); err != nil {
			return err
		}
	}
	if err := validateRecursiveDirective(step.Directive); err != nil {
		return err
	}
	return validateRecursiveProof(step.Proof)
}

func validateRecursiveDirective(directive *parser.ProofDirective) error {
	if directive == nil {
		return nil
	}
	return validateRecursiveExpressions(directive.Facts, directive.Defs)
}

func validateRecursiveExpressions(groups ...[]*parser.Expression) error {
	for _, expressions := range groups {
		for _, expr := range expressions {
			if err := validateRecursiveExpression(expr); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRecursiveDeclarations(decls []parser.Declaration, defs []parser.Definition) error {
	if len(decls) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(decls))
	for _, decl := range decls {
		if seen[decl.Name] {
			return fmt.Errorf("duplicate RECURSIVE declaration %s", decl.Name)
		}
		seen[decl.Name] = true
		definition := recursiveDefinition(decl, defs)
		if definition == nil {
			return fmt.Errorf("symbol %s declared RECURSIVE but not defined", decl.Name)
		}
		if recursiveArityMismatch(decl, definition) {
			return fmt.Errorf("definition of %s has different arity than its RECURSIVE declaration", decl.Name)
		}
	}
	return nil
}

// SANY 1.7.4's Generator.processOperator overwrites paramsMatch for each
// parameter, so only the last parameter's operator arity is checked.
func recursiveArityMismatch(decl parser.Declaration, definition *parser.Definition) bool {
	params := definition.Params
	return len(params) != decl.Arity || len(params) > 0 && params[len(params)-1].Arity != 0
}

func recursiveDefinition(decl parser.Declaration, defs []parser.Definition) *parser.Definition {
	for i := range defs {
		d := &defs[i]
		if d.Name == decl.Name && positionAfter(d.Location.Position, decl.Location.Position) {
			return d
		}
	}
	return nil
}

func positionAfter(a, b parser.Position) bool {
	return a.Line == 0 || b.Line == 0 || a.Line > b.Line || a.Line == b.Line && a.Column > b.Column
}

func validateRecursiveDefinition(d parser.Definition) error {
	if err := validateRecursiveExpression(d.Body); err != nil {
		return err
	}
	for _, b := range d.Bounds {
		if b.Set != nil {
			if err := validateRecursiveExpression(b.Set); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRecursiveExpression(expr *parser.Expression) error {
	if expr == nil {
		return nil
	}
	if err := validateRecursiveDeclarations(expr.RecursiveDeclarations, expr.Definitions); err != nil {
		return err
	}
	if err := validateRecursiveInstances(expr.Instances); err != nil {
		return err
	}
	for _, selector := range expr.Selectors {
		for _, arg := range selector.Args {
			if err := validateRecursiveExpression(arg); err != nil {
				return err
			}
		}
	}
	var failure error
	walkChildren(expr, func(child *parser.Expression) {
		if failure == nil {
			failure = validateRecursiveExpression(child)
		}
	})
	return failure
}

func validateRecursiveInstances(instances []parser.Instance) error {
	for _, instance := range instances {
		for _, substitution := range instance.Substitutions {
			if err := validateRecursiveExpression(substitution.Expression); err != nil {
				return err
			}
		}
	}
	return nil
}
