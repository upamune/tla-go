package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

// Analyze a call after substitution so ENABLED and unused parameters shield
// their arguments in the same way they do in the called operator's body.
func (m *model) actionCallLevel(expr *parser.Expression, seen map[string]bool) (bool, bool) {
	if expr.Kind != "Apply" || seen[expr.Text] {
		return false, false
	}
	definition, ok := m.defs[expr.Text]
	if !ok || definition.Kind != "Operator" || len(definition.Params) != len(expr.Args) {
		return false, false
	}
	expanded, err := substituteActionCall(definition, expr)
	if err != nil {
		renamer := alphaRenamer{prefix: fmt.Sprintf("level-%p", expr)}
		expanded, err = substituteActionCall(renamer.definition(definition, nil), expr)
		if err != nil {
			return false, false
		}
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return m.hasAction(expanded, seen), true
}
