package checker

import (
	"fmt"
	"maps"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

type structuredProofScope struct {
	resolver    *resolver
	selector    *subexpressionResolver
	symbols     map[string]symbol
	definitions map[string]parser.Definition
	localDefs   map[string]bool
	facts       map[string]bool
	namedFacts  map[string]bool
	lineOffset  int
}

func (r *resolver) validateStructuredTheorem(module *parser.Module, theorem parser.Theorem) error {
	proofResolver := *r
	proofResolver.proofScope = true
	proofResolver.position = theorem.Location.Position
	scope := &structuredProofScope{
		resolver: &proofResolver,
		selector: newSubexpressionResolver(module),
		symbols:  map[string]symbol{}, definitions: map[string]parser.Definition{}, localDefs: map[string]bool{}, facts: map[string]bool{},
		lineOffset: theorem.Location.Line - theorem.Location.Range.Start.Line,
		namedFacts: proofFactNames(module),
	}
	for _, d := range module.Definitions {
		scope.definitions[d.Name] = d
	}
	if theorem.Statement.Kind == "AssumeProve" {
		var err error
		scope.symbols, err = proofResolver.assumeProveScope(theorem.Statement, nil)
		if err != nil {
			return err
		}
	}
	return scope.structured(theorem.Proof)
}

func (s *structuredProofScope) clone() *structuredProofScope {
	return &structuredProofScope{resolver: s.resolver, selector: s.selector, symbols: maps.Clone(s.symbols), definitions: maps.Clone(s.definitions), localDefs: maps.Clone(s.localDefs), facts: maps.Clone(s.facts), namedFacts: s.namedFacts, lineOffset: s.lineOffset}
}

func (s *structuredProofScope) structured(proof *parser.Proof) error {
	local := s.clone()
	for _, step := range proof.Steps {
		if namedProofStep(step.Label) {
			local.facts[step.Label] = proofStepIsFact(step.Kind)
		}
		if err := local.step(step); err != nil {
			return err
		}
	}
	return nil
}

func proofStepIsFact(kind string) bool {
	return kind != "Define" && kind != "Use" && kind != "Hide"
}

func namedProofStep(label string) bool {
	_, name, ok := strings.Cut(label, ">")
	return ok && name != ""
}

func (s *structuredProofScope) step(step parser.ProofStep) error {
	switch step.Kind {
	case "Define":
		return s.defineStep(step)
	case "Use", "Hide":
		return s.directive(step.Directive)
	default:
		return s.assertionStep(step)
	}
}

func (s *structuredProofScope) assertionStep(step parser.ProofStep) error {
	child := s.clone()
	if step.Statement != nil {
		var err error
		child.symbols, err = s.statementScope(step.Statement)
		if err != nil {
			return err
		}
	}
	if step.Kind == "Suffices" {
		child.symbols = maps.Clone(s.symbols)
	}
	if err := child.proof(step.Proof); err != nil {
		return err
	}
	if step.Kind == "Suffices" && step.Statement.Kind == "AssumeProve" {
		var err error
		s.symbols, err = s.statementScope(step.Statement)
		return err
	}
	return nil
}

func (s *structuredProofScope) statementScope(statement *parser.Expression) (map[string]symbol, error) {
	resolved, err := s.resolveExpression(statement)
	if err != nil {
		return nil, err
	}
	if resolved.Kind == "AssumeProve" {
		return s.resolver.assumeProveScope(resolved, s.symbols)
	}
	if err := s.resolver.expression(resolved, s.symbols); err != nil {
		return nil, err
	}
	return maps.Clone(s.symbols), nil
}

func (s *structuredProofScope) resolveExpression(expr *parser.Expression) (*parser.Expression, error) {
	if !containsSubexpression(expr) {
		return expr, nil
	}
	return s.selector.rewriteRoot(expr, s.selectionDefinitions())
}

func (s *structuredProofScope) selectionDefinitions() map[string]parser.Definition {
	visible := make(map[string]parser.Definition, len(s.definitions))
	for name, definition := range s.definitions {
		if s.localDefs[name] {
			visible[name] = definition
			continue
		}
		if _, shadowed := s.symbols[name]; !shadowed && s.resolver.declaredBefore(name) {
			visible[name] = definition
		}
	}
	return visible
}

func (s *structuredProofScope) proof(proof *parser.Proof) error {
	if proof == nil {
		return nil
	}
	switch proof.Kind {
	case "Structured":
		return s.structured(proof)
	case "By":
		return s.by(proof)
	case "Obvious", "Omitted":
		return nil
	default:
		return fmt.Errorf("unsupported proof kind %s", proof.Kind)
	}
}
