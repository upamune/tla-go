package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

func (g *generator) conditional(item work, rest *workList, partial evaluator.State, action string) (bool, error) {
	current, next := g.context(partial)
	condition, err := g.holdsWithLocals(item.expr.Args[0], current, next, item.locals)
	if err != nil {
		return false, err
	}
	branch := 2
	if condition {
		branch = 1
	}
	return g.solve(prepend(work{expr: item.expr.Args[branch], path: item.path, locals: item.locals}, rest), partial, action)
}

func (g *generator) implication(item work, rest *workList, partial evaluator.State, action string) (bool, error) {
	current, next := g.context(partial)
	condition, err := g.holdsWithLocals(item.expr.Args[0], current, next, item.locals)
	if err != nil {
		return false, err
	}
	if !condition {
		return g.solve(rest, partial, action)
	}
	return g.solve(prepend(work{expr: item.expr.Args[1], path: item.path, locals: item.locals}, rest), partial, action)
}

func (g *generator) stutteringAction(item work, rest *workList, partial evaluator.State, action string) (bool, error) {
	subscript := item.expr.Args[1]
	unchanged := &parser.Expression{Kind: "Unchanged", Args: []*parser.Expression{subscript}}
	if item.expr.Kind == "BoxAction" {
		branch := &parser.Expression{Kind: "Or", Args: []*parser.Expression{item.expr.Args[0], unchanged}}
		item.expr = branch
		return g.disjunction(item, rest, partial, action)
	}
	negated := &parser.Expression{Kind: "Apply", Text: "~", Args: []*parser.Expression{unchanged}}
	rest = prepend(work{expr: negated, path: item.path, locals: item.locals}, rest)
	return g.solve(prepend(work{expr: item.expr.Args[0], path: item.path, label: item.label, locals: item.locals}, rest), partial, action)
}

func (g *generator) caseAction(item work, rest *workList, partial evaluator.State, action string) (bool, error) {
	current, next := g.context(partial)
	for _, arm := range item.expr.Arms {
		condition, err := g.holdsWithLocals(arm.Condition, current, next, item.locals)
		if err != nil {
			return false, err
		}
		if condition {
			item.expr = arm.Body
			item.label = false
			return g.solve(prepend(item, rest), partial, action)
		}
	}
	if len(item.expr.Args) == 0 {
		return false, fmt.Errorf("no CASE guard is true")
	}
	item.expr = item.expr.Args[0]
	item.label = false
	return g.solve(prepend(item, rest), partial, action)
}
