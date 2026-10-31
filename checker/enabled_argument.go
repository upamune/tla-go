package checker

import "github.com/upamune/tla-go/evaluator"

func (g *generator) lazyEnabledAction(item work, rest *workList, partial evaluator.State, action string) (bool, bool, error) {
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
	tail := g.scopedTail(rest)
	expanded := work{expr: expr, path: item.path.withScope(scope), label: item.label}
	more, err := g.solve(prepend(expanded, tail), partial, action)
	return true, more, err
}

func (g *generator) lazyEnabledOperatorAction(item work, rest *workList, partial evaluator.State, action string) (bool, bool, error) {
	if !g.enabled {
		return false, false, nil
	}
	body, scope, ok, err := g.lexical.BindActionOperator(item.expr.Text, item.expr.Args)
	if !ok || err != nil {
		return ok, false, err
	}
	tail := g.scopedTail(rest)
	expanded := work{expr: body, path: item.path.withScope(scope), label: item.label}
	more, err := g.solve(prepend(expanded, tail), partial, action)
	return true, more, err
}
