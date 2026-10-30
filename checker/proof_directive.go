package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func (r *resolver) validateProofDirectives(module *parser.Module) error {
	if len(module.ProofDirectives) == 0 {
		return nil
	}
	definitions := make(map[string]bool, len(module.Definitions))
	for _, definition := range module.Definitions {
		definitions[definition.Name] = true
	}
	facts := proofFactNames(module)
	for _, directive := range module.ProofDirectives {
		r.position = directive.Location.Position
		if err := r.validateDirective(directive, definitions, facts); err != nil {
			return &SemanticError{Err: err}
		}
	}
	return nil
}

func proofFactNames(module *parser.Module) map[string]bool {
	names := map[string]bool{}
	for _, assumption := range module.Assumptions {
		names[assumption.Name] = assumption.Name != ""
	}
	for _, theorem := range module.Theorems {
		names[theorem.Name] = theorem.Name != ""
	}
	for _, name := range module.ImportedProofFacts {
		names[name] = true
	}
	return names
}

func renameLocalProofDirectives(directives []parser.ProofDirective, replacements map[string]*parser.Expression) []parser.ProofDirective {
	result := append([]parser.ProofDirective(nil), directives...)
	for i := range result {
		result[i].Facts = substituteProofEntries(result[i].Facts, replacements)
		result[i].Defs = substituteProofEntries(result[i].Defs, replacements)
	}
	return result
}

func substituteProofEntries(entries []*parser.Expression, replacements map[string]*parser.Expression) []*parser.Expression {
	result := make([]*parser.Expression, len(entries))
	for i, entry := range entries {
		result[i] = substitute(entry, replacements)
	}
	return result
}

func (r *resolver) validateDirective(directive parser.ProofDirective, definitions, facts map[string]bool) error {
	for _, fact := range directive.Facts {
		if err := r.validateDirectiveFact(directive.Kind, fact, facts); err != nil {
			return err
		}
	}
	for _, definition := range directive.Defs {
		if err := r.validateDirectiveDefinition(definition, definitions); err != nil {
			return err
		}
	}
	return nil
}

func (r *resolver) validateDirectiveFact(kind string, fact *parser.Expression, facts map[string]bool) error {
	if kind == "HIDE" && (fact.Kind != "Identifier" || !facts[fact.Text]) {
		return fmt.Errorf("HIDE requires a named theorem or assumption")
	}
	proofResolver := *r
	proofResolver.proofScope = true
	return proofResolver.expression(fact, nil)
}

func (r *resolver) proofNameInScope(name string, scope map[string]symbol) bool {
	if _, local := scope[name]; local {
		return true
	}
	if _, global := r.globals[name]; global {
		return r.declaredBefore(name)
	}
	return true
}

func (r *resolver) validateDirectiveDefinition(expr *parser.Expression, definitions map[string]bool) error {
	if expr.Kind != "Identifier" && expr.Kind != "OperatorReference" {
		return fmt.Errorf("DEF entry must name a defined operator")
	}
	if definitions[expr.Text] && r.declaredBefore(expr.Text) {
		return nil
	}
	if _, ok := r.lookupStandard(expr.Text); ok {
		return nil
	}
	return fmt.Errorf("undefined definition %s", expr.Text)
}
