package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func (r *resolver) expressionArguments(expr *parser.Expression, scope map[string]symbol) error {
	var params []int
	if expr.Kind == "Apply" {
		sym, _ := r.lookup(expr.Text, scope)
		params = sym.params
	}
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

func (r *resolver) operatorArgument(expr *parser.Expression, arity int, scope map[string]symbol) error {
	if expr.Kind == "Lambda" {
		return r.lambdaArgument(expr, arity, scope)
	}
	if expr.Kind != "Identifier" && expr.Kind != "OperatorReference" {
		return fmt.Errorf("expected an operator argument")
	}
	name := expr.Text
	sym, ok := r.lookup(name, scope)
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
