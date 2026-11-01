package checker

import (
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

func moduleHasLetInstances(module *parser.Module) bool {
	if slices.ContainsFunc(module.Definitions, definitionHasLetInstance) {
		return true
	}
	for _, assumption := range module.Assumptions {
		if expressionHasLetInstance(assumption.Expression) {
			return true
		}
	}
	return false
}

func definitionHasLetInstance(def parser.Definition) bool {
	if expressionHasLetInstance(def.Body) {
		return true
	}
	for _, bound := range def.Bounds {
		if expressionHasLetInstance(bound.Set) {
			return true
		}
	}
	return false
}

func expressionHasLetInstance(expr *parser.Expression) bool {
	if expr == nil {
		return false
	}
	if len(expr.Instances) > 0 {
		return true
	}
	return expressionListHasLetInstance(expr.Args) || boundsHaveLetInstance(expr.Bounds) || fieldsHaveLetInstance(expr.Fields) || armsHaveLetInstance(expr.Arms) || updatesHaveLetInstance(expr.Updates) || definitionsHaveLetInstance(expr.Definitions)
}

func expressionListHasLetInstance(items []*parser.Expression) bool {
	return slices.ContainsFunc(items, expressionHasLetInstance)
}

func boundsHaveLetInstance(bounds []parser.Bound) bool {
	for _, bound := range bounds {
		if expressionHasLetInstance(bound.Set) {
			return true
		}
	}
	return false
}

func fieldsHaveLetInstance(fields []parser.Field) bool {
	for _, field := range fields {
		if expressionHasLetInstance(field.Value) {
			return true
		}
	}
	return false
}

func armsHaveLetInstance(arms []parser.CaseArm) bool {
	for _, arm := range arms {
		if expressionHasLetInstance(arm.Condition) || expressionHasLetInstance(arm.Body) {
			return true
		}
	}
	return false
}

func updatesHaveLetInstance(updates []parser.Update) bool {
	for _, update := range updates {
		if expressionHasLetInstance(update.Value) || pathHasLetInstance(update.Path) {
			return true
		}
	}
	return false
}

func pathHasLetInstance(path []parser.PathStep) bool {
	for _, step := range path {
		if expressionListHasLetInstance(step.Args) {
			return true
		}
	}
	return false
}

func definitionsHaveLetInstance(definitions []parser.Definition) bool {
	return slices.ContainsFunc(definitions, definitionHasLetInstance)
}

func (r *instanceResolver) expandLetInstances(module *parser.Module) error {
	for i := range module.Definitions {
		if err := r.expandLetDefinition(&module.Definitions[i]); err != nil {
			return err
		}
	}
	module.Assumptions = append([]parser.Assumption(nil), module.Assumptions...)
	for i := range module.Assumptions {
		expr, err := r.expandLetExpression(module.Assumptions[i].Expression)
		if err != nil {
			return err
		}
		module.Assumptions[i].Expression = expr
	}
	return nil
}

func (r *instanceResolver) expandLetDefinition(definition *parser.Definition) error {
	bounds := append([]parser.Bound(nil), definition.Bounds...)
	for i := range bounds {
		set, err := r.expandLetExpression(bounds[i].Set)
		if err != nil {
			return err
		}
		bounds[i].Set = set
	}
	definition.Bounds = bounds
	body, err := r.expandLetExpression(definition.Body)
	if err != nil {
		return err
	}
	definition.Body = body
	return nil
}

func (r *instanceResolver) expandLetExpression(expr *parser.Expression) (*parser.Expression, error) {
	if expr == nil || !expressionHasLetInstance(expr) {
		return expr, nil
	}
	copy := substitute(expr, nil)
	if err := r.expandLetTree(copy, nil, r.rootInstances); err != nil {
		return nil, err
	}
	return copy, nil
}

func (r *instanceResolver) expandLetTree(expr *parser.Expression, outer []parser.Definition, outerInstances []parser.Instance) error {
	scope := outer
	instanceScope := outerInstances
	if expr.Kind == "Let" {
		scope = append(append([]parser.Definition(nil), outer...), expr.Definitions...)
		instanceScope = append(append([]parser.Instance(nil), outerInstances...), expr.Instances...)
	}
	if len(expr.Instances) > 0 {
		if err := r.expandLetScope(expr, scope, outerInstances); err != nil {
			return err
		}
	}
	var failure error
	walkChildren(expr, func(child *parser.Expression) {
		if failure == nil && expressionHasLetInstance(child) {
			failure = r.expandLetTree(child, scope, instanceScope)
		}
	})
	return failure
}

func (r *instanceResolver) expandLetScope(expr *parser.Expression, locals []parser.Definition, outerInstances []parser.Instance) error {
	if err := validateLetInstanceNames(expr); err != nil {
		return err
	}
	generated := &parser.Module{}
	scopedRoot := *r.root
	scopedRoot.Definitions = append(append([]parser.Definition(nil), r.root.Definitions...), locals...)
	scoped := *r
	scoped.root = &scopedRoot
	for index, instance := range expr.Instances {
		if err := r.validateOuterInstanceName(instance, locals, outerInstances); err != nil {
			return err
		}
		var err error
		instance, err = r.expandLetSubstitutions(instance, locals, outerInstances)
		if err != nil {
			return err
		}
		if err := scoped.instantiate(generated, instance, index); err != nil {
			return err
		}
		outerInstances = append(append([]parser.Instance(nil), outerInstances...), instance)
	}
	r.proofModules = append(r.proofModules, generated.ProofModules...)
	r.instanceStandardModules = appendUniqueModules(r.instanceStandardModules, generated.InstanceStandardModules...)
	expr.Definitions = append(expr.Definitions, generated.Definitions...)
	expr.Instances = nil
	return nil
}

func (r *instanceResolver) expandLetSubstitutions(instance parser.Instance, locals []parser.Definition, outerInstances []parser.Instance) (parser.Instance, error) {
	instance.Substitutions = append([]parser.Substitution(nil), instance.Substitutions...)
	for i := range instance.Substitutions {
		expr := instance.Substitutions[i].Expression
		if !expressionHasLetInstance(expr) {
			continue
		}
		copy := substitute(expr, nil)
		if err := r.expandLetTree(copy, locals, outerInstances); err != nil {
			return instance, err
		}
		instance.Substitutions[i].Expression = copy
	}
	return instance, nil
}
