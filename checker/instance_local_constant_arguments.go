package checker

import (
	"maps"

	"github.com/upamune/tla-go/syntax/parser"
)

func (r *instanceResolver) checkLocalConstantChildren(expr *parser.Expression, scope map[string]bool, instance parser.Instance, required map[string]bool, model *model) error {
	previousRequirements, previousNames := r.constantRequirements, r.constantLocalNames
	r.constantRequirements = maps.Clone(previousRequirements)
	r.constantLocalNames = maps.Clone(previousNames)
	if r.constantLocalNames == nil {
		r.constantLocalNames = map[string]bool{}
	}
	defer func() {
		r.constantRequirements, r.constantLocalNames = previousRequirements, previousNames
	}()
	localModel := *model
	localModel.defs = maps.Clone(model.defs)
	localScope := maps.Clone(scope)
	for _, definition := range expr.Definitions {
		r.constantRequirements[definition.Name] = make([]bool, len(definition.Params))
		r.constantLocalNames[definition.Name] = true
		localModel.defs[definition.Name] = definition
		localScope[definition.Name] = true
	}
	if err := r.inferLocalConstantRequirements(expr.Definitions, localScope, instance, required, &localModel); err != nil {
		return err
	}
	return r.checkConstantChildren(expr, localScope, instance, required, &localModel)
}

func (r *instanceResolver) inferLocalConstantRequirements(definitions []parser.Definition, scope map[string]bool, instance parser.Instance, required map[string]bool, model *model) error {
	for {
		changed := false
		for _, definition := range definitions {
			updated, err := r.inferDefinitionRequirements(definition, scope, instance, required, model)
			if err != nil {
				return err
			}
			changed = changed || updated
		}
		if !changed {
			return nil
		}
	}
}
