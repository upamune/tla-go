package checker

import "github.com/upamune/tla-go/syntax/parser"

// TLC 1.7.4 leaves some named runtime counters undefined. The Go runtime
// currently exposes them, so counting a model that reads one is unreliable.
// Numeric registers are a separate supported operation.
func (m *model) coverageConstraintUsesRuntimeRead() bool {
	for _, name := range m.cfg.Constraints {
		if m.coverageRuntimeRead(m.defs[name].Body, map[string]bool{}) {
			return true
		}
	}
	return false
}

func (m *model) coverageRuntimeRead(expr *parser.Expression, seen map[string]bool) bool {
	if expr == nil {
		return false
	}
	if tlcOperatorName(expr) == "TLCGet" && (expr.Kind != "Apply" || len(expr.Args) != 1 || expr.Args[0].Kind != "Number") {
		return true
	}
	if m.coverageDefinitionRuntimeRead(expr, seen) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = m.coverageRuntimeRead(child, seen)
		}
	})
	return found
}

func (m *model) coverageDefinitionRuntimeRead(expr *parser.Expression, seen map[string]bool) bool {
	if expr.Kind != "Identifier" && expr.Kind != "Apply" {
		return false
	}
	d, ok := m.defs[expr.Text]
	if !ok || seen[expr.Text] {
		return false
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return m.coverageRuntimeRead(d.Body, seen)
}
