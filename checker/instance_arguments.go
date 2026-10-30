package checker

import (
	"fmt"
	"maps"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

func (r *instanceResolver) validateConstantInstanceArguments(instance parser.Instance, required map[string]bool, model *model) error {
	if len(required) == 0 {
		return nil
	}
	for _, definition := range r.root.Definitions {
		if err := r.checkConstantDefinition(definition, nil, instance, required, model); err != nil {
			return err
		}
	}
	for _, assumption := range r.root.Assumptions {
		if err := r.checkConstantCalls(assumption.Expression, nil, instance, required, model); err != nil {
			return err
		}
	}
	return nil
}

func (r *instanceResolver) checkConstantDefinition(definition parser.Definition, outer map[string]bool, instance parser.Instance, required map[string]bool, model *model) error {
	scope := maps.Clone(outer)
	if scope == nil {
		scope = map[string]bool{}
	}
	for name := range unknownParameters(definition.Params) {
		scope[name] = false
	}
	for _, bound := range definition.Bounds {
		if err := r.checkConstantBound(bound, scope, instance, required, model); err != nil {
			return err
		}
	}
	return r.checkConstantCalls(definition.Body, scope, instance, required, model)
}

func unknownParameters(params []parser.Declaration) map[string]bool {
	scope := map[string]bool{}
	for _, param := range params {
		scope[param.Name] = false
	}
	return scope
}

func (r *instanceResolver) checkConstantCalls(expr *parser.Expression, outer map[string]bool, instance parser.Instance, required map[string]bool, model *model) error {
	if err := r.checkInstanceCall(expr, outer, instance, required, model); err != nil {
		return err
	}
	scope := maps.Clone(outer)
	if scope == nil {
		scope = map[string]bool{}
	}
	for _, bound := range expr.Bounds {
		if err := r.checkConstantBound(bound, scope, instance, required, model); err != nil {
			return err
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
	return r.checkConstantChildren(expr, scope, instance, required, model)
}

func (r *instanceResolver) checkConstantBound(bound parser.Bound, scope map[string]bool, instance parser.Instance, required map[string]bool, model *model) error {
	if bound.Set != nil {
		if err := r.checkConstantCalls(bound.Set, scope, instance, required, model); err != nil {
			return err
		}
	}
	constant := bound.Set != nil && r.isConstantArgument(bound.Set, scope, model)
	for _, name := range bound.Names {
		scope[name] = constant
	}
	return nil
}

func (r *instanceResolver) checkInstanceCall(expr *parser.Expression, scope map[string]bool, instance parser.Instance, required map[string]bool, model *model) error {
	if expr.Kind != "Apply" || !strings.HasPrefix(expr.Text, instance.Name+"!") {
		return nil
	}
	if len(expr.ArgumentCounts) == 0 || expr.ArgumentCounts[0] != len(instance.Params) {
		return fmt.Errorf("invalid arguments for instance %s", instance.Name)
	}
	for i, param := range instance.Params {
		if required[param.Name] && !r.isConstantArgument(expr.Args[i], scope, model) {
			return fmt.Errorf("instance parameter %s requires a constant argument", param.Name)
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
