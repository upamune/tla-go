package checker

import (
	"maps"

	"github.com/upamune/tla-go/syntax/parser"
)

// staticConstantExpression accepts only a closed, side-effect-free subset of
// TLA+ syntax. Unknown operators and contextual values are left uncached.
func (m *model) staticConstantExpression(expr *parser.Expression, bound map[string]bool) bool {
	if expr == nil {
		return false
	}
	switch expr.Kind {
	case "Number", "String":
		return true
	case "Identifier":
		return m.staticIdentifier(expr.Text, bound)
	case "Apply":
		if !m.staticOperator(expr.Text) {
			return false
		}
	case "Choose", "Exists", "Forall":
		return m.staticBoundExpression(expr, bound)
	case "And", "Or", "If", "Set", "Tuple":
	default:
		return false
	}
	return m.staticArguments(expr.Args, bound)
}

func (m *model) staticIdentifier(name string, bound map[string]bool) bool {
	_, constant := m.constants[name]
	return bound[name] || constant || name == "TRUE" || name == "FALSE"
}

func (m *model) staticOperator(name string) bool {
	if _, shadowed := m.defs[name]; shadowed {
		return false
	}
	switch name {
	case "SUBSET", "UNION", "~", "-.", "+", "-", "*", "^", "\\div", "%", "..", "=", "#", "<", "<=", ">", ">=", "\\in", "\\notin", "\\subseteq", "\\cup", "\\cap", "\\", "=>", "<=>":
		return true
	default:
		return false
	}
}

func (m *model) staticBoundExpression(expr *parser.Expression, bound map[string]bool) bool {
	if len(expr.Bounds) == 0 {
		return false
	}
	local := maps.Clone(bound)
	if local == nil {
		local = make(map[string]bool)
	}
	for _, b := range expr.Bounds {
		if !m.staticConstantExpression(b.Set, local) {
			return false
		}
		for _, name := range b.Names {
			local[name] = true
		}
	}
	return m.staticArguments(expr.Args, local)
}

func (m *model) staticArguments(args []*parser.Expression, bound map[string]bool) bool {
	for _, arg := range args {
		if !m.staticConstantExpression(arg, bound) {
			return false
		}
	}
	return true
}
