package checker

import (
	"strconv"

	"github.com/upamune/tla-go/syntax/parser"
)

func (s *subexpressionSelection) selectOperand(selector parser.Selector) error {
	if selector.Kind == "Colon" {
		return s.selectColon()
	}
	for s.current != nil && s.current.Kind == "Label" {
		s.current = s.current.Args[0]
	}
	if s.current == nil {
		return selectionError(s.location, "Subexpression selection failed")
	}
	if s.current.Kind == "Let" {
		return s.selectLetOperand(selector)
	}
	if selector.Kind == "Bound" || selector.Kind == "At" {
		return s.selectBoundOperand(selector)
	}
	return s.selectNumberedOperand(selector)
}

func (s *subexpressionSelection) selectColon() error {
	if s.current != nil && s.current.Kind == "AssumeProve" {
		return selectionError(s.location, "ASSUME/PROVE is not an expression")
	}
	return nil
}

func (s *subexpressionSelection) selectNumberedOperand(selector parser.Selector) error {
	if selector.Kind != "Operand" {
		return selectionError(s.location, "Unexpected subexpression selector")
	}
	if s.current.Kind == "Case" {
		return s.selectCaseOperand(selector)
	}
	operands := subexpressionOperands(s.current)
	index := subexpressionOperandIndex(selector.Operand, len(operands))
	if index < 0 {
		return selectionError(s.location, "Nonexistent operand specified by the selector")
	}
	s.current = operands[index]
	return nil
}

func (s *subexpressionSelection) selectLetOperand(selector parser.Selector) error {
	if selector.Kind != "Operand" || subexpressionOperandIndex(selector.Operand, 1) != 0 {
		return selectionError(s.location, "A LET/IN expression has only one operand")
	}
	s.lets = append(s.lets, s.current.Definitions)
	s.current = s.current.Args[0]
	return nil
}

func (s *subexpressionSelection) selectBoundOperand(selector parser.Selector) error {
	if !isSubexpressionBinder(s.current) {
		return selectionError(s.location, "Unexpected subexpression selector")
	}
	var names []string
	for _, bound := range s.current.Bounds {
		names = append(names, bound.Names...)
	}
	if s.current.Kind == "Lambda" || s.current.Kind == "UnboundedQuantifier" {
		names = s.current.Names
	}
	for _, name := range names {
		s.params = append(s.params, parser.Declaration{Name: name})
	}
	if selector.Kind == "Bound" {
		s.args = append(s.args, s.takeArguments(selector.Args, len(names))...)
	}
	s.current = s.current.Args[0]
	return nil
}

func isSubexpressionBinder(expr *parser.Expression) bool {
	switch expr.Kind {
	case "Forall", "Exists", "FunctionConstruction", "SetMap", "SetFilter", "Choose", "Lambda", "UnboundedQuantifier":
		return len(expr.Args) > 0
	}
	return false
}

func subexpressionOperands(expr *parser.Expression) []*parser.Expression {
	if isSubexpressionBinder(expr) {
		return subexpressionBoundSets(expr.Bounds)
	}
	switch expr.Kind {
	case "Apply", "And", "Or", "If", "Tuple", "SetEnumeration", "Prime", "Unchanged", "Enabled", "Always", "Eventually", "BoxAction", "AngleAction", "WeakFairness", "StrongFairness", "FunctionSet", "Product", "AssumeProve":
		return expr.Args
	case "FunctionApplication":
		return subexpressionFunctionOperands(expr)
	case "FieldAccess":
		return []*parser.Expression{expr.Args[0], {Kind: "String", Text: expr.Text, Location: expr.Location}}
	case "Record", "RecordSet":
		var result []*parser.Expression
		for _, field := range expr.Fields {
			result = append(result, field.Value)
		}
		return result
	case "Except":
		return expr.Args[:1]
	}
	return nil
}

func subexpressionFunctionOperands(expr *parser.Expression) []*parser.Expression {
	if len(expr.Args) <= 2 {
		return expr.Args
	}
	tuple := &parser.Expression{Kind: "Tuple", Args: expr.Args[1:], Location: expr.Location}
	return []*parser.Expression{expr.Args[0], tuple}
}

func subexpressionBoundSets(bounds []parser.Bound) []*parser.Expression {
	var result []*parser.Expression
	for i, bound := range bounds {
		if bound.Set != nil && (i == 0 || bound.Set != bounds[i-1].Set) {
			result = append(result, bound.Set)
		}
	}
	return result
}

func subexpressionOperandIndex(operand string, count int) int {
	if operand == "<<" && count > 0 {
		return 0
	}
	if operand == ">>" && count == 2 {
		return 1
	}
	n, err := strconv.Atoi(operand)
	if err != nil || n < 1 || n > count {
		return -1
	}
	return n - 1
}

func (s *subexpressionSelection) selectCaseOperand(selector parser.Selector) error {
	arms := s.current.Arms
	if len(s.current.Args) != 0 {
		arms = append(append([]parser.CaseArm(nil), arms...), parser.CaseArm{Body: s.current.Args[0]})
	}
	index := subexpressionOperandIndex(selector.Operand, len(arms))
	if index < 0 || s.index+1 >= len(s.selectors) || s.selectors[s.index+1].Kind != "Operand" {
		return selectionError(s.location, "A subexpression of CASE must have the form !i!j")
	}
	s.index++
	part := subexpressionOperandIndex(s.selectors[s.index].Operand, 2)
	if part == 0 {
		s.current = arms[index].Condition
	} else if part == 1 {
		s.current = arms[index].Body
	} else {
		s.current = nil
	}
	if s.current == nil {
		return selectionError(s.location, "Nonexistent CASE operand")
	}
	return nil
}
