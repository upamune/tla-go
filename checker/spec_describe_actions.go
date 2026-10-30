package checker

import (
	"maps"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

type actionDescriptionScope struct {
	name       string
	parameters []describedParameter
}

func (d *specDescription) collectActions(expr *parser.Expression, scope actionDescriptionScope, locals map[string]value.Value, seen map[string]bool) error {
	switch expr.Kind {
	case "Or":
		return d.collectActionAlternatives(expr.Args, scope, locals, seen)
	case "Label":
		return d.collectActions(expr.Args[0], scope, locals, seen)
	case "Let":
		body, err := d.model.letBody(expr)
		if err != nil {
			return d.addSplitAction(expr, scope)
		}
		return d.collectActions(body, scope, locals, seen)
	case "Exists":
		return d.collectExistentialActions(expr, scope, locals, seen)
	default:
		return d.collectActionCall(expr, scope, locals, seen)
	}
}

func (d *specDescription) collectActionAlternatives(args []*parser.Expression, scope actionDescriptionScope, locals map[string]value.Value, seen map[string]bool) error {
	for _, arg := range args {
		if err := d.collectActions(arg, scope, locals, seen); err != nil {
			return err
		}
	}
	return nil
}

func (d *specDescription) collectExistentialActions(expr *parser.Expression, scope actionDescriptionScope, locals map[string]value.Value, seen map[string]bool) error {
	before := len(d.sets["actions"])
	err := d.model.expandBindings(expr.Bounds, locals, func(bound map[string]value.Value) error {
		return d.collectActions(expr.Args[0], scope, bound, seen)
	})
	if err != nil || len(d.sets["actions"]) == before {
		d.sets["actions"] = d.sets["actions"][:before]
		return d.addSplitAction(expr, scope)
	}
	return nil
}

func (d *specDescription) collectActionCall(expr *parser.Expression, scope actionDescriptionScope, locals map[string]value.Value, seen map[string]bool) error {
	if expr.Kind != "Apply" && expr.Kind != "Identifier" {
		return d.addSplitAction(expr, scope)
	}
	definition, ok := d.model.defs[expr.Text]
	if !ok || definition.Kind != "Operator" || seen[expr.Text] {
		return d.addSplitAction(expr, scope)
	}
	return d.collectDefinedAction(expr, definition, locals, seen)
}

func (d *specDescription) collectDefinedAction(expr *parser.Expression, definition parser.Definition, locals map[string]value.Value, seen map[string]bool) error {
	if definition.InstanceRange.Module != "" {
		return d.addInstanceAction(expr, definition, locals)
	}
	if d.isConstantDefinition(definition) {
		return d.add("actions", describedAction{name: definition.Name, location: expr.Location.Range})
	}
	parameters, bound, ok := d.actionParameters(definition.Params, expr.Args, locals)
	if !ok {
		return d.add("actions", describedAction{name: definition.Name, location: expr.Location.Range})
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return d.collectActions(definition.Body, actionDescriptionScope{name: definition.Name, parameters: parameters}, bound, seen)
}

func (d *specDescription) addInstanceAction(expr *parser.Expression, definition parser.Definition, locals map[string]value.Value) error {
	n := definition.InstanceParamCount
	if n > len(definition.Params) || n > len(expr.Args) {
		return d.add("actions", describedAction{name: definition.Name, location: expr.Location.Range})
	}
	parameters, _, ok := d.actionParameters(definition.Params[:n], expr.Args[:n], locals)
	if !ok {
		return d.add("actions", describedAction{name: definition.Name, location: expr.Location.Range})
	}
	return d.add("actions", describedAction{name: definition.Name, location: definition.InstanceRange, parameters: parameters})
}

func (d *specDescription) actionParameters(params []parser.Declaration, args []*parser.Expression, locals map[string]value.Value) ([]describedParameter, map[string]value.Value, bool) {
	if len(params) != len(args) {
		return nil, nil, false
	}
	bound := make(map[string]value.Value, len(params))
	parameters := make([]describedParameter, 0, len(params))
	for i, param := range params {
		if param.Arity != 0 || d.model.readsState(args[i], map[string]bool{}) || d.model.hasAction(args[i], map[string]bool{}) {
			return nil, nil, false
		}
		v, err := d.model.program.Evaluate(args[i], evaluator.Context{Constants: d.model.constants, Locals: maps.Clone(locals)})
		if err != nil {
			return nil, nil, false
		}
		bound[param.Name] = v
		parameters = append(parameters, describedParameter{name: param.Name, value: v})
	}
	return parameters, bound, true
}

func (d *specDescription) addSplitAction(expr *parser.Expression, scope actionDescriptionScope) error {
	name := scope.name
	if name == "" {
		name = "UnnamedAction"
	}
	return d.add("actions", describedAction{name: name, location: expr.Location.Range, parameters: scope.parameters})
}
