package checker

import (
	"fmt"
	"maps"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

func (r *instanceResolver) validateConstantInstanceArguments(instance parser.Instance, required map[string]bool, targets map[string][]bool, model *model) error {
	if len(required) == 0 {
		return nil
	}
	previous := r.constantRequirements
	previousTargets := r.constantTargets
	r.constantRequirements = map[string][]bool{}
	r.constantTargets = targets
	defer func() {
		r.constantRequirements = previous
		r.constantTargets = previousTargets
	}()
	if err := r.inferConstantRequirements(instance, required, model); err != nil {
		return err
	}
	for _, assumption := range r.root.Assumptions {
		if err := r.checkConstantCalls(assumption.Expression, nil, instance, required, model); err != nil {
			return err
		}
	}
	return nil
}

func (r *instanceResolver) inferConstantRequirements(instance parser.Instance, required map[string]bool, model *model) error {
	for _, definition := range r.root.Definitions {
		r.constantRequirements[definition.Name] = make([]bool, len(definition.Params))
	}
	for {
		changed, err := r.inferConstantPass(instance, required, model)
		if err != nil || !changed {
			return err
		}
	}
}

func (r *instanceResolver) inferConstantPass(instance parser.Instance, required map[string]bool, model *model) (bool, error) {
	changed := false
	for _, definition := range r.root.Definitions {
		updated, err := r.inferDefinitionRequirements(definition, nil, instance, required, model)
		if err != nil {
			return false, err
		}
		changed = changed || updated
	}
	return changed, nil
}

func (r *instanceResolver) inferDefinitionRequirements(definition parser.Definition, outer map[string]bool, instance parser.Instance, required map[string]bool, model *model) (bool, error) {
	if !r.hasConstantConstraint(definition, instance) {
		return false, nil
	}
	if err := r.checkConstantDefinitionWithProbe(definition, outer, instance, required, model, ""); err != nil {
		return false, err
	}
	if constantRecursiveDefinition(definition, model) {
		return false, nil
	}
	return r.probeConstantParameters(definition, outer, instance, required, model), nil
}

func constantRecursiveDefinition(definition parser.Definition, model *model) bool {
	// TLC does not propagate these constraints through constant-level
	// recursive operators, but does so when the body reads state.
	return definition.Recursive && !model.readsState(definition.Body, map[string]bool{}) && !model.hasAction(definition.Body, map[string]bool{})
}

func (r *instanceResolver) probeConstantParameters(definition parser.Definition, outer map[string]bool, instance parser.Instance, required map[string]bool, model *model) bool {
	changed := false
	for i, param := range definition.Params {
		if !r.constantRequirements[definition.Name][i] && r.checkConstantDefinitionWithProbe(definition, outer, instance, required, model, param.Name) != nil {
			r.constantRequirements[definition.Name][i] = true
			changed = true
		}
	}
	return changed
}

func (r *instanceResolver) checkConstantDefinitionWithProbe(definition parser.Definition, outer map[string]bool, instance parser.Instance, required map[string]bool, model *model, probe string) error {
	scope := maps.Clone(outer)
	if scope == nil {
		scope = map[string]bool{}
	}
	for _, param := range definition.Params {
		scope[param.Name] = param.Name != probe
	}
	for _, bound := range definition.Bounds {
		if err := r.checkConstantBound(bound, scope, instance, required, model); err != nil {
			return err
		}
	}
	return r.checkConstantCalls(definition.Body, scope, instance, required, model)
}

func (r *instanceResolver) checkConstantCalls(expr *parser.Expression, outer map[string]bool, instance parser.Instance, required map[string]bool, model *model) error {
	if err := r.checkInstanceCall(expr, outer, instance, required, model); err != nil {
		return err
	}
	scope, err := r.constantExpressionScope(expr, outer, instance, required, model)
	if err != nil {
		return err
	}
	if len(expr.Definitions) != 0 {
		return r.checkLocalConstantChildren(expr, scope, instance, required, model)
	}
	return r.checkConstantChildren(expr, scope, instance, required, model)
}

func (r *instanceResolver) constantExpressionScope(expr *parser.Expression, outer map[string]bool, instance parser.Instance, required map[string]bool, model *model) (map[string]bool, error) {
	scope := maps.Clone(outer)
	if scope == nil {
		scope = map[string]bool{}
	}
	for _, bound := range expr.Bounds {
		if err := r.checkConstantBound(bound, scope, instance, required, model); err != nil {
			return nil, err
		}
	}
	if expr.Kind == "Lambda" {
		for _, name := range expr.Names {
			scope[name] = false
		}
	}
	for _, definition := range expr.Definitions {
		scope[definition.Name] = false
	}
	return scope, nil
}

func (r *instanceResolver) checkConstantBound(bound parser.Bound, scope map[string]bool, instance parser.Instance, required map[string]bool, model *model) error {
	if bound.Set != nil {
		if err := r.checkConstantCalls(bound.Set, scope, instance, required, model); err != nil {
			return err
		}
	}
	constant := bound.Set != nil
	for _, name := range bound.Names {
		scope[name] = constant
	}
	return nil
}

func (r *instanceResolver) checkInstanceCall(expr *parser.Expression, scope map[string]bool, instance parser.Instance, required map[string]bool, model *model) error {
	if expr.Kind != "Apply" {
		return nil
	}
	if !strings.HasPrefix(expr.Text, instance.Name+"!") {
		return r.checkRequiredOperatorArguments(expr, scope, model)
	}
	if len(expr.ArgumentCounts) == 0 || expr.ArgumentCounts[0] != len(instance.Params) {
		return fmt.Errorf("invalid arguments for instance %s", instance.Name)
	}
	for i, param := range instance.Params {
		if r.invalidConstantInstanceArgument(expr, i, required[param.Name], scope, model) {
			return fmt.Errorf("instance parameter %s requires a constant argument", param.Name)
		}
	}
	return nil
}

func (r *instanceResolver) invalidConstantInstanceArgument(expr *parser.Expression, index int, required bool, scope map[string]bool, model *model) bool {
	return required && r.constantTargetArgument(expr.Text, index) && !r.isConstantArgument(expr.Args[index], scope, model)
}

func (r *instanceResolver) constantTargetArgument(name string, index int) bool {
	target := r.constantTargets[name]
	return target == nil || index < len(target) && target[index]
}

func (r *instanceResolver) checkRequiredOperatorArguments(expr *parser.Expression, scope map[string]bool, model *model) error {
	if _, shadowed := scope[expr.Text]; shadowed && !r.constantLocalNames[expr.Text] {
		return nil
	}
	for i, required := range r.constantRequirements[expr.Text] {
		if required && i < len(expr.Args) && !r.isConstantArgument(expr.Args[i], scope, model) {
			return fmt.Errorf("operator %s requires a constant argument", expr.Text)
		}
	}
	return nil
}

func (r *instanceResolver) isConstantArgument(expr *parser.Expression, scope map[string]bool, model *model) bool {
	if model.hasAction(expr, map[string]bool{}) || model.readsState(expr, map[string]bool{}) || containsQualifiedReference(model, expr, map[string]bool{}) {
		return false
	}
	return r.knownConstantNames(expr, scope, model)
}

func (r *instanceResolver) knownConstantNames(expr *parser.Expression, scope map[string]bool, model *model) bool {
	if expr.Kind == "Lambda" {
		scope = maps.Clone(scope)
		if scope == nil {
			scope = map[string]bool{}
		}
		for _, name := range expr.Names {
			scope[name] = true
		}
	}
	if expr.Kind == "Identifier" && !r.constantName(expr.Text, scope, model) {
		return false
	}
	valid := true
	walkChildren(expr, func(child *parser.Expression) {
		if valid && !r.knownConstantNames(child, scope, model) {
			valid = false
		}
	})
	return valid
}

func (r *instanceResolver) constantName(name string, scope map[string]bool, model *model) bool {
	if known, ok := scope[name]; ok {
		return known
	}
	if _, ok := model.defs[name]; ok {
		return true
	}
	for _, declaration := range r.root.Constants {
		if declaration.Name == name {
			return true
		}
	}
	switch name {
	case "TRUE", "FALSE", "BOOLEAN", "STRING", "Nat", "Int":
		return true
	}
	return false
}
