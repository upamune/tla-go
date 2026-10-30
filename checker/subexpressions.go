package checker

import (
	"fmt"
	"maps"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

type subexpressionResolver struct {
	definitions map[string]parser.Definition
	next        int
}

func resolveSubexpressions(module *parser.Module) (*parser.Module, error) {
	if !moduleHasSubexpressions(module) {
		return module, nil
	}
	r := newSubexpressionResolver(module)
	return r.rewriteModule(module)
}

func (r *extensionResolver) resolveExtensionSubexpressions(module *parser.Module) (*parser.Module, error) {
	if !moduleHasSubexpressions(module) {
		return module, nil
	}
	units := r.scopeUnits(module)
	scope := mergeExtensions(module, units, scopeStandards(units))
	resolver := newSubexpressionResolver(scope)
	return resolver.rewriteModule(module)
}

func newSubexpressionResolver(module *parser.Module) *subexpressionResolver {
	r := subexpressionResolver{definitions: make(map[string]parser.Definition, len(module.Definitions))}
	for _, definition := range module.Definitions {
		r.definitions[definition.Name] = definition
	}
	for _, theorem := range module.Theorems {
		if theorem.Name != "" {
			r.definitions[theorem.Name] = parser.Definition{Kind: "Operator", Name: theorem.Name, Body: theorem.Statement, Location: theorem.Location}
		}
	}
	return &r
}

func (r *subexpressionResolver) rewriteModule(module *parser.Module) (*parser.Module, error) {
	result := *module
	result.Definitions = append([]parser.Definition(nil), module.Definitions...)
	result.Assumptions = append([]parser.Assumption(nil), module.Assumptions...)
	result.Theorems = append([]parser.Theorem(nil), module.Theorems...)
	for i := range result.Definitions {
		body, err := r.rewriteRoot(result.Definitions[i].Body, r.definitions)
		if err != nil {
			return nil, err
		}
		result.Definitions[i].Body = body
		bounds, err := r.rewriteBounds(result.Definitions[i].Bounds, r.definitions)
		if err != nil {
			return nil, err
		}
		result.Definitions[i].Bounds = bounds
	}
	for i := range result.Assumptions {
		body, err := r.rewriteRoot(result.Assumptions[i].Expression, r.definitions)
		if err != nil {
			return nil, err
		}
		result.Assumptions[i].Expression = body
	}
	if err := r.rewriteTheoremStatements(result.Theorems); err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *subexpressionResolver) rewriteTheoremStatements(theorems []parser.Theorem) error {
	for i := range theorems {
		body, err := r.rewriteRoot(theorems[i].Statement, r.definitions)
		if err != nil {
			return err
		}
		theorems[i].Statement = body
	}
	return nil
}

func moduleHasSubexpressions(module *parser.Module) bool {
	for _, definition := range module.Definitions {
		if containsSubexpression(definition.Body) {
			return true
		}
		if boundsContainSubexpressions(definition.Bounds) {
			return true
		}
	}
	for _, assumption := range module.Assumptions {
		if containsSubexpression(assumption.Expression) {
			return true
		}
	}
	return theoremsHaveSubexpressions(module.Theorems)
}

func theoremsHaveSubexpressions(theorems []parser.Theorem) bool {
	for _, theorem := range theorems {
		if containsSubexpression(theorem.Statement) {
			return true
		}
	}
	return false
}

func boundsContainSubexpressions(bounds []parser.Bound) bool {
	for _, bound := range bounds {
		if containsSubexpression(bound.Set) {
			return true
		}
	}
	return false
}

func (r *subexpressionResolver) rewriteBounds(bounds []parser.Bound, scope map[string]parser.Definition) ([]parser.Bound, error) {
	result := append([]parser.Bound(nil), bounds...)
	for i := range result {
		set, err := r.rewriteRoot(result[i].Set, scope)
		if err != nil {
			return nil, err
		}
		result[i].Set = set
	}
	return result, nil
}

func (r *subexpressionResolver) rewriteRoot(expr *parser.Expression, scope map[string]parser.Definition) (*parser.Expression, error) {
	if !containsSubexpression(expr) {
		return expr, nil
	}
	return r.rewrite(substitute(expr, nil), scope)
}

func containsSubexpression(expr *parser.Expression) bool {
	if expr == nil {
		return false
	}
	if expr.Kind == "Subexpression" || ((expr.Kind == "Apply" || expr.Kind == "Identifier") && strings.Contains(expr.Text, "!")) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = containsSubexpression(child)
		}
	})
	return found
}

func (r *subexpressionResolver) rewrite(expr *parser.Expression, scope map[string]parser.Definition) (*parser.Expression, error) {
	if expr == nil {
		return nil, nil
	}
	if expr.Kind == "Let" {
		scope = maps.Clone(scope)
		for _, definition := range expr.Definitions {
			scope[definition.Name] = definition
		}
	}
	if err := r.rewriteChildren(expr, scope); err != nil {
		return nil, err
	}
	selectors, pool, selected := r.reference(expr, scope)
	if !selected {
		return expr, nil
	}
	body, err := r.selectExpression(selectors, pool, scope, expr.Location)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (r *subexpressionResolver) rewriteChildren(expr *parser.Expression, scope map[string]parser.Definition) error {
	var failure error
	walkChildren(expr, func(child *parser.Expression) {
		if failure != nil || child == nil {
			return
		}
		var resolved *parser.Expression
		resolved, failure = r.rewrite(child, scope)
		if failure == nil {
			*child = *resolved
		}
	})
	if failure != nil {
		return failure
	}
	for i := range expr.Selectors {
		for j, arg := range expr.Selectors[i].Args {
			resolved, err := r.rewrite(arg, scope)
			if err != nil {
				return err
			}
			expr.Selectors[i].Args[j] = resolved
		}
	}
	return nil
}

func (r *subexpressionResolver) reference(expr *parser.Expression, scope map[string]parser.Definition) ([]parser.Selector, []*parser.Expression, bool) {
	if expr.Kind == "Subexpression" {
		return expr.Selectors, nil, true
	}
	if expr.Kind != "Identifier" && expr.Kind != "Apply" {
		return nil, nil, false
	}
	if _, exists := scope[expr.Text]; exists || !strings.Contains(expr.Text, "!") {
		return nil, nil, false
	}
	return qualifiedSubexpression(expr, scope)
}

func qualifiedSubexpression(expr *parser.Expression, scope map[string]parser.Definition) ([]parser.Selector, []*parser.Expression, bool) {
	parts := strings.Split(expr.Text, "!")
	for i := range parts {
		prefix := strings.Join(parts[:i+1], "!")
		if _, ok := scope[prefix]; !ok {
			continue
		}
		selectors := []parser.Selector{{Kind: "Name", Name: prefix}}
		for _, name := range parts[i+1:] {
			selectors = append(selectors, parser.Selector{Kind: "Name", Name: name})
		}
		return selectors, expr.Args, true
	}
	return nil, nil, false
}

func selectionError(location parser.Location, message string, args ...any) error {
	return fmt.Errorf("%s at line %d, column %d", fmt.Sprintf(message, args...), location.Line, location.Column)
}
