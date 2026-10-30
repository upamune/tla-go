package checker

import (
	"fmt"
	"maps"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (g *generator) unchanged(expr *parser.Expression, rest []work, partial evaluator.State, action string) (bool, error) {
	names, err := g.unchangedNames(expr.Args[0])
	if err != nil {
		return false, err
	}
	next := maps.Clone(partial)
	for _, name := range names {
		ok, err := g.copyUnchanged(name, next)
		if err != nil || !ok {
			return ok, err
		}
	}
	return g.solve(rest, next, action)
}

func (g *generator) copyUnchanged(name string, next evaluator.State) (bool, error) {
	v, ok := g.current[name]
	if !ok {
		return false, fmt.Errorf("variable %s is unassigned", name)
	}
	if existing, ok := next[name]; ok {
		same, err := value.Equal(existing, v)
		if err != nil || !same {
			return same, err
		}
	}
	next[name] = v
	return true, nil
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
