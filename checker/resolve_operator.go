package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func (r *resolver) expressionArguments(expr *parser.Expression, scope map[string]symbol) error {
	params := r.argumentParameters(expr, scope)
	for i, arg := range expr.Args {
		if i < len(params) && params[i] > 0 {
			if err := r.operatorArgument(arg, params[i], scope); err != nil {
				return err
			}
			continue
		}
		if err := r.expression(arg, scope); err != nil {
			return err
		}
	}
	return nil
}

func (r *resolver) argumentParameters(expr *parser.Expression, scope map[string]symbol) []int {
	var params []int
	if expr.BuiltinTLC && expr.Text == "!tlc!SortSeq" && len(expr.Args) > 0 {
		params = make([]int, len(expr.Args))
		params[len(params)-1] = 2
	} else if expr.Kind == "Apply" {
		sym, _ := r.lookup(expr.Text, scope)
		params = sym.params
	}
	return params
}

func (r *resolver) operatorArgument(expr *parser.Expression, arity int, scope map[string]symbol) error {
	previous := r.reference
	r.reference = sourceSymbolLocation(expr.Location)
	defer func() { r.reference = previous }()
	if expr.Kind == "Lambda" {
		return r.lambdaArgument(expr, arity, scope)
	}
	if expr.Kind != "Identifier" && expr.Kind != "OperatorReference" {
		return fmt.Errorf("expected an operator argument, got %s %q at line %d, column %d", expr.Kind, expr.Text, expr.Location.Line, expr.Location.Column)
	}
	name := expr.Text
	sym, ok := r.operatorSymbol(name, scope)
	if !ok && !r.visibleGlobal(name) {
		return &SemanticError{Err: fmt.Errorf("undefined symbol %s", name)}
	}
	return validateOperatorArgument(name, arity, sym, ok)
}

func validateOperatorArgument(name string, arity int, sym symbol, ok bool) error {
	if !ok || sym.arity != arity {
		return fmt.Errorf("operator %s must have arity %d", name, arity)
	}
	for _, param := range sym.params {
		if param > 0 {
			return fmt.Errorf("higher-order operator %s cannot be passed as an argument", name)
		}
	}
	return nil
}

func (r *resolver) operatorSymbol(name string, scope map[string]symbol) (symbol, bool) {
	if sym, ok := r.lookup(name, scope); ok {
		return sym, true
	}
	if arity, ok := parser.SpecialOperatorArity(name); ok {
		return symbol{arity: arity}, true
	}
	return symbol{}, false
}

func (r *resolver) lambdaArgument(expr *parser.Expression, arity int, scope map[string]symbol) error {
	if len(expr.Names) != arity {
		return fmt.Errorf("lambda must have arity %d", arity)
	}
	local := cloneScope(scope)
	if err := r.bound(parser.Bound{Names: expr.Names}, local); err != nil {
		return err
	}
	return r.expression(expr.Args[0], local)
}
