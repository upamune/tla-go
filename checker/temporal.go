package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func (m *model) validateActions() error {
	names := append([]string{m.cfg.Next}, m.cfg.ActionConstraints...)
	for _, name := range names {
		expr, _ := m.definition(name)
		if m.hasTemporal(expr, map[string]bool{}) {
			return fmt.Errorf("%s uses an unsupported temporal expression", name)
		}
	}
	return nil
}

func (m *model) hasTemporal(expr *parser.Expression, seen map[string]bool) bool {
	if m.isTLCDefer(expr) {
		return false
	}
	if expr.Kind == "Apply" && expr.Text == "~>" {
		return true
	}
	switch expr.Kind {
	case "Always", "Eventually", "WeakFairness", "StrongFairness":
		return true
	}
	if m.definitionHasTemporal(expr, seen) {
		return true
	}
	return m.childrenHaveTemporal(expr, seen)
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
