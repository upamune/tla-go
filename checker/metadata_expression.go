package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func (s *metadataScope) expression(expr *parser.Expression, visiting map[*metadataDefinition]bool) (int, error) {
	if expr == nil {
		return 0, nil
	}
	if level, handled, err := s.reference(expr, visiting); handled {
		return level, err
	}
	if expr.Kind == "OperatorReference" || expr.Kind == "Subexpression" || expr.Kind == "ModuleFact" {
		return 0, fmt.Errorf("unsupported level expression %s", expr.Kind)
	}
	local, level, err := s.expressionScope(expr, visiting)
	if err != nil {
		return 0, err
	}
	childLevel, err := local.childrenLevel(expr, visiting)
	if err != nil {
		return 0, err
	}
	return metadataHeadLevel(expr, max(level, childLevel))
}

func (s *metadataScope) reference(expr *parser.Expression, visiting map[*metadataDefinition]bool) (int, bool, error) {
	if expr.Kind == "AssumeProve" {
		level, err := s.assumeProveLevel(expr, visiting)
		return level, true, err
	}
	if expr.Kind != "Identifier" && expr.Kind != "Apply" {
		return 0, false, nil
	}
	if d, ok := s.defs[expr.Text]; ok {
		level, err := s.call(d, expr, visiting)
		return level, true, err
	}
	if level, handled, err := s.builtinTLCLevel(expr, visiting); handled {
		return level, true, err
	}
	if level, ok := s.values[expr.Text]; ok {
		return s.declaredValueLevel(expr, level, visiting)
	}
	return 0, false, nil
}

func (s *metadataScope) declaredValueLevel(expr *parser.Expression, level int, visiting map[*metadataDefinition]bool) (int, bool, error) {
	if expr.Kind == "Identifier" {
		return level, true, nil
	}
	children, err := s.childrenLevel(expr, visiting)
	return max(level, children), true, err
}

// SANY determines these operators' levels from their TLA+ bodies, including
// ignored arguments, rather than from TLC's Java override implementations.
func (s *metadataScope) builtinTLCLevel(expr *parser.Expression, visiting map[*metadataDefinition]bool) (int, bool, error) {
	if !s.isBuiltinTLC(expr) {
		return 0, false, nil
	}
	switch tlcOperatorName(expr) {
	case "PrintT", "TLCGet", "TLCSet":
		return s.tlcArgumentLevel(expr, -1, visiting)
	case "Print":
		return s.tlcArgumentLevel(expr, 1, visiting)
	case "Assert":
		return s.tlcArgumentLevel(expr, 0, visiting)
	}
	return 0, false, nil
}

func (s *metadataScope) isBuiltinTLC(expr *parser.Expression) bool {
	if expr.Kind != "Apply" || !(s.tlc || expr.BuiltinTLC) {
		return false
	}
	_, shadowed := s.values[expr.Text]
	return !shadowed || expr.BuiltinTLC
}

func (s *metadataScope) tlcArgumentLevel(expr *parser.Expression, index int, visiting map[*metadataDefinition]bool) (int, bool, error) {
	if index >= len(expr.Args) {
		return 0, true, fmt.Errorf("missing TLC operator argument")
	}
	level := 0
	for i, arg := range expr.Args {
		argumentLevel, err := s.expression(arg, visiting)
		if err != nil {
			return 0, true, err
		}
		if i == index {
			level = argumentLevel
		}
	}
	return level, true, nil
}

func (s *metadataScope) expressionScope(expr *parser.Expression, visiting map[*metadataDefinition]bool) (*metadataScope, int, error) {
	if !metadataNeedsScope(expr) {
		return s, 0, nil
	}
	local := s.clone()
	if expr.Kind == "Let" {
		local.addDefinitions(expr.Definitions)
	}
	level, err := local.bounds(expr.Bounds, visiting)
	if err != nil {
		return nil, 0, err
	}
	local.bindExpressionNames(expr)
	return local, level, nil
}

func metadataNeedsScope(expr *parser.Expression) bool {
	return len(expr.Bounds) > 0 || expr.Kind == "Let" || expr.Kind == "Lambda" || expr.Kind == "UnboundedQuantifier"
}

func (s *metadataScope) bindExpressionNames(expr *parser.Expression) {
	if expr.Kind == "Lambda" || expr.Kind == "UnboundedQuantifier" {
		for _, name := range expr.Names {
			s.bind(name, 0)
		}
	}
}

func (s *metadataScope) childrenLevel(expr *parser.Expression, visiting map[*metadataDefinition]bool) (int, error) {
	copy := *expr
	copy.Bounds, copy.Definitions = nil, nil
	level := 0
	var err error
	walkChildren(&copy, func(child *parser.Expression) {
		if err == nil {
			var childLevel int
			childLevel, err = s.expression(child, visiting)
			level = max(level, childLevel)
		}
	})
	return level, err
}

func (s *metadataScope) call(d *metadataDefinition, expr *parser.Expression, visiting map[*metadataDefinition]bool) (int, error) {
	args := make([]int, len(expr.Args))
	for i, arg := range expr.Args {
		level, err := s.expression(arg, visiting)
		if err != nil {
			return 0, err
		}
		args[i] = level
	}
	return s.definition(d, args, visiting)
}

func metadataHeadLevel(expr *parser.Expression, children int) (int, error) {
	switch expr.Kind {
	case "Enabled":
		return metadataEnabledLevel(children)
	case "Prime", "Unchanged":
		return metadataStateHeadLevel(expr.Kind, children)
	case "BoxAction", "AngleAction":
		return max(2, children), nil
	case "Always", "Eventually", "WeakFairness", "StrongFairness":
		return 3, nil
	case "Apply":
		if expr.Text == "~>" || expr.Text == "-+->" {
			return 3, nil
		}
	}
	return children, nil
}

func metadataEnabledLevel(children int) (int, error) {
	if children > 2 {
		return 0, fmt.Errorf("ENABLED cannot contain a temporal expression")
	}
	return 1, nil
}

func metadataStateHeadLevel(kind string, children int) (int, error) {
	if children > 1 {
		return 0, fmt.Errorf("%s requires a constant or state expression", kind)
	}
	return 2, nil
}
