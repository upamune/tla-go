package checker

import (
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// An operator such as a == x' can stand in for a primed variable on the
// left of an assignment. Follow aliases without crossing local bindings.
func (g *generator) actionAssignmentTarget(target *parser.Expression, locals map[string]value.Value) (string, bool) {
	if target.Kind == "Identifier" && !g.model.variables[target.Text] {
		target = g.primedActionTarget(target, locals, map[string]bool{})
	}
	if target.Kind != "Prime" || len(target.Args) != 1 {
		return "", false
	}
	if name, ok := g.lexical.EnabledVariable(target); g.enabled && ok {
		return name, true
	}
	return g.assignable(target.Args[0], map[string]bool{})
}

func (g *generator) primedActionTarget(expr *parser.Expression, locals map[string]value.Value, seen map[string]bool) *parser.Expression {
	if expr.Kind != "Identifier" || g.boundActionName(expr.Text, locals, seen) {
		return expr
	}
	def, ok := g.definition(expr.Text)
	if !ok || def.Kind != "Operator" || len(def.Params) != 0 {
		return expr
	}
	seen[expr.Text] = true
	return g.primedActionTarget(def.Body, locals, seen)
}

func (g *generator) boundActionName(name string, locals map[string]value.Value, seen map[string]bool) bool {
	_, constant := g.model.constants[name]
	return seen[name] || g.hasLocalValue(name, locals) || g.model.variables[name] || constant
}
