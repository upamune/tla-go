package checker

import "github.com/upamune/tla-go/syntax/parser"

func (r *instanceResolver) checkConstantChildren(expr *parser.Expression, scope map[string]bool, instance parser.Instance, required map[string]bool, model *model) error {
	var failure error
	visit := func(child *parser.Expression) {
		if failure == nil {
			failure = r.checkConstantCalls(child, scope, instance, required, model)
		}
	}
	walkUnscopedChildren(expr, visit)
	for _, definition := range expr.Definitions {
		if failure == nil {
			failure = r.checkConstantDefinition(definition, scope, instance, required, model)
		}
	}
	return failure
}

func walkUnscopedChildren(expr *parser.Expression, visit func(*parser.Expression)) {
	for _, child := range expr.Args {
		visit(child)
	}
	for _, field := range expr.Fields {
		visit(field.Value)
	}
	for _, arm := range expr.Arms {
		visit(arm.Condition)
		visit(arm.Body)
	}
	walkUpdates(expr.Updates, visit)
}
