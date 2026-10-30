package checker

import "github.com/upamune/tla-go/syntax/parser"

func (r *instanceResolver) resolveInstanceSubexpressions(instance parser.Instance, result *parser.Module) (parser.Instance, error) {
	if !instanceHasSubexpressions(instance) {
		return instance, nil
	}
	scope := *r.root
	scope.Definitions = append(append([]parser.Definition(nil), r.root.Definitions...), result.Definitions...)
	resolver := newSubexpressionResolver(&scope)
	instance.Substitutions = append([]parser.Substitution(nil), instance.Substitutions...)
	for i := range instance.Substitutions {
		expr, err := resolver.rewriteRoot(instance.Substitutions[i].Expression, resolver.definitions)
		if err != nil {
			return instance, err
		}
		instance.Substitutions[i].Expression = expr
	}
	return instance, nil
}

func instanceHasSubexpressions(instance parser.Instance) bool {
	for _, substitution := range instance.Substitutions {
		if containsSubexpression(substitution.Expression) {
			return true
		}
	}
	return false
}
