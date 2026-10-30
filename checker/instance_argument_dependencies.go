package checker

import (
	"slices"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

func (r *instanceResolver) hasConstantConstraint(definition parser.Definition, instance parser.Instance) bool {
	if r.hasConstantConstraintCall(definition.Body, instance) {
		return true
	}
	for _, bound := range definition.Bounds {
		if bound.Set != nil && r.hasConstantConstraintCall(bound.Set, instance) {
			return true
		}
	}
	return false
}

func (r *instanceResolver) hasConstantConstraintCall(expr *parser.Expression, instance parser.Instance) bool {
	if expr.Kind == "Apply" && (strings.HasPrefix(expr.Text, instance.Name+"!") || slices.Contains(r.constantRequirements[expr.Text], true)) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found && r.hasConstantConstraintCall(child, instance) {
			found = true
		}
	})
	return found
}
