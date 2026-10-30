package checker

import (
	"fmt"
	"maps"

	"github.com/upamune/tla-go/syntax/parser"
)

type stateInstanceCheck struct {
	model        *model
	targets      map[string][]bool
	requirements map[string][]bool
	localNames   map[string]bool
	maxLevel     int
}

func (r *instanceResolver) validateStateInstanceArguments(module *parser.Module, instance parser.Instance, replacements map[string]*parser.Expression, model *model) error {
	if instance.Name == "" || len(instance.Params) == 0 || len(module.Variables) == 0 {
		return nil
	}
	check := stateInstanceCheck{model: model, targets: stateInstanceTargets(module, instance, replacements), requirements: map[string][]bool{}, maxLevel: 1}
	return check.validate(r.root)
}

func (c *stateInstanceCheck) validate(root *parser.Module) error {
	if !c.hasTargets() {
		return nil
	}
	for _, def := range root.Definitions {
		c.requirements[def.Name] = make([]bool, len(def.Params))
	}
	if err := c.infer(root.Definitions); err != nil {
		return err
	}
	return c.statements(root)
}

func (c *stateInstanceCheck) statements(module *parser.Module) error {
	for _, assumption := range module.Assumptions {
		if err := c.expression(assumption.Expression, nil); err != nil {
			return err
		}
	}
	for _, theorem := range module.Theorems {
		if err := c.expression(theorem.Statement, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *stateInstanceCheck) hasTargets() bool {
	for _, limits := range c.targets {
		for _, required := range limits {
			if required {
				return true
			}
		}
	}
	return false
}

func (c *stateInstanceCheck) infer(defs []parser.Definition) error {
	return c.inferScoped(defs, nil)
}

func (c *stateInstanceCheck) inferScoped(defs []parser.Definition, outer map[string]bool) error {
	for {
		changed, err := c.inferPass(defs, outer)
		if err != nil || !changed {
			return err
		}
	}
}

func (c *stateInstanceCheck) inferPass(defs []parser.Definition, outer map[string]bool) (bool, error) {
	changed := false
	for _, def := range defs {
		if !c.hasConstraint(def.Body) && !c.boundsHaveConstraint(def.Bounds) {
			continue
		}
		if err := c.definition(def, outer, ""); err != nil {
			return false, err
		}
		changed = c.probeParameters(def, outer) || changed
	}
	return changed, nil
}

func (c *stateInstanceCheck) probeParameters(def parser.Definition, outer map[string]bool) bool {
	changed := false
	for i, param := range def.Params {
		if i >= len(c.requirements[def.Name]) {
			break
		}
		if !c.requirements[def.Name][i] && c.definition(def, outer, param.Name) != nil {
			c.requirements[def.Name][i] = true
			changed = true
		}
	}
	return changed
}

func (c *stateInstanceCheck) boundsHaveConstraint(bounds []parser.Bound) bool {
	for _, bound := range bounds {
		if bound.Set != nil && c.hasConstraint(bound.Set) {
			return true
		}
	}
	return false
}

func (c *stateInstanceCheck) hasConstraint(expr *parser.Expression) bool {
	if expr == nil {
		return false
	}
	if expr.Kind == "Apply" && (hasStateLimit(c.targets[expr.Text]) || hasStateLimit(c.requirements[expr.Text])) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = c.hasConstraint(child)
		}
	})
	return found
}

func hasStateLimit(limits []bool) bool {
	for _, required := range limits {
		if required {
			return true
		}
	}
	return false
}

func (c *stateInstanceCheck) definition(def parser.Definition, outer map[string]bool, probe string) error {
	scope := maps.Clone(outer)
	if scope == nil {
		scope = map[string]bool{}
	}
	for _, param := range def.Params {
		scope[param.Name] = param.Name != probe
	}
	for _, bound := range def.Bounds {
		if err := c.bound(bound, scope); err != nil {
			return err
		}
	}
	return c.expression(def.Body, scope)
}

func (c *stateInstanceCheck) expression(expr *parser.Expression, outer map[string]bool) error {
	if expr == nil {
		return nil
	}
	if err := c.call(expr, outer); err != nil {
		return err
	}
	scope, err := c.expressionScope(expr, outer)
	if err != nil {
		return err
	}
	return c.children(expr, scope)
}

func (c *stateInstanceCheck) expressionScope(expr *parser.Expression, outer map[string]bool) (map[string]bool, error) {
	scope := maps.Clone(outer)
	if scope == nil {
		scope = map[string]bool{}
	}
	for _, bound := range expr.Bounds {
		if err := c.bound(bound, scope); err != nil {
			return nil, err
		}
	}
	addStateLocalNames(scope, expr)
	return scope, nil
}

func addStateLocalNames(scope map[string]bool, expr *parser.Expression) {
	if expr.Kind == "Lambda" || expr.Kind == "UnboundedQuantifier" {
		for _, name := range expr.Names {
			scope[name] = true
		}
	}
	for _, def := range expr.Definitions {
		scope[def.Name] = true
	}
}

