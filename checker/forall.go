package checker

import (
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (g *generator) universal(item work, rest *workList, partial evaluator.State, action string) (bool, error) {
	var bindings []map[string]value.Value
	err := g.collectBindings(item.expr.Bounds, partial, item.locals, &bindings)
	if err != nil {
		return false, err
	}
	reverseTail(bindings)
	items, nodes, mark := g.model.work.prepend(len(bindings), rest)
	for i, locals := range bindings {
		nodes[i].item = work{expr: item.expr.Args[0], path: item.path, locals: locals}
	}
	more, err := g.solve(items, partial, action)
	g.model.work.release(mark)
	return more, err
}

// tla-ts uses the first binding followed by the remaining bindings in reverse.
func reverseTail(bindings []map[string]value.Value) {
	for left, right := 1, len(bindings)-1; left < right; left, right = left+1, right-1 {
		bindings[left], bindings[right] = bindings[right], bindings[left]
	}
}

func (g *generator) collectBindings(bounds []parser.Bound, partial evaluator.State, locals map[string]value.Value, out *[]map[string]value.Value) error {
	if len(bounds) == 0 {
		*out = append(*out, locals)
		return nil
	}
	current, next := g.context(partial)
	domain, err := g.evalWithLocals(bounds[0].Set, current, next, locals)
	if err != nil {
		return err
	}
	return domain.Each(func(item value.Value) (bool, error) {
		bindings, err := bindBound(bounds[0], item, locals, "tuple binding has wrong arity")
		if err != nil {
			return false, err
		}
		err = g.collectBindings(bounds[1:], partial, bindings, out)
		return err == nil, err
	})
}
