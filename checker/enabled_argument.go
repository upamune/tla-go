package checker

import "github.com/upamune/tla-go/evaluator"

func (g *generator) lazyEnabledAction(item work, rest []work, partial evaluator.State, action string) (bool, bool, error) {
	if !g.enabled || item.expr.Kind != "Identifier" {
		return false, false, nil
	}
	if _, shadowed := item.locals[item.expr.Text]; shadowed {
		return false, false, nil
	}
	expr, scope, ok := g.lexical.Argument(item.expr.Text)
	if !ok {
		return false, false, nil
	}
	tail := append([]work(nil), rest...)
	for i := range tail {
		if !tail[i].path.hasLexicalScope() {
			tail[i].path = tail[i].path.withScope(g.lexical)
		}
	}
	expanded := work{expr: expr, path: item.path.withScope(scope), label: item.label}
	more, err := g.solve(prepend(expanded, tail), partial, action)
	return true, more, err
}

func (g *generator) lazyEnabledOperatorAction(item work, rest []work, partial evaluator.State, action string) (bool, bool, error) {
	if !g.enabled {
		return false, false, nil
	}
	body, scope, ok, err := g.lexical.BindActionOperator(item.expr.Text, item.expr.Args)
	if !ok || err != nil {
		return ok, false, err
	}
	tail := append([]work(nil), rest...)
	for i := range tail {
		if !tail[i].path.hasLexicalScope() {
			tail[i].path = tail[i].path.withScope(g.lexical)
		}
	}
	expanded := work{expr: body, path: item.path.withScope(scope), label: item.label}
	more, err := g.solve(prepend(expanded, tail), partial, action)
	return true, more, err
}
