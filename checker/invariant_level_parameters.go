package checker

import "github.com/upamune/tla-go/syntax/parser"

// SANY propagates level parameters only through operators whose result level
// depends on that argument. Action and temporal operators have weight zero.
func (m *model) invariantLevelParameters(expr *parser.Expression, seen map[string]bool) bool {
	if invariantLevelStops(expr) {
		return false
	}
	if expr.Kind == "Identifier" || expr.Kind == "Apply" {
		if found, handled := m.invariantReferenceParameters(expr, seen); handled {
			return found
		}
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = m.invariantLevelParameters(child, seen)
		}
	})
	return found
}

func (m *model) invariantReferenceParameters(expr *parser.Expression, seen map[string]bool) (bool, bool) {
	if expr.Kind == "Identifier" && (m.variables[expr.Text] || m.moduleConstant(expr.Text)) {
		return true, true
	}
	definition, ok := m.defs[expr.Text]
	if !ok || seen[expr.Text] {
		return false, false
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	body := definition.Body
	if len(definition.Params) == len(expr.Args) {
		body = substitute(body, parameterReplacements(definition.Params, expr.Args))
	}
	return m.invariantLevelParameters(body, seen), true
}

func invariantLevelStops(expr *parser.Expression) bool {
	switch expr.Kind {
	case "Prime", "Enabled", "Unchanged", "Always", "Eventually", "BoxAction", "AngleAction", "WeakFairness", "StrongFairness":
		return true
	case "Apply":
		return expr.Text == "~>" || expr.Text == "-+->" || expr.Text == "\\cdot"
	}
	return false
}

func (m *model) moduleConstant(name string) bool {
	for _, declaration := range m.module.Constants {
		if declaration.Name == name {
			return true
		}
	}
	return false
}

func parameterReplacements(params []parser.Declaration, args []*parser.Expression) map[string]*parser.Expression {
	replacements := make(map[string]*parser.Expression, len(params))
	for i, param := range params {
		replacements[param.Name] = args[i]
	}
	return replacements
}
