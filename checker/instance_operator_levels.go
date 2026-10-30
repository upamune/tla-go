package checker

import "github.com/upamune/tla-go/syntax/parser"

// SANY constrains operator constants to constant-level substitutions when the
// instantiated module declares variables. In a parameterized instance, that
// constraint applies only to exported definitions that depend on the operator.
func addOperatorConstantRestrictions(module *parser.Module, instance parser.Instance, replacements map[string]*parser.Expression, model *model, required map[string]bool, targets *map[string][]bool) error {
	if len(module.Variables) == 0 {
		return nil
	}
	operators := operatorConstants(module.Constants)
	for _, declaration := range operators {
		expr := replacements[declaration.Name]
		if unsafeConstantSubstitution(model, expr) {
			return instanceConstantLevelError(declaration.Name, 0)
		}
		addConstantParameters(required, declarationNames(instance.Params), expr)
	}
	mergeOperatorConstantTargets(module, instance, replacements, operators, targets)
	return nil
}

func operatorConstants(constants []parser.Declaration) []parser.Declaration {
	var result []parser.Declaration
	for _, declaration := range constants {
		if declaration.Arity > 0 {
			result = append(result, declaration)
		}
	}
	return result
}

func mergeOperatorConstantTargets(module *parser.Module, instance parser.Instance, replacements map[string]*parser.Expression, operators []parser.Declaration, targets *map[string][]bool) {
	if instance.Name == "" || len(operators) == 0 {
		return
	}
	if *targets == nil {
		*targets = map[string][]bool{}
	}
	defs := make(map[string]parser.Definition, len(module.Definitions))
	for _, definition := range module.Definitions {
		defs[definition.Name] = definition
	}
	parameters := instanceVariableParameters(operators, instance.Params, replacements)
	for _, definition := range module.Definitions {
		if !definition.Local {
			mergeOperatorDefinitionTargets(instance, definition, parameters, defs, *targets)
		}
	}
}

func mergeOperatorDefinitionTargets(instance parser.Instance, definition parser.Definition, parameters map[string][]bool, defs map[string]parser.Definition, targets map[string][]bool) {
	name := instance.Name + "!" + definition.Name
	limits := targets[name]
	if limits == nil {
		limits = make([]bool, len(instance.Params))
	}
	for operator, required := range parameters {
		if instanceDefinitionDependsOnVariable(definition, operator, defs) {
			for i, value := range required {
				limits[i] = limits[i] || value
			}
		}
	}
	targets[name] = limits
}
