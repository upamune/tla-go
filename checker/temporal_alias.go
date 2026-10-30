package checker

import "github.com/upamune/tla-go/syntax/parser"

// temporalAlias expands a chain of temporal operator calls before recognizing a
// compound form. Existing substitution checks prevent capture.
func (m *model) temporalAlias(expr *parser.Expression) (*parser.Expression, bool) {
	seen := map[string]bool{}
	for expr.Kind == "Identifier" || expr.Kind == "Apply" {
		definition, ok := m.defs[expr.Text]
		if !ok || !m.hasTemporal(expr, map[string]bool{}) {
			break
		}
		expanded, ok := expandTemporalAlias(definition, expr, seen)
		if !ok {
			return nil, false
		}
		expr = expanded
	}
	return expr, true
}

func expandTemporalAlias(definition parser.Definition, expr *parser.Expression, seen map[string]bool) (*parser.Expression, bool) {
	if seen[expr.Text] || definition.Kind != "Operator" || len(definition.Params) != len(expr.Args) {
		return nil, false
	}
	seen[expr.Text] = true
	expanded, err := substituteActionCall(definition, expr)
	return expanded, err == nil
}
