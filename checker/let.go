package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

func (g *generator) letAction(item work, rest *workList, partial evaluator.State, action string) (bool, error) {
	tail := g.scopedTail(rest)
	scope := g.model.program.ExtendScope(g.lexical, item.expr.Definitions, item.locals)
	body := work{expr: item.expr.Args[0], path: item.path.withScope(scope), label: item.label, locals: item.locals}
	return g.solve(prepend(body, tail), partial, action)
}

func (m *model) letBody(expr *parser.Expression) (*parser.Expression, error) {
	if body, ok := m.letCache[expr]; ok {
		return body, nil
	}
	replacements, err := letReplacements(expr.Definitions)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for name, body := range replacements {
		names[name] = true
		collectNames(body, names)
	}
	if captures(expr.Args[0], names) {
		return nil, fmt.Errorf("LET action requires capture-avoiding substitution")
	}
	body := substitute(expr.Args[0], replacements)
	m.letCache[expr] = body
	return body, nil
}

func letReplacements(definitions []parser.Definition) (map[string]*parser.Expression, error) {
	defs := map[string]parser.Definition{}
	for _, d := range definitions {
		if d.Kind != "Operator" || len(d.Params) > 0 || d.Recursive {
			return nil, fmt.Errorf("parameterized or recursive LET actions are not yet supported")
		}
		defs[d.Name] = d
	}
	result := map[string]*parser.Expression{}
	for name := range defs {
		if err := resolveLet(name, defs, result, map[string]bool{}); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func resolveLet(name string, defs map[string]parser.Definition, resolved map[string]*parser.Expression, visiting map[string]bool) error {
	if _, ok := resolved[name]; ok {
		return nil
	}
	if visiting[name] {
		return fmt.Errorf("recursive LET definition %s", name)
	}
	visiting[name] = true
	defer delete(visiting, name)
	d := defs[name]
	names := map[string]bool{}
	collectNames(d.Body, names)
	for dependency := range names {
		if _, ok := defs[dependency]; ok {
			if err := resolveLet(dependency, defs, resolved, visiting); err != nil {
				return err
			}
		}
	}
	resolved[name] = substitute(d.Body, resolved)
	return nil
}
