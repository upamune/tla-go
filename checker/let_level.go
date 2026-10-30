package checker

import "github.com/upamune/tla-go/syntax/parser"

// Local definitions only affect the level of a LET when they are used outside
// ENABLED. Include dependencies between local definitions before checking its body.
func (m *model) letHasAction(expr *parser.Expression, seen map[string]bool) bool {
	actions := make(map[string]bool, len(expr.Definitions))
	for _, d := range expr.Definitions {
		if m.hasAction(d.Body, seen) {
			actions[d.Name] = true
		}
	}
	for propagateLocalActions(expr.Definitions, actions) {
	}
	return m.hasAction(expr.Args[0], seen) || referencesAction(expr.Args[0], actions)
}

func propagateLocalActions(definitions []parser.Definition, actions map[string]bool) bool {
	changed := false
	for _, d := range definitions {
		if !actions[d.Name] && referencesAction(d.Body, actions) {
			actions[d.Name] = true
			changed = true
		}
	}
	return changed
}

func referencesAction(expr *parser.Expression, actions map[string]bool) bool {
	if expr.Kind == "Enabled" {
		return false
	}
	if (expr.Kind == "Identifier" || expr.Kind == "Apply") && actions[expr.Text] {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = referencesAction(child, actions)
		}
	})
	return found
}
