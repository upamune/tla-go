package checker

import "github.com/upamune/tla-go/syntax/parser"

func collectNames(expr *parser.Expression, names map[string]bool) {
	if expr.Kind == "Identifier" || expr.Kind == "Apply" {
		names[expr.Text] = true
	}
	walkChildren(expr, func(child *parser.Expression) { collectNames(child, names) })
}

func captures(expr *parser.Expression, names map[string]bool) bool {
	if boundCapture(expr, names) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if captures(child, names) {
			found = true
		}
	})
	return found
}

func boundCapture(expr *parser.Expression, names map[string]bool) bool {
	return boundsCapture(expr.Bounds, names) || definitionsCapture(expr.Definitions, names) || namedCapture(expr, names) || len(expr.Instances) > 0
}

func boundsCapture(bounds []parser.Bound, names map[string]bool) bool {
	for _, bound := range bounds {
		if anyNames(bound.Names, names) {
			return true
		}
	}
	return false
}

func definitionsCapture(definitions []parser.Definition, names map[string]bool) bool {
	for _, d := range definitions {
		if names[d.Name] || paramsCapture(d.Params, names) {
			return true
		}
	}
	return false
}

func paramsCapture(params []parser.Declaration, names map[string]bool) bool {
	for _, p := range params {
		if names[p.Name] {
			return true
		}
	}
	return false
}

func namedCapture(expr *parser.Expression, names map[string]bool) bool {
	return (expr.Kind == "Lambda" || expr.Kind == "Label" || expr.Kind == "UnboundedQuantifier") && anyNames(expr.Names, names)
}

func anyNames(items []string, names map[string]bool) bool {
	for _, name := range items {
		if names[name] {
			return true
		}
	}
	return false
}

func walkChildren(expr *parser.Expression, visit func(*parser.Expression)) {
	for _, arg := range expr.Args {
		visit(arg)
	}
	walkBounds(expr.Bounds, visit)
	for _, field := range expr.Fields {
		visit(field.Value)
	}
	for _, arm := range expr.Arms {
		visit(arm.Condition)
		visit(arm.Body)
	}
	walkUpdates(expr.Updates, visit)
	walkDefinitions(expr.Definitions, visit)
}

func walkBounds(bounds []parser.Bound, visit func(*parser.Expression)) {
	for _, bound := range bounds {
		if bound.Set != nil {
			visit(bound.Set)
		}
	}
}

func walkDefinitions(definitions []parser.Definition, visit func(*parser.Expression)) {
	for _, d := range definitions {
		visit(d.Body)
		walkBounds(d.Bounds, visit)
	}
}

func walkUpdates(updates []parser.Update, visit func(*parser.Expression)) {
	for _, u := range updates {
		visit(u.Value)
		for _, step := range u.Path {
			for _, arg := range step.Args {
				visit(arg)
			}
		}
	}
}
