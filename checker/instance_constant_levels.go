package checker

import (
	"errors"

	"github.com/upamune/tla-go/syntax/parser"
)

// instanceLevelConstraint distinguishes a level limit from an unsupported
// metadata expression, for which the existing conservative checks apply.
type instanceLevelConstraint struct{}

func (instanceLevelConstraint) Error() string { return "instance level constraint" }

func (s *metadataScope) checkInstanceLevel(level, limit int) error {
	if s.instanceProbe && level > limit {
		return instanceLevelConstraint{}
	}
	return nil
}

func instanceConstantModule(module *parser.Module, scope *metadataScope) bool {
	for _, definition := range module.Definitions {
		level, err := scope.definition(scope.defs[definition.Name], nil, map[*metadataDefinition]bool{})
		if err != nil || level != 0 {
			return false
		}
	}
	for _, theorem := range module.Theorems {
		level, err := scope.expression(theorem.Statement, map[*metadataDefinition]bool{})
		if err != nil || level != 0 {
			return false
		}
	}
	return true
}

func instanceConstantLimits(module *parser.Module) (map[string]int, bool) {
	if len(module.Variables) != 0 {
		return nil, false
	}
	scope := newMetadataScope(module)
	if !instanceConstantModule(module, scope) {
		return nil, false
	}
	scope.instanceProbe = true
	limits := make(map[string]int)
	for _, declaration := range module.Constants {
		if declaration.Arity != 0 {
			continue
		}
		limit, ok := instanceConstantLimit(module, scope, declaration.Name)
		if !ok {
			return nil, false
		}
		limits[declaration.Name] = limit
	}
	return limits, true
}

func instanceConstantLimit(module *parser.Module, scope *metadataScope, name string) (int, bool) {
	original := scope.values[name]
	defer func() { scope.values[name] = original }()
	for level := 3; level > 0; level-- {
		scope.values[name] = level
		err := probeInstanceModule(module, scope)
		if err == nil {
			return level, true
		}
		if _, ok := errors.AsType[instanceLevelConstraint](err); !ok {
			return 0, false
		}
	}
	return 0, true
}

func probeInstanceModule(module *parser.Module, scope *metadataScope) error {
	for _, definition := range module.Definitions {
		if _, err := scope.definition(scope.defs[definition.Name], nil, map[*metadataDefinition]bool{}); err != nil {
			return err
		}
	}
	for _, assumption := range module.Assumptions {
		if _, err := scope.expression(assumption.Expression, map[*metadataDefinition]bool{}); err != nil {
			return err
		}
	}
	for _, theorem := range module.Theorems {
		if _, err := scope.expression(theorem.Statement, map[*metadataDefinition]bool{}); err != nil {
			return err
		}
	}
	return nil
}

func instanceExpressionLimit(expr *parser.Expression) int {
	switch expr.Kind {
	case "Let", "Label":
		return 4
	case "Apply":
		switch expr.Text {
		case "~", "=>", "<=>", "\\equiv", "~>", "-+->":
			return 3
		default:
			return 2
		}
	case "SetEnumeration", "SetFilter", "SetMap", "Tuple", "Product", "RecordSet", "FunctionSet", "FunctionApplication", "FieldAccess", "Except", "Choose":
		return 2
	default:
		return 3
	}
}
