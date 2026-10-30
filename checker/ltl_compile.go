package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

func (c *ltlCompiler) compileTemporal(expr *parser.Expression, negative bool, seen map[string]bool) (*ltlFormula, error) {
	switch expr.Kind {
	case "Always", "Eventually":
		if formula, ok := c.compileRecurrentAction(expr, negative); ok {
			return formula, nil
		}
		return c.compileUnary(expr, negative, seen)
	case "And", "Or":
		kind := ltlAnd
		if (expr.Kind == "Or") != negative {
			kind = ltlOr
		}
		return c.compileJunction(kind, expr.Args, negative, seen)
	case "Apply":
		return c.compileOperator(expr, negative, seen)
	case "If":
		return c.compileConditional(expr, negative, seen)
	case "Forall", "Exists":
		return c.compileQuantifier(expr, negative, seen)
	default:
		return c.compileOtherTemporal(expr, negative, seen)
	}
}

func (c *ltlCompiler) compileOtherTemporal(expr *parser.Expression, negative bool, seen map[string]bool) (*ltlFormula, error) {
	if expr.Kind == "Let" {
		return c.compileLet(expr, negative, seen)
	}
	if expr.Kind == "WeakFairness" || expr.Kind == "StrongFairness" {
		return c.compileFairness(expr, negative)
	}
	return nil, fmt.Errorf("unsupported temporal expression %s", expr.Kind)
}

func (c *ltlCompiler) compileConditional(expr *parser.Expression, negative bool, seen map[string]bool) (*ltlFormula, error) {
	if len(expr.Args) != 3 {
		return nil, fmt.Errorf("invalid temporal conditional")
	}
	constant, known, err := c.constantCondition(expr.Args[0])
	if err != nil {
		return nil, err
	}
	if known {
		return c.compileConditionalBranch(expr, negative, seen, constant)
	}
	return c.compileConditionalFormula(expr, negative, seen)
}

func (c *ltlCompiler) compileConditionalBranch(expr *parser.Expression, negative bool, seen map[string]bool, constant bool) (*ltlFormula, error) {
	branch := 2
	if constant {
		branch = 1
	}
	return c.compile(expr.Args[branch], negative, seen)
}

func (c *ltlCompiler) compileConditionalFormula(expr *parser.Expression, negative bool, seen map[string]bool) (*ltlFormula, error) {
	condition, err := c.compile(expr.Args[0], false, seen)
	if err != nil {
		return nil, err
	}
	otherwise, err := c.compile(expr.Args[0], true, seen)
	if err != nil {
		return nil, err
	}
	then, err := c.compile(expr.Args[1], negative, seen)
	if err != nil {
		return nil, err
	}
	other, err := c.compile(expr.Args[2], negative, seen)
	if err != nil {
		return nil, err
	}
	left := c.intern(ltlAnd, condition, then, nil, false)
	right := c.intern(ltlAnd, otherwise, other, nil, false)
	return c.intern(ltlOr, left, right, nil, false), nil
}

func (c *ltlCompiler) constantCondition(expr *parser.Expression) (bool, bool, error) {
	if c.model.readsState(expr, map[string]bool{}) || c.model.hasAction(expr, map[string]bool{}) {
		return false, false, nil
	}
	result, err := c.model.program.Evaluate(c.wrapLexical(expr), evaluator.Context{Constants: c.model.constants, Locals: c.locals, Enabled: c.model.enabledCallback})
	if err != nil {
		return false, false, err
	}
	value, ok := result.Bool()
	return value, ok, nil
}

func (c *ltlCompiler) compileRecurrentAction(expr *parser.Expression, negative bool) (*ltlFormula, bool) {
	action := recurrentAction(expr)
	if action == nil || c.model.hasTemporal(action, map[string]bool{}) || c.model.hasAction(action.Args[1], map[string]bool{}) {
		return nil, false
	}
	inner := expr.Args[0]
	formula := c.intern(ltlAction, nil, nil, action, negative)
	innerKind, outerKind := ltlAlways, ltlAlways
	if (inner.Kind == "Eventually") != negative {
		innerKind = ltlEventually
	}
	if (expr.Kind == "Eventually") != negative {
		outerKind = ltlEventually
	}
	formula = c.intern(innerKind, formula, nil, nil, false)
	return c.intern(outerKind, formula, nil, nil, false), true
}

