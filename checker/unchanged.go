package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (g *generator) unchanged(expr *parser.Expression, rest *workList, partial evaluator.State, action string) (bool, error) {
	names, err := g.unchangedNames(expr.Args[0])
	if err != nil {
		return false, err
	}
	mark := len(g.undo)
	for _, name := range names {
		ok, err := g.copyUnchanged(name, partial)
		if err != nil || !ok {
			g.restore(partial, mark)
			return ok, err
		}
	}
	more, err := g.solve(rest, partial, action)
	g.restore(partial, mark)
	return more, err
}

// stateUndo restores a partial-state entry overwritten by UNCHANGED.
type stateUndo struct {
	name     string
	previous value.Value
	assigned bool
}

func (g *generator) copyUnchanged(name string, next evaluator.State) (bool, error) {
	v, ok := g.current[name]
	if !ok {
		return false, fmt.Errorf("variable %s is unassigned", name)
	}
	existing, assigned := next[name]
	if assigned {
		same, err := value.Equal(existing, v)
		if err != nil || !same {
			return same, err
		}
	}
	// An equal value can differ in presentation, so the current value replaces it.
	g.undo = append(g.undo, stateUndo{name: name, previous: existing, assigned: assigned})
	next[name] = v
	return true, nil
}

func (g *generator) restore(partial evaluator.State, mark int) {
	for i := len(g.undo) - 1; i >= mark; i-- {
		entry := g.undo[i]
		if entry.assigned {
			partial[entry.name] = entry.previous
		} else {
			delete(partial, entry.name)
		}
	}
	clear(g.undo[mark:])
	g.undo = g.undo[:mark]
}

func (g *generator) unchangedNames(expr *parser.Expression) ([]string, error) {
	return g.unchangedNamesSeen(expr, map[string]bool{})
}

func (g *generator) unchangedNamesSeen(expr *parser.Expression, seen map[string]bool) ([]string, error) {
	if name, ok := g.assignable(expr, map[string]bool{}); ok {
		return []string{name}, nil
	}
	if expr.Kind == "Identifier" && !seen[expr.Text] {
		definition, ok := g.definition(expr.Text)
		if ok && definition.Kind == "Operator" && len(definition.Params) == 0 {
			seen[expr.Text] = true
			defer delete(seen, expr.Text)
			return g.unchangedNamesSeen(definition.Body, seen)
		}
	}
	if expr.Kind == "Tuple" {
		return g.unchangedTuple(expr.Args, seen)
	}
	return nil, fmt.Errorf("unsupported UNCHANGED target")
}

func (g *generator) unchangedTuple(args []*parser.Expression, seen map[string]bool) ([]string, error) {
	var names []string
	for _, arg := range args {
		items, err := g.unchangedNamesSeen(arg, seen)
		if err != nil {
			return nil, err
		}
		names = append(names, items...)
	}
	return names, nil
}
