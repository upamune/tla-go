package checker

import (
	"fmt"
	"maps"

	"github.com/upamune/tla-go/syntax/parser"
)

// metadataOperator retains the caller's scope so that level restrictions of
// an operator argument can be checked when a higher-order parameter uses it.
type metadataOperator struct {
	expr  *parser.Expression
	scope *metadataScope
}

func specialOperatorReferenceLevel(name string) (int, bool) {
	if kind, ok := parser.SpecialOperatorKind(name); ok {
		level, _ := metadataHeadLevel(&parser.Expression{Kind: kind}, 0)
		return level, true
	}
	switch name {
	case "~>", "-+->":
		return 3, true
	case "\\cdot":
		return 2, true
	default:
		return 0, false
	}
}

func (s *metadataScope) addOperatorBindings(operators map[string]metadataOperator) {
	if len(operators) == 0 {
		return
	}
	if s.operators == nil {
		s.operators = make(map[string]metadataOperator, len(operators))
	}
	maps.Copy(s.operators, operators)
}

func (s *metadataScope) operatorArguments(params []parser.Declaration, args []*parser.Expression) map[string]metadataOperator {
	var result map[string]metadataOperator
	for i, param := range params {
		if param.Arity > 0 && i < len(args) {
			if result == nil {
				result = make(map[string]metadataOperator)
			}
			result[param.Name] = s.operatorArgument(args[i])
		}
	}
	return result
}

func (s *metadataScope) operatorArgument(arg *parser.Expression) metadataOperator {
	if bound, ok := s.operators[arg.Text]; ok && (arg.Kind == "Identifier" || arg.Kind == "OperatorReference") {
		return bound
	}
	return metadataOperator{expr: arg, scope: s}
}

func (s *metadataScope) boundOperatorCall(operator metadataOperator, expr *parser.Expression, visiting map[*metadataDefinition]bool) (int, error) {
	args := make([]int, len(expr.Args))
	level := s.values[expr.Text]
	for i, arg := range expr.Args {
		v, err := s.expression(arg, visiting)
		if err != nil {
			return 0, err
		}
		args[i] = v
		level = max(level, v)
	}
	return level, operator.validate(args, visiting)
}

func (o metadataOperator) validate(args []int, visiting map[*metadataDefinition]bool) error {
	probe := o.scope.clone()
	probe.instanceProbe = true
	if o.expr.Kind == "Lambda" {
		return probe.validateOperatorLambda(o.expr, args, visiting)
	}
	if d, ok := probe.defs[o.expr.Text]; ok {
		_, err := probe.definition(d, args, visiting)
		return err
	}
	if _, abstract := probe.values[o.expr.Text]; abstract {
		return nil
	}
	return validateBuiltinOperatorLevel(o.expr.Text, args, probe.tlc || o.expr.BuiltinTLC)
}

func (s *metadataScope) validateOperatorLambda(expr *parser.Expression, args []int, visiting map[*metadataDefinition]bool) error {
	if len(expr.Names) != len(args) || len(expr.Args) != 1 {
		return fmt.Errorf("invalid operator lambda")
	}
	for i, name := range expr.Names {
		s.bind(name, args[i])
	}
	_, err := s.expression(expr.Args[0], visiting)
	return err
}

func validateBuiltinOperatorLevel(name string, args []int, tlc bool) error {
	level := 0
	for _, arg := range args {
		level = max(level, arg)
	}
	if tlc && metadataTLCHigherOrder(name) {
		return validateTLCHigherOrderLevel(name, args, level)
	}
	if kind, ok := parser.SpecialOperatorKind(name); ok {
		_, err := metadataHeadLevel(&parser.Expression{Kind: kind}, level)
		return err
	}
	probe := &metadataScope{instanceProbe: true}
	if name == "/\\" || name == "\\/" {
		return probe.checkInstanceLevel(level, 3)
	}
	return probe.checkInstanceLevel(level, instanceExpressionLimit(&parser.Expression{Kind: "Apply", Text: name}))
}

func metadataTLCHigherOrder(name string) bool {
	switch name {
	case "Print", "PrintT", "TLCGet", "TLCSet", "Assert":
		return true
	default:
		return false
	}
}

func validateTLCHigherOrderLevel(name string, args []int, level int) error {
	probe := &metadataScope{instanceProbe: true}
	if name == "Assert" && len(args) > 0 {
		if err := probe.checkInstanceLevel(args[0], 2); err != nil {
			return err
		}
	}
	return probe.checkInstanceLevel(level, 3)
}
