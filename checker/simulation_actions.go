package checker

import (
	"maps"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// simulationAction retains the bindings with which TLC split the next-state
// relation. Different actions are sampled before their successors are sampled.
type simulationAction struct {
	name   string
	expr   *parser.Expression
	locals map[string]value.Value
	scope  *evaluator.LexicalScope
}

type simulationActionCollector struct {
	model   *model
	actions []simulationAction
	seen    map[string]bool
}

func (m *model) simulationActions() ([]simulationAction, error) {
	next, err := m.definition(m.cfg.Next)
	if err != nil {
		return nil, err
	}
	collector := simulationActionCollector{model: m, seen: map[string]bool{}}
	err = collector.collect(next, m.cfg.Next, nil, nil)
	return collector.actions, err
}

func (c *simulationActionCollector) collect(expr *parser.Expression, name string, locals map[string]value.Value, scope *evaluator.LexicalScope) error {
	if err := c.model.ctx.Err(); err != nil {
		return err
	}
	switch expr.Kind {
	case "Or":
		for _, branch := range expr.Args {
			if err := c.collect(branch, name, locals, scope); err != nil {
				return err
			}
		}
		return nil
	case "Label":
		return c.collect(expr.Args[0], name, locals, scope)
	case "Let":
		nested := c.model.program.ExtendScope(scope, expr.Definitions, locals)
		return c.collect(expr.Args[0], name, locals, nested)
	case "Exists":
		return c.collectExists(expr, name, locals, scope)
	default:
		return c.collectCall(expr, name, locals, scope)
	}
}

func (c *simulationActionCollector) add(expr *parser.Expression, name string, locals map[string]value.Value, scope *evaluator.LexicalScope) {
	c.actions = append(c.actions, simulationAction{name: name, expr: expr, locals: maps.Clone(locals), scope: scope})
}

func (c *simulationActionCollector) collectExists(expr *parser.Expression, name string, locals map[string]value.Value, scope *evaluator.LexicalScope) error {
	before := len(c.actions)
	err := c.expandBounds(expr.Bounds, locals, scope, func(bound map[string]value.Value) error {
		return c.collect(expr.Args[0], name, bound, scope)
	})
	if err != nil {
		if contextErr := c.model.ctx.Err(); contextErr != nil {
			return contextErr
		}
		c.actions = c.actions[:before]
		c.add(expr, name, locals, scope)
	}
	return nil
}

func (c *simulationActionCollector) expandBounds(bounds []parser.Bound, locals map[string]value.Value, scope *evaluator.LexicalScope, visit func(map[string]value.Value) error) error {
	if len(bounds) == 0 {
		return visit(locals)
	}
	domain, err := c.model.program.Evaluate(bounds[0].Set, evaluator.Context{Constants: c.model.constants, Locals: locals, Lexical: scope})
	if err != nil {
		return err
	}
	return domain.Each(func(item value.Value) (bool, error) {
		bound, err := bindBound(bounds[0], item, locals, "simulation tuple binding has wrong arity")
		if err != nil {
			return false, err
		}
		return true, c.expandBounds(bounds[1:], bound, scope, visit)
	})
}

func (c *simulationActionCollector) collectCall(expr *parser.Expression, name string, locals map[string]value.Value, scope *evaluator.LexicalScope) error {
	if expr.Kind != "Apply" && expr.Kind != "Identifier" {
		c.add(expr, name, locals, scope)
		return nil
	}
	if c.callIsBound(expr.Text, locals, scope) {
		c.add(expr, name, locals, scope)
		return nil
	}
	definition, ok := c.definition(expr.Text, scope)
	if !ok || definition.Kind != "Operator" {
		c.add(expr, name, locals, scope)
		return nil
	}
	bound, ok := c.bindArguments(definition, expr.Args, locals, scope)
	if !ok {
		c.add(expr, definition.Name, locals, scope)
		return nil
	}
	c.seen[expr.Text] = true
	defer delete(c.seen, expr.Text)
	return c.collect(definition.Body, definition.Name, bound, scope)
}

func (c *simulationActionCollector) callIsBound(name string, locals map[string]value.Value, scope *evaluator.LexicalScope) bool {
	_, ok := locals[name]
	return ok || scope.HasValue(name) || c.seen[name]
}

func (c *simulationActionCollector) definition(name string, scope *evaluator.LexicalScope) (parser.Definition, bool) {
	if scope != nil {
		return scope.Definition(name)
	}
	definition, ok := c.model.defs[name]
	return definition, ok
}

func (c *simulationActionCollector) bindArguments(definition parser.Definition, args []*parser.Expression, locals map[string]value.Value, scope *evaluator.LexicalScope) (map[string]value.Value, bool) {
	if len(args) != len(definition.Params) {
		return nil, false
	}
	bound := maps.Clone(locals)
	if bound == nil {
		bound = map[string]value.Value{}
	}
	for i, arg := range args {
		if definition.Params[i].Arity != 0 || c.model.readsState(arg, map[string]bool{}) || c.model.hasAction(arg, map[string]bool{}) {
			return nil, false
		}
		v, err := c.model.program.Evaluate(arg, evaluator.Context{Constants: c.model.constants, Locals: locals, Lexical: scope})
		if err != nil {
			return nil, false
		}
		bound[definition.Params[i].Name] = v
	}
	return bound, true
}
