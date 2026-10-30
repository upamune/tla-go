package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (e *evaluator) action(expr *parser.Expression) (value.Value, error) {
	action, err := e.condition(expr.Args[0])
	if err != nil {
		return value.Value{}, err
	}
	box := expr.Kind == "BoxAction"
	if action == box {
		return value.Bool(box), nil
	}
	current, err := e.eval(expr.Args[1])
	if err != nil {
		return value.Value{}, err
	}
	primed := &parser.Expression{Args: []*parser.Expression{expr.Args[1]}}
	next, err := e.prime(primed)
	if err != nil {
		return value.Value{}, err
	}
	same, err := value.Equal(current, next)
	if !box {
		same = !same
	}
	return value.Bool(same), err
}

func (e *evaluator) except(expr *parser.Expression) (value.Value, error) {
	current, err := e.functionOperand(expr.Args[0])
	if err != nil {
		return value.Value{}, err
	}
	for _, update := range expr.Updates {
		current, err = e.update(current, update.Path, update.Value)
		if err != nil {
			return value.Value{}, err
		}
	}
	return current, nil
}

func (e *evaluator) update(current value.Value, path []parser.PathStep, expr *parser.Expression) (value.Value, error) {
	if len(path) == 0 {
		child := *e
		child.scope = &scope{parent: e.scope, values: map[string]value.Value{"@": current}}
		return child.eval(expr)
	}
	key, err := e.pathKey(path[0])
	if err != nil {
		return value.Value{}, err
	}
	if current.Kind() == value.LazyFunctionKind {
		captured := e.snapshot()
		return current.Update(key, func(previous value.Value) (value.Value, error) {
			instance := captured
			return instance.update(previous, path[1:], expr)
		})
	}
	return current.Update(key, func(previous value.Value) (value.Value, error) { return e.update(previous, path[1:], expr) })
}

func (e *evaluator) pathKey(step parser.PathStep) (value.Value, error) {
	if step.Kind == "Field" {
		return value.String(step.Name), nil
	}
	if step.Kind != "Index" {
		return value.Value{}, fmt.Errorf("unsupported EXCEPT path %s", step.Kind)
	}
	args, err := e.evalArgs(step.Args)
	if err != nil {
		return value.Value{}, err
	}
	return functionKey(args)
}

func (e *evaluator) exceptAt() (value.Value, error) {
	v, ok, err := e.scoped("@")
	if !ok {
		return value.Value{}, fmt.Errorf("@ outside EXCEPT")
	}
	return v, err
}
