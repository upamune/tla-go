package checker

import (
	"fmt"
	"maps"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (g *generator) functionAction(item work, rest []work, partial evaluator.State, action string) (bool, bool, error) {
	expr := item.expr
	if len(expr.Args) < 2 || (expr.Args[0].Kind != "Identifier" && expr.Args[0].Kind != "Apply") {
		return false, false, nil
	}
	name := expr.Args[0].Text
	if g.hasLocalValue(name, item.locals) {
		return false, false, nil
	}
	definition, ok := g.definition(name)
	if !ok || definition.Kind != "Function" {
		return false, false, nil
	}
	more, err := g.checkedFunctionAction(item, definition, rest, partial, action)
	return true, more, err
}

func (g *generator) checkedFunctionAction(item work, definition parser.Definition, rest []work, partial evaluator.State, action string) (bool, error) {
	if len(definition.Params) != len(item.expr.Args[0].Args) || hasOperatorParameters(definition.Params) {
		return false, fmt.Errorf("unsupported function action arguments for %s", definition.Name)
	}
	return g.expandFunctionAction(item, definition, rest, partial, action)
}

func (g *generator) expandFunctionAction(item work, definition parser.Definition, rest []work, partial evaluator.State, action string) (bool, error) {
	if item.path.contains(definition.Name) {
		return false, fmt.Errorf("recursive action expansion: %s", definition.Name)
	}
	bindings, err := g.functionWitnesses(item, definition, partial)
	if err != nil {
		return false, err
	}
	path := item.path.withName(definition.Name)
	body := work{expr: definition.Body, path: path, locals: bindings}
	return g.solve(prepend(body, rest), partial, action)
}

func (g *generator) functionArgument(args []*parser.Expression, current, next evaluator.State, locals map[string]value.Value) (value.Value, error) {
	values := make([]value.Value, len(args))
	for i, arg := range args {
		v, err := g.evalWithLocals(arg, current, next, locals)
		if err != nil {
			return value.Value{}, err
		}
		values[i] = v
	}
	if len(values) == 1 {
		return values[0], nil
	}
	return value.Tuple(values)
}

func bindFunctionWitnesses(bounds []parser.Bound, argument value.Value, locals map[string]value.Value) (map[string]value.Value, error) {
	values := []value.Value{argument}
	if len(bounds) != 1 {
		var err error
		values, err = argument.TupleElements()
		if err != nil {
			return nil, err
		}
	}
	if len(bounds) != len(values) {
		return nil, fmt.Errorf("function argument has wrong arity")
	}
	bindings := locals
	for i, bound := range bounds {
		var err error
		bindings, err = bindBound(bound, values[i], bindings, "tuple binding has wrong arity")
		if err != nil {
			return nil, err
		}
	}
	return bindings, nil
}

func (g *generator) functionWitnesses(item work, definition parser.Definition, partial evaluator.State) (map[string]value.Value, error) {
	current, next := g.context(partial)
	fn, err := g.evalWithLocals(item.expr.Args[0], current, next, item.locals)
	if err != nil {
		return nil, err
	}
	argument, err := g.functionArgument(item.expr.Args[1:], current, next, item.locals)
	if err != nil {
		return nil, err
	}
	domain, err := fn.FunctionDomain()
	if err != nil {
		return nil, err
	}
	found, err := domain.Contains(argument)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("argument is outside the function's domain")
	}
	locals, err := g.functionParameterBindings(item, definition, current, next)
	if err != nil {
		return nil, err
	}
	bindings, err := bindFunctionWitnesses(definition.Bounds, argument, locals)
	if err != nil {
		return nil, err
	}
	return bindings, nil
}

func (g *generator) functionParameterBindings(item work, definition parser.Definition, current, next evaluator.State) (map[string]value.Value, error) {
	if len(definition.Params) == 0 {
		return item.locals, nil
	}
	bindings := maps.Clone(item.locals)
	if bindings == nil {
		bindings = map[string]value.Value{}
	}
	for i, parameter := range definition.Params {
		v, err := g.evalWithLocals(item.expr.Args[0].Args[i], current, next, item.locals)
		if err != nil {
			return nil, err
		}
		bindings[parameter.Name] = v
	}
	return bindings, nil
}
