package checker

import "github.com/upamune/tla-go/syntax/parser"

func (m *model) hasTemporal(expr *parser.Expression, seen map[string]bool) bool {
	if m.isTLCDefer(expr) {
		return false
	}
	if temporalHead(expr) {
		return true
	}
	if expr.Kind == "Let" {
		return m.temporalScope(expr).hasTemporal(expr.Args[0], seen)
	}
	if temporal, ok := m.temporalCallLevel(expr, seen); ok {
		return temporal
	}
	if m.definitionHasTemporal(expr, seen) {
		return true
	}
	return m.childrenHaveTemporal(expr, seen)
}

func temporalHead(expr *parser.Expression) bool {
	if expr.Kind == "Apply" {
		return expr.Text == "~>" || expr.Text == "-+->"
	}
	switch expr.Kind {
	case "Always", "Eventually", "WeakFairness", "StrongFairness":
		return true
	default:
		return false
	}
}

func (m *model) definitionHasTemporal(expr *parser.Expression, seen map[string]bool) bool {
	name := ""
	if expr.Kind == "Identifier" || expr.Kind == "Apply" {
		name = expr.Text
	}
	d, ok := m.defs[name]
	if !ok || seen[name] {
		return false
	}
	seen[name] = true
	defer delete(seen, name)
	return m.hasTemporal(d.Body, seen)
}

func (m *model) childrenHaveTemporal(expr *parser.Expression, seen map[string]bool) bool {
	return expressionChildrenMatch(m, expr, seen, temporalAnalysis)
}
