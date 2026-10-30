package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

func (g *generator) expandCall(item work, rest []work, partial evaluator.State, action string) (bool, error) {
	name := item.expr.Text
	if item.path.contains(name) {
		return false, fmt.Errorf("recursive action expansion: %s", name)
	}
	expanded, err := g.actionBody(item.expr)
	if err != nil {
		return false, err
	}
	return g.expandWork(item, expanded, rest, partial, action)
}

func (g *generator) actionBody(call *parser.Expression) (*parser.Expression, error) {
	if body, ok := g.model.actionCache[call]; ok && g.lexical == nil {
		return body, nil
	}
	d, ok := g.definition(call.Text)
	if !ok {
		return nil, fmt.Errorf("undefined action %s", call.Text)
	}
	if d.Kind != "Operator" || len(d.Params) != len(call.Args) {
		return nil, fmt.Errorf("invalid action call %s", call.Text)
	}
	body, err := substituteActionCall(d, call)
	if err != nil {
		return nil, err
	}
	if g.lexical == nil {
		g.model.actionCache[call] = body
	}
	return body, nil
}

func substituteActionCall(d parser.Definition, call *parser.Expression) (*parser.Expression, error) {
	replacements, operators, err := actionArguments(d.Params, call.Args)
	if err != nil {
		return nil, err
	}
	if err = validateOperatorCalls(d.Body, operators, replacements); err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, arg := range call.Args {
		collectArgumentNames(arg, names)
	}
	if captures(d.Body, names) {
		return nil, fmt.Errorf("action call %s requires capture-avoiding substitution", call.Text)
	}
	return substitute(d.Body, replacements), nil
}

func actionArguments(params []parser.Declaration, args []*parser.Expression) (map[string]*parser.Expression, map[string]*parser.Expression, error) {
	replacements := make(map[string]*parser.Expression, len(params))
	operators := make(map[string]*parser.Expression)
	for i, param := range params {
		if param.Arity > 0 {
			if err := validActionOperator(args[i], param.Arity); err != nil {
				return nil, nil, err
			}
			operators[param.Name] = args[i]
		}
		replacements[param.Name] = args[i]
	}
	return replacements, operators, nil
}
