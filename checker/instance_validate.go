package checker

import (
	"fmt"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

func (r *instanceResolver) validateInstanceNames() error {
	names := map[string]bool{}
	declarations := append(append([]parser.Declaration(nil), r.root.Constants...), r.root.Variables...)
	for _, declaration := range declarations {
		names[declaration.Name] = true
	}
	for _, definition := range r.root.Definitions {
		names[definition.Name] = true
	}
	for _, instance := range r.root.Instances {
		if instance.Name == "" {
			continue
		}
		if names[instance.Name] {
			return fmt.Errorf("duplicate instance name %s", instance.Name)
		}
		names[instance.Name] = true
		if err := r.validateReferenceOrder(instance); err != nil {
			return err
		}
	}
	return nil
}

func (r *instanceResolver) validateReferenceOrder(instance parser.Instance) error {
	for _, definition := range r.root.Definitions {
		if earlierDefinitionReference(definition, instance) {
			return fmt.Errorf("instance %s is referenced before its declaration", instance.Name)
		}
	}
	for _, assumption := range r.root.Assumptions {
		if earlierInstanceReference(assumption.Expression, instance) {
			return fmt.Errorf("instance %s is referenced before its declaration", instance.Name)
		}
	}
	return nil
}

func earlierDefinitionReference(definition parser.Definition, instance parser.Instance) bool {
	if earlierInstanceReference(definition.Body, instance) {
		return true
	}
	for _, bound := range definition.Bounds {
		if bound.Set != nil && earlierInstanceReference(bound.Set, instance) {
			return true
		}
	}
	return false
}

func earlierInstanceReference(expr *parser.Expression, instance parser.Instance) bool {
	if (expr.Kind == "Identifier" || expr.Kind == "Apply") && strings.HasPrefix(expr.Text, instance.Name+"!") && expr.Location.Line < instance.Location.Line {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found && earlierInstanceReference(child, instance) {
			found = true
		}
	})
	return found
}

func (r *instanceResolver) validateSubstitutionLevels(module *parser.Module, instance parser.Instance, replacements map[string]*parser.Expression, index int) error {
	if !hasInstanceLevelConstraints(module) {
		return nil
	}
	model := r.substitutionModel()
	model.module = r.root
	addInstanceAnalysisDefinitions(&model, module, instance, replacements, index)
	if err := r.validateStateInstanceArguments(module, instance, replacements, &model); err != nil {
		return err
	}
	limits, _ := instanceConstantLimits(module)
	if err := r.validateBoundedConstantArguments(module, instance, replacements, limits, &model); err != nil {
		return err
	}
	required, err := constantSubstitutionParameters(module.Constants, instance.Params, replacements, limits, &model)
	if err != nil {
		return err
	}
	var targets map[string][]bool
	if len(required) != 0 {
		declarations := constantsWithLevelLimit(module.Constants, limits, 0)
		targets = instanceTargetsForDeclarations(module, declarations, instance, replacements, 0, limits == nil)
	}
	if err := addOperatorConstantRestrictions(module, instance, replacements, &model, required, &targets); err != nil {
		return err
	}
	if err := r.validateConstantInstanceArguments(instance, required, targets, &model); err != nil {
		return err
	}
	return validateVariableSubstitutions(module.Variables, replacements, &model)
}

func hasInstanceLevelConstraints(module *parser.Module) bool {
	if len(module.Variables) > 0 {
		return true
	}
	for _, declaration := range module.Constants {
		if declaration.Arity == 0 {
			return true
		}
	}
	return false
}

func addInstanceAnalysisDefinitions(model *model, module *parser.Module, instance parser.Instance, replacements map[string]*parser.Expression, index int) {
	names := instanceDefinitionNames(module, instance, index)
	generated := instanceGeneratedArities(module, names)
	for _, definition := range module.Definitions {
		name := names[definition.Name]
		self := ""
		if definition.Kind == "Function" {
			self = name
		}
		definition.Name = name
		definition.Body = instanceExpression(definition.Body, replacements, generated, instance.Params, self)
		definition.Params = append(append([]parser.Declaration(nil), instance.Params...), definition.Params...)
		model.defs[name] = definition
	}
}

func (r *instanceResolver) substitutionModel() model {
	model := model{variables: map[string]bool{}, defs: map[string]parser.Definition{}}
	for _, variable := range r.root.Variables {
		model.variables[variable.Name] = true
	}
	for _, definition := range r.root.Definitions {
		model.defs[definition.Name] = definition
	}
	return model
}

func constantSubstitutionParameters(constants, instanceParams []parser.Declaration, replacements map[string]*parser.Expression, limits map[string]int, model *model) (map[string]bool, error) {
	params := declarationNames(instanceParams)
	required := map[string]bool{}
	for _, declaration := range constants {
		// TLC permits operator substitutions to depend on the current state.
		if declaration.Arity > 0 {
			continue
		}
		limit := limits[declaration.Name]
		if limit == 0 {
			addConstantParameters(required, params, replacements[declaration.Name])
		}
		if invalidInstanceConstant(model, replacements[declaration.Name], limit) {
			return nil, instanceConstantLevelError(declaration.Name, limit)
		}
	}
	return required, nil
}

func instanceConstantLevelError(name string, limit int) error {
	if limit == 0 {
		return fmt.Errorf("instance constant %s requires a constant-level substitution", name)
	}
	return fmt.Errorf("instance constant %s requires a substitution at level %d or below", name, limit)
}

func addConstantParameters(required, params map[string]bool, expr *parser.Expression) {
	names := map[string]bool{}
	collectArgumentNames(expr, names)
	for name := range params {
		if names[name] {
			required[name] = true
		}
	}
}

func invalidInstanceConstant(m *model, expr *parser.Expression, limit int) bool {
	if limit == 0 {
		return unsafeConstantSubstitution(m, expr)
	}
	if m.hasTemporal(expr, map[string]bool{}) {
		return limit < 3
	}
	return limit < 2 && m.hasAction(expr, map[string]bool{})
}

func validateVariableSubstitutions(variables []parser.Declaration, replacements map[string]*parser.Expression, model *model) error {
	for _, declaration := range variables {
		expr := replacements[declaration.Name]
		if model.hasAction(expr, map[string]bool{}) || containsQualifiedReference(model, expr, map[string]bool{}) {
			return fmt.Errorf("instance variable %s requires a state-level substitution", declaration.Name)
		}
	}
	return nil
}

func unsafeConstantSubstitution(m *model, expr *parser.Expression) bool {
	return m.hasAction(expr, map[string]bool{}) || m.readsState(expr, map[string]bool{}) || containsQualifiedReference(m, expr, map[string]bool{})
}

func containsQualifiedReference(m *model, expr *parser.Expression, seen map[string]bool) bool {
	if expr.Kind == "Identifier" || expr.Kind == "Apply" || expr.Kind == "OperatorReference" {
		if strings.Contains(expr.Text, "!") && !strings.HasPrefix(expr.Text, "!bound!") {
			if _, ok := m.defs[expr.Text]; !ok {
				return true
			}
		}
		if qualifiedDefinitionReference(m, expr.Text, seen) {
			return true
		}
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found && containsQualifiedReference(m, child, seen) {
			found = true
		}
	})
	return found
}

func qualifiedDefinitionReference(m *model, name string, seen map[string]bool) bool {
	d, ok := m.defs[name]
	if !ok || seen[name] {
		return false
	}
	seen[name] = true
	return containsQualifiedReference(m, d.Body, seen)
}