func (c *stateInstanceCheck) bound(bound parser.Bound, scope map[string]bool) error {
	if bound.Set != nil {
		if err := c.expression(bound.Set, scope); err != nil {
			return err
		}
	}
	for _, name := range bound.Names {
		scope[name] = true
	}
	return nil
}

func (c *stateInstanceCheck) call(expr *parser.Expression, scope map[string]bool) error {
	if expr.Kind != "Apply" {
		return nil
	}
	if c.localNameShadowsTarget(expr.Text, scope) {
		return nil
	}
	limits := c.targets[expr.Text]
	if limits == nil {
		limits = c.requirements[expr.Text]
	}
	for i, required := range limits {
		if required && i < len(expr.Args) && !c.stateArgument(expr.Args[i], scope) {
			return &SemanticError{Err: fmt.Errorf("Level error in applying operator %s: The level of argument %d exceeds the maximum level allowed by the operator.", expr.Text, i+1)}
		}
	}
	return nil
}

func (c *stateInstanceCheck) localNameShadowsTarget(name string, scope map[string]bool) bool {
	_, shadowed := scope[name]
	return shadowed && !c.localNames[name]
}

func (c *stateInstanceCheck) children(expr *parser.Expression, scope map[string]bool) error {
	if len(expr.Definitions) != 0 {
		return c.localChildren(expr, scope)
	}
	return c.expressionChildren(expr, scope)
}

func (c *stateInstanceCheck) expressionChildren(expr *parser.Expression, scope map[string]bool) error {
	for _, def := range expr.Definitions {
		if err := c.definition(def, scope, ""); err != nil {
			return err
		}
	}
	copy := *expr
	copy.Bounds, copy.Definitions = nil, nil
	var failure error
	walkChildren(&copy, func(child *parser.Expression) {
		if failure == nil {
			failure = c.expression(child, scope)
		}
	})
	return failure
}

func (c *stateInstanceCheck) stateArgument(expr *parser.Expression, scope map[string]bool) bool {
	local := *c.model
	shadows := stateArgumentShadows(expr, scope, local.defs)
	if len(shadows) > 0 {
		local.defs = maps.Clone(local.defs)
		for name := range shadows {
			delete(local.defs, name)
		}
	}
	return (c.maxLevel >= 2 || !local.hasAction(expr, map[string]bool{})) && !stateArgumentTemporal(expr, local.defs, map[string]bool{}) && !stateProbeUsed(expr, scope, local.defs, map[string]bool{})
}

func stateArgumentShadows(expr *parser.Expression, scope map[string]bool, defs map[string]parser.Definition) map[string]bool {
	shadows := map[string]bool{}
	for name := range scope {
		if _, ok := defs[name]; ok {
			shadows[name] = true
		}
	}
	collectStateArgumentShadows(expr, defs, shadows)
	return shadows
}

func stateProbeUsed(expr *parser.Expression, scope map[string]bool, defs map[string]parser.Definition, seen map[string]bool) bool {
	if expr.Kind == "Enabled" {
		return false
	}
	if len(expr.Definitions) > 0 {
		return stateProbeLet(expr, scope, defs, seen)
	}
	if expanded, ok := stateProbeCall(expr, scope, defs, seen); ok {
		seen[expr.Text] = true
		defer delete(seen, expr.Text)
		return stateProbeUsed(expanded, scope, defs, seen)
	}
	if stateProbeName(expr, scope) {
		return true
	}
	found := false
	scope = stateProbeScope(expr, scope)
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = stateProbeUsed(child, scope, defs, seen)
		}
	})
	return found
}

func stateProbeName(expr *parser.Expression, scope map[string]bool) bool {
	if expr.Kind != "Identifier" && expr.Kind != "Apply" && expr.Kind != "OperatorReference" {
		return false
	}
	valid, known := scope[expr.Text]
	return known && !valid
}

func stateProbeCall(expr *parser.Expression, scope map[string]bool, defs map[string]parser.Definition, seen map[string]bool) (*parser.Expression, bool) {
	if expr.Kind != "Apply" && expr.Kind != "Identifier" || seen[expr.Text] {
		return nil, false
	}
	if _, shadowed := scope[expr.Text]; shadowed {
		return nil, false
	}
	def, ok := stateProbeDefinition(expr, defs)
	if !ok {
		return nil, false
	}
	if expr.Kind == "Identifier" {
		return def.Body, true
	}
	expanded, err := substituteActionCall(def, expr)
	if err != nil {
		renamer := alphaRenamer{prefix: fmt.Sprintf("state-level-%p", expr)}
		expanded, err = substituteActionCall(renamer.definition(def, nil), expr)
	}
	return expanded, err == nil
}

func stateProbeDefinition(expr *parser.Expression, defs map[string]parser.Definition) (parser.Definition, bool) {
	def, ok := defs[expr.Text]
	return def, ok && def.Kind == "Operator" && len(def.Params) == len(expr.Args)
}
