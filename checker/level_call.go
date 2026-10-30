package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

// Analyze a call after substitution so ENABLED and unused parameters shield
// their arguments in the same way they do in the called operator's body.
func (m *model) actionCallLevel(expr *parser.Expression, seen map[string]bool) (bool, bool) {
	expanded, ok := m.levelCallBody(expr, seen)
	if !ok {
		return false, false
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return m.hasAction(expanded, seen), true
}

func (m *model) temporalCallLevel(expr *parser.Expression, seen map[string]bool) (bool, bool) {
	expanded, ok := m.levelCallBody(expr, seen)
	if !ok {
		return false, false
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return m.hasTemporal(expanded, seen), true
}

func (m *model) levelCallBody(expr *parser.Expression, seen map[string]bool) (*parser.Expression, bool) {
	if expr.Kind != "Apply" || seen[expr.Text] {
		return nil, false
	}
	definition, ok := m.defs[expr.Text]
	if !ok || definition.Kind != "Operator" || len(definition.Params) != len(expr.Args) {
		return nil, false
	}
	expanded, err := substituteActionCall(definition, expr)
	if err != nil {
		renamer := alphaRenamer{prefix: fmt.Sprintf("level-%p", expr)}
		expanded, err = substituteActionCall(renamer.definition(definition, nil), expr)
		if err != nil {
			return nil, false
		}
	}
	return expanded, true
}
