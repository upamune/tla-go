package checker

import "github.com/upamune/tla-go/syntax/parser"

func substitute(expr *parser.Expression, replacements map[string]*parser.Expression) *parser.Expression {
	if expr == nil {
		return nil
	}
	if expr.Kind == "Apply" {
		if replacement, ok := replacements[expr.Text]; ok && isOperatorReplacement(replacement) {
			return substituteOperatorCall(expr, replacement, replacements)
		}
	}
	if expr.Kind == "Identifier" || expr.Kind == "OperatorReference" {
		if replacement, ok := replacements[expr.Text]; ok {
			return replacement
		}
	}
	result := *expr
	result.EnabledScopes = append([]string(nil), expr.EnabledScopes...)
	result.EnabledBindings = append([]parser.EnabledBinding(nil), expr.EnabledBindings...)
	result.Args = subExpressions(expr.Args, replacements)
	result.Bounds = subBounds(expr.Bounds, replacements)
	result.Fields = subFields(expr.Fields, replacements)
	result.Arms = subArms(expr.Arms, replacements)
	result.Updates = subUpdates(expr.Updates, replacements)
	result.Definitions = subDefinitions(expr.Definitions, replacements)
	result.Selectors = subSelectors(expr.Selectors, replacements)
	return &result
}

func subSelectors(items []parser.Selector, replacements map[string]*parser.Expression) []parser.Selector {
	if items == nil {
		return nil
	}
	out := append([]parser.Selector(nil), items...)
	for i := range out {
		out[i].Args = subExpressions(out[i].Args, replacements)
	}
	return out
}

func subExpressions(items []*parser.Expression, replacements map[string]*parser.Expression) []*parser.Expression {
	if items == nil {
		return nil
	}
	out := make([]*parser.Expression, len(items))
	for i, item := range items {
		out[i] = substitute(item, replacements)
	}
	return out
}

func subBounds(items []parser.Bound, replacements map[string]*parser.Expression) []parser.Bound {
	if items == nil {
		return nil
	}
	out := append([]parser.Bound(nil), items...)
	for i := range out {
		if out[i].Set != nil {
			out[i].Set = substitute(out[i].Set, replacements)
		}
	}
	return out
}

func subFields(items []parser.Field, replacements map[string]*parser.Expression) []parser.Field {
	if items == nil {
		return nil
	}
	out := append([]parser.Field(nil), items...)
	for i := range out {
		out[i].Value = substitute(out[i].Value, replacements)
	}
	return out
}

func subArms(items []parser.CaseArm, replacements map[string]*parser.Expression) []parser.CaseArm {
	if items == nil {
		return nil
	}
	out := append([]parser.CaseArm(nil), items...)
	for i := range out {
		out[i].Condition = substitute(out[i].Condition, replacements)
		out[i].Body = substitute(out[i].Body, replacements)
	}
	return out
}

func subUpdates(items []parser.Update, replacements map[string]*parser.Expression) []parser.Update {
	if items == nil {
		return nil
	}
	out := append([]parser.Update(nil), items...)
	for i := range out {
		out[i].Value = substitute(out[i].Value, replacements)
		out[i].Path = append([]parser.PathStep(nil), out[i].Path...)
		for j := range out[i].Path {
			out[i].Path[j].Args = subExpressions(out[i].Path[j].Args, replacements)
		}
	}
	return out
}

func subDefinitions(items []parser.Definition, replacements map[string]*parser.Expression) []parser.Definition {
	if items == nil {
		return nil
	}
	out := append([]parser.Definition(nil), items...)
	for i := range out {
		out[i].Body = substitute(out[i].Body, replacements)
		out[i].Bounds = subBounds(out[i].Bounds, replacements)
	}
	return out
}

func isOperatorReplacement(expr *parser.Expression) bool {
	return expr.Kind == "Identifier" || expr.Kind == "OperatorReference" || expr.Kind == "Lambda"
}

func substituteOperatorCall(expr, operator *parser.Expression, replacements map[string]*parser.Expression) *parser.Expression {
	args := subExpressions(expr.Args, replacements)
	if operator.Kind == "Lambda" {
		bindings := make(map[string]*parser.Expression, len(args))
		for i, name := range operator.Names {
			bindings[name] = args[i]
		}
		return substitute(operator.Args[0], bindings)
	}
	result := *expr
	result.Text = operator.Text
	result.Args = args
	if kind, ok := parser.SpecialOperatorKind(operator.Text); ok {
		result.Kind = kind
		result.Text = ""
	}
	return &result
}
