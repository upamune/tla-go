package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (m *model) enabled(action *parser.Expression, current evaluator.State, lexical *evaluator.LexicalScope) (bool, error) {
	return m.enabledWithLocals(action, current, lexical, nil)
}

func (m *model) enabledWithLocals(action *parser.Expression, current evaluator.State, lexical *evaluator.LexicalScope, locals map[string]value.Value) (bool, error) {
	if current == nil {
		return false, fmt.Errorf("ENABLED needs a complete state")
	}
	for _, name := range m.names {
		if _, ok := current[name]; !ok {
			return false, fmt.Errorf("ENABLED needs a complete state")
		}
	}
	found := false
	g := generator{model: m, current: current, enabled: true, lexical: lexical, emit: func(evaluator.State, string, *parser.Expression) (bool, error) {
		found = true
		return false, nil
	}}
	_, err := g.solve([]work{{expr: action, locals: locals}}, evaluator.State{}, "")
	return found, err
}

func (g *generator) definition(name string) (parser.Definition, bool) {
	if g.lexical != nil {
		return g.lexical.Definition(name)
	}
	d, ok := g.model.defs[name]
	return d, ok
}

func (g *generator) hasLocalValue(name string, locals map[string]value.Value) bool {
	if _, ok := locals[name]; ok {
		return true
	}
	return g.lexical != nil && g.lexical.HasValue(name)
}

func (g *generator) evalWithLocals(expr *parser.Expression, current, next evaluator.State, locals map[string]value.Value) (value.Value, error) {
	return g.model.program.Evaluate(expr, evaluator.Context{Current: current, Next: next, Constants: g.model.constants, Locals: locals, Lexical: g.lexical, Enabled: g.model.enabledCallback, UncachedArguments: g.enabled})
}

func (g *generator) holdsWithLocals(expr *parser.Expression, current, next evaluator.State, locals map[string]value.Value) (bool, error) {
	v, err := g.evalWithLocals(expr, current, next, locals)
	if err != nil {
		return false, err
	}
	b, ok := v.Bool()
	if !ok {
		if g.enabled {
			return false, fmt.Errorf("predicate must be boolean, got %s", v)
		}
		return false, g.nonBooleanGenerationError(expr, v)
	}
	return b, nil
}

func (g *generator) nonBooleanGenerationError(expr *parser.Expression, v value.Value) error {
	phase := "next"
	if g.initial {
		phase = "initial"
	}
	detail := v.String()
	r := expr.Location.Range
	if expr.Kind != "Number" && r.Start.Line > 0 {
		detail = fmt.Sprintf("line %d, col %d to line %d, col %d of module %s", r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
	}
	message := fmt.Sprintf("In computing %s states, TLC expected a boolean expression,\nbut instead found %s.\n%s", phase, v, detail)
	return &evaluator.RuntimeError{Message: message, ToolCode: 2247, TrackSource: true, Sources: []*parser.Expression{expr}}
}
