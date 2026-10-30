package checker

import "github.com/upamune/tla-go/syntax/parser"

func (r *instanceResolver) validateBoundedConstantArguments(module *parser.Module, instance parser.Instance, replacements map[string]*parser.Expression, limits map[string]int, model *model) error {
	if instance.Name == "" || len(instance.Params) == 0 || len(limits) == 0 {
		return nil
	}
	for _, level := range []int{1, 2} {
		declarations := constantsWithLevelLimit(module.Constants, limits, level)
		if len(declarations) == 0 {
			continue
		}
		check := stateInstanceCheck{
			model: model, targets: instanceTargetsForDeclarations(module, declarations, instance, replacements, level, false),
			requirements: map[string][]bool{}, maxLevel: level,
		}
		if err := check.validate(r.root); err != nil {
			return err
		}
	}
	return nil
}

func constantsWithLevelLimit(constants []parser.Declaration, limits map[string]int, level int) []parser.Declaration {
	var result []parser.Declaration
	for _, declaration := range constants {
		if declaration.Arity == 0 && limits[declaration.Name] == level {
			result = append(result, declaration)
		}
	}
	return result
}