func recurrentAction(expr *parser.Expression) *parser.Expression {
	if len(expr.Args) != 1 {
		return nil
	}
	inner := expr.Args[0]
	if len(inner.Args) != 1 || expr.Kind == inner.Kind || (inner.Kind != "Always" && inner.Kind != "Eventually") {
		return nil
	}
	action := inner.Args[0]
	if len(action.Args) != 2 || !isRecurrentAction(expr.Kind, inner.Kind, action.Kind) {
		return nil
	}
	return action
}

func isRecurrentAction(outer, inner, action string) bool {
	return outer == "Always" && inner == "Eventually" && action == "AngleAction" || outer == "Eventually" && inner == "Always" && action == "BoxAction"
}

func (c *ltlCompiler) compileUnary(expr *parser.Expression, negative bool, seen map[string]bool) (*ltlFormula, error) {
	if len(expr.Args) != 1 {
		return nil, fmt.Errorf("invalid temporal expression %s", expr.Kind)
	}
	body, err := c.compile(expr.Args[0], negative, seen)
	if err != nil {
		return nil, err
	}
	kind := ltlAlways
	if (expr.Kind == "Eventually") != negative {
		kind = ltlEventually
	}
	if body.kind == kind && hasFairnessAction(body) {
		return body, nil
	}
	return c.intern(kind, body, nil, nil, false), nil
}

func (c *ltlCompiler) compileJunction(kind ltlKind, args []*parser.Expression, negative bool, seen map[string]bool) (*ltlFormula, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("empty temporal junction")
	}
	result, err := c.compile(args[0], negative, seen)
	if err != nil {
		return nil, err
	}
	for _, arg := range args[1:] {
		right, err := c.compile(arg, negative, seen)
		if err != nil {
			return nil, err
		}
		result = c.intern(kind, result, right, nil, false)
	}
	return result, nil
}

func (c *ltlCompiler) compileOperator(expr *parser.Expression, negative bool, seen map[string]bool) (*ltlFormula, error) {
	if expr.Text == "~" && len(expr.Args) == 1 {
		return c.compile(expr.Args[0], !negative, seen)
	}
	return c.compileBinaryOperator(expr, negative, seen)
}

func (c *ltlCompiler) compileBinaryOperator(expr *parser.Expression, negative bool, seen map[string]bool) (*ltlFormula, error) {
	if len(expr.Args) != 2 {
		return nil, fmt.Errorf("unsupported temporal operator %s", expr.Text)
	}
	switch expr.Text {
	case "/\\", "\\/":
		kind := ltlAnd
		if (expr.Text == "\\/") != negative {
			kind = ltlOr
		}
		return c.compileJunction(kind, expr.Args, negative, seen)
	case "=>":
		return c.compileImplication(expr.Args[0], expr.Args[1], negative, seen)
	case "~>":
		return c.compileLeadsTo(expr.Args[0], expr.Args[1], negative, seen)
	default:
		return nil, fmt.Errorf("unsupported temporal operator %s", expr.Text)
	}
}

func (c *ltlCompiler) compileImplication(a, b *parser.Expression, negative bool, seen map[string]bool) (*ltlFormula, error) {
	left, err := c.compile(a, !negative, seen)
	if err != nil {
		return nil, err
	}
	right, err := c.compile(b, negative, seen)
	if err != nil {
		return nil, err
	}
	kind := ltlOr
	if negative {
		kind = ltlAnd
	}
	return c.intern(kind, left, right, nil, false), nil
}

func (c *ltlCompiler) compileLeadsTo(a, b *parser.Expression, negative bool, seen map[string]bool) (*ltlFormula, error) {
	left, err := c.compile(a, !negative, seen)
	if err != nil {
		return nil, err
	}
	right, err := c.compile(b, negative, seen)
	if err != nil {
		return nil, err
	}
	inner, outer, junction := ltlEventually, ltlAlways, ltlOr
	if negative {
		inner, outer, junction = ltlAlways, ltlEventually, ltlAnd
	}
	right = c.intern(inner, right, nil, nil, false)
	body := c.intern(junction, left, right, nil, false)
	return c.intern(outer, body, nil, nil, false), nil
}
