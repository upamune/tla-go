package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func (r *instanceResolver) instantiate(result *parser.Module, instance parser.Instance, index int) error {
	module, err := r.instanceModule(instance)
	if err != nil {
		return err
	}
	module, instance = alphaInstance(module, instance)
	instance, err = r.resolveInstanceSubexpressions(instance, result)
	if err != nil {
		return err
	}
	replacements, err := r.prepareInstance(module, instance, index)
	if err != nil {
		if isBuiltinTLCModule(module) {
			return &SemanticError{Err: err}
		}
		return err
	}
	generated := instanceDefinitionNames(module, instance, index)
	generatedSet := instanceGeneratedArities(module, generated)
	variables := make(map[string]bool, len(module.Variables))
	for _, variable := range module.Variables {
		variables[variable.Name] = true
	}
	r.session.nextInstance++
	scope := fmt.Sprintf("!instance-scope!%d!", r.session.nextInstance)
	for _, definition := range module.Definitions {
		result.Definitions = append(result.Definitions, instanceDefinition(definition, instance, replacements, generated, generatedSet, variables, scope))
	}
	result.Definitions = append(result.Definitions, instanceValidationDefinitions(module, instance, replacements, index)...)
	return nil
}

func instanceGeneratedArities(module *parser.Module, generated map[string]string) map[string]int {
	arities := make(map[string]int, len(generated))
	for _, definition := range module.Definitions {
		arity := 0
		if definition.Kind == "Operator" {
			arity = len(definition.Params)
			if hasOperatorParameters(definition.Params) {
				arity = -1
			}
		}
		arities[generated[definition.Name]] = arity
	}
	return arities
}

func (r *instanceResolver) prepareInstance(module *parser.Module, instance parser.Instance, index int) (map[string]*parser.Expression, error) {
	replacements, err := instanceReplacements(module, instance, index)
	if err != nil {
		return nil, err
	}
	if err := r.validateSubstitutionLevels(module, instance, replacements, index); err != nil {
		return nil, err
	}
	return replacements, nil
}

func instanceDefinitionNames(module *parser.Module, instance parser.Instance, index int) map[string]string {
	names := make(map[string]string, len(module.Definitions))
	for _, definition := range module.Definitions {
		names[definition.Name] = instanceDefinitionName(definition, instance, index)
	}
	return names
}

func instanceDefinitionName(definition parser.Definition, instance parser.Instance, index int) string {
	if definition.Local {
		return fmt.Sprintf("!instance!%d!%s", index, definition.Name)
	}
	if instance.Name != "" {
		return instance.Name + "!" + definition.Name
	}
	return definition.Name
}

func instanceReplacements(module *parser.Module, instance parser.Instance, index int) (map[string]*parser.Expression, error) {
	replacements := make(map[string]*parser.Expression)
	declarations := append(append([]parser.Declaration(nil), module.Constants...), module.Variables...)
	for _, declaration := range declarations {
		replacements[declaration.Name] = &parser.Expression{Kind: "Identifier", Text: declaration.Name}
	}
	if err := applyInstanceSubstitutions(replacements, instance.Substitutions); err != nil {
		return nil, err
	}
	if err := validateInstanceOperators(module.Constants, replacements); err != nil {
		return nil, err
	}
	if err := addInstanceDefinitions(replacements, instanceDefinitionNames(module, instance, index)); err != nil {
		return nil, err
	}
	return replacements, nil
}

func applyInstanceSubstitutions(replacements map[string]*parser.Expression, substitutions []parser.Substitution) error {
	seen := map[string]bool{}
	for _, substitution := range substitutions {
		if _, ok := replacements[substitution.Name]; !ok || seen[substitution.Name] {
			return fmt.Errorf("invalid instance substitution %s", substitution.Name)
		}
		seen[substitution.Name] = true
		replacements[substitution.Name] = substitution.Expression
	}
	return nil
}

func validateInstanceOperators(declarations []parser.Declaration, replacements map[string]*parser.Expression) error {
	for _, declaration := range declarations {
		if declaration.Arity > 0 {
			if err := validActionOperator(replacements[declaration.Name], declaration.Arity); err != nil {
				return fmt.Errorf("invalid operator substitution for %s", declaration.Name)
			}
		}
	}
	return nil
}

func addInstanceDefinitions(replacements map[string]*parser.Expression, names map[string]string) error {
	for original, name := range names {
		if _, exists := replacements[original]; exists {
			return fmt.Errorf("duplicate instance declaration %s", original)
		}
		replacements[original] = &parser.Expression{Kind: "Identifier", Text: name}
	}
	return nil
}

func instanceValidationDefinitions(module *parser.Module, instance parser.Instance, replacements map[string]*parser.Expression, index int) []parser.Definition {
	declarations := append(append([]parser.Declaration(nil), module.Constants...), module.Variables...)
	var checks []parser.Definition
	for _, declaration := range declarations {
		if declaration.Arity == 0 {
			checks = append(checks, parser.Definition{Kind: "Operator", Name: fmt.Sprintf("!instance-check!%d!%s", index, declaration.Name), Params: instance.Params, Body: replacements[declaration.Name], Location: instance.Location})
		} else {
			checks = append(checks, operatorValidationDefinitions(instance, declaration, replacements[declaration.Name], index)...)
		}
	}
	return checks
}

func operatorValidationDefinitions(instance parser.Instance, declaration parser.Declaration, replacement *parser.Expression, index int) []parser.Definition {
	guard := fmt.Sprintf("!instance-operator-guard!%d!%s", index, declaration.Name)
	parameter := parser.Declaration{Name: "!operator-parameter", Arity: declaration.Arity}
	return []parser.Definition{
		{Kind: "Operator", Name: guard, Params: []parser.Declaration{parameter}, Body: &parser.Expression{Kind: "Identifier", Text: "TRUE"}, Location: instance.Location},
		{Kind: "Operator", Name: fmt.Sprintf("!instance-check!%d!%s", index, declaration.Name), Params: instance.Params, Body: &parser.Expression{Kind: "Apply", Text: guard, Args: []*parser.Expression{replacement}}, Location: instance.Location},
	}
}
