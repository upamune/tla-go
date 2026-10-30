package checker

import (
	"fmt"
	"slices"

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
	d, ok := g.definition(call.Text)
	if !ok {
		return nil, fmt.Errorf("undefined action %s", call.Text)
	}
	if d.Kind != "Operator" || len(d.Params) != len(call.Args) {
		return nil, fmt.Errorf("invalid action call %s", call.Text)
	}
	cacheable := g.cacheableActionBody(call, d)
	if body, ok := g.cachedActionBody(call, cacheable); ok {
		return body, nil
	}
	body, err := substituteActionCall(d, call)
	if err != nil {
		return nil, err
	}
	g.rememberActionBody(call, body, cacheable)
	return body, nil
}

func (g *generator) cachedActionBody(call *parser.Expression, cacheable bool) (*parser.Expression, bool) {
	if !cacheable {
		return nil, false
	}
	body, ok := g.model.actionCache[call]
	return body, ok
}

func (g *generator) rememberActionBody(call, body *parser.Expression, cacheable bool) {
	if cacheable && (g.lexical == nil || len(g.model.actionCache) < 4096) {
		g.model.actionCache[call] = body
	}
}

// ENABLED can retain a lexical scope even when an action resolves to the
// unchanged module definition. Substitution depends on the definition and
// call syntax, not on the values captured in that scope.
func (g *generator) cacheableActionBody(call *parser.Expression, d parser.Definition) bool {
	if g.lexical == nil {
		return true
	}
	root, ok := g.model.defs[call.Text]
	return ok && root.Body == d.Body && slices.Equal(root.Params, d.Params)
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
