package checker

import "github.com/upamune/tla-go/syntax/parser"

func (m *model) propertyUsesUnstableTLC(property temporalProperty) bool {
	if m.expressionUsesUnstableTLC(property.predicate, map[string]bool{}) || m.expressionUsesUnstableTLC(property.trigger, map[string]bool{}) {
		return true
	}
	for _, expr := range property.sequence {
		if m.expressionUsesUnstableTLC(expr, map[string]bool{}) {
			return true
		}
	}
	return false
}

func (m *model) expressionUsesUnstableTLC(expr *parser.Expression, seen map[string]bool) bool {
	if expr == nil {
		return false
	}
	if expr.Kind == "Apply" && expr.Text == "TLCGet" && !stableTLCRead(expr) {
		return true
	}
	if m.definitionUsesUnstableTLC(expr, seen) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = m.expressionUsesUnstableTLC(child, seen)
		}
	})
	return found
}

func stableTLCRead(expr *parser.Expression) bool {
	if len(expr.Args) != 1 {
		return false
	}
	arg := expr.Args[0]
	return arg.Kind == "Number" || arg.Kind == "String" && arg.Text != "duration"
}

func (m *model) definitionUsesUnstableTLC(expr *parser.Expression, seen map[string]bool) bool {
	if expr.Kind != "Identifier" && expr.Kind != "Apply" {
		return false
	}
	d, ok := m.defs[expr.Text]
	if !ok || seen[expr.Text] {
		return false
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return m.expressionUsesUnstableTLC(d.Body, seen)
}
