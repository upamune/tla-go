package checker

import (
	"fmt"
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

func (g *generator) functionActionDefinition(definition parser.Definition, call *parser.Expression) (parser.Definition, error) {
	if !hasOperatorParameters(definition.Params) {
		return definition, nil
	}
	cacheable := g.cacheableFunctionAction(call, definition)
	if cached, ok := g.model.funcActionCache[call]; cacheable && ok {
		return cached, nil
	}
	expanded, err := substituteFunctionOperators(definition, call)
	if err == nil && cacheable {
		g.cacheFunctionAction(call, expanded)
	}
	return expanded, err
}

func (g *generator) cacheFunctionAction(call *parser.Expression, definition parser.Definition) {
	if g.lexical != nil && len(g.model.funcActionCache) >= 4096 {
		return
	}
	if g.model.funcActionCache == nil {
		g.model.funcActionCache = make(map[*parser.Expression]parser.Definition)
	}
	g.model.funcActionCache[call] = definition
}

func (g *generator) cacheableFunctionAction(call *parser.Expression, definition parser.Definition) bool {
	if !g.cacheableActionBody(call, definition) {
		return false
	}
	if g.lexical == nil {
		return true
	}
	root := g.model.defs[call.Text]
	return root.Kind == definition.Kind && slices.EqualFunc(root.Bounds, definition.Bounds, sameFunctionBound)
}

func sameFunctionBound(a, b parser.Bound) bool {
	return a.Set == b.Set && a.Tuple == b.Tuple && slices.Equal(a.Names, b.Names)
}

// Operator-valued instance parameters cannot be stored in the generator's
// value bindings. Substitute them into an alpha-renamed action instead.
func substituteFunctionOperators(definition parser.Definition, call *parser.Expression) (parser.Definition, error) {
	if !hasOperatorParameters(definition.Params) {
		return definition, nil
	}
	renamer := alphaRenamer{prefix: fmt.Sprintf("function-action-%p", call)}
	definition = renamer.definition(definition, nil)
	operators := make(map[string]*parser.Expression)
	for i, param := range definition.Params {
		if param.Arity == 0 {
			continue
		}
		arg := renamer.expression(call.Args[i], nil)
		if err := validActionOperator(arg, param.Arity); err != nil {
			return definition, err
		}
		operators[param.Name] = arg
	}
	if err := validateFunctionOperatorCalls(definition, operators); err != nil {
		return definition, err
	}
	definition.Body = substitute(definition.Body, operators)
	definition.Bounds = subBounds(definition.Bounds, operators)
	return definition, nil
}

func validateFunctionOperatorCalls(definition parser.Definition, operators map[string]*parser.Expression) error {
	if err := validateOperatorCalls(definition.Body, operators, operators); err != nil {
		return err
	}
	for _, bound := range definition.Bounds {
		if bound.Set != nil {
			if err := validateOperatorCalls(bound.Set, operators, operators); err != nil {
				return err
			}
		}
	}
	return nil
}
