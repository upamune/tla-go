package evaluator

import (
	"errors"
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
		var missing bool
		current, missing, err = e.update(current, update.Path, update.Value)
		if err != nil {
			return value.Value{}, err
		}
		if missing {
			e.warnUnknownExceptField(expr.Args[0])
		}
	}
	return current, nil
}

// update applies one EXCEPT path. A path leaving a function's domain leaves
// the value unchanged and is reported as missing.
func (e *evaluator) update(current value.Value, path []parser.PathStep, expr *parser.Expression) (value.Value, bool, error) {
	if len(path) == 0 {
		result, err := e.evalInScope(singleValueScope(e.scope, "@", current), expr)
		return result, false, err
	}
	key, err := e.pathKey(path[0])
	if err != nil {
		return value.Value{}, false, err
	}
	if current.Kind() == value.LazyFunctionKind {
		return e.lazyUpdate(current, key, path, expr)
	}
	nestedMissing := false
	updated, found, err := current.UpdateExisting(key, func(previous value.Value) (value.Value, error) {
		result, missing, err := e.update(previous, path[1:], expr)
		nestedMissing = missing
		return result, err
	})
	return updated, !found || nestedMissing, err
}

// lazyUpdate defers the update of a lazily evaluated function. As Java
// TLC's Value.select does, it first checks that the whole path selects a
// value; otherwise the update is skipped and reported as missing.
func (e *evaluator) lazyUpdate(current, key value.Value, path []parser.PathStep, expr *parser.Expression) (value.Value, bool, error) {
	domain, err := current.FunctionDomain()
	if err != nil {
		return value.Value{}, false, err
	}
	found, err := domain.Contains(key)
	if err == nil && found && len(path) > 1 {
		found, err = e.selectsPath(current, key, path[1:])
	}
	if err != nil || !found {
		return current, err == nil, err
	}
	captured := e.snapshot()
	updated, err := current.Update(key, func(previous value.Value) (value.Value, error) {
		instance := captured
		result, _, err := instance.update(previous, path[1:], expr)
		return result, err
	})
	return updated, false, err
}

// selectsPath reports whether the steps below key select a value. A value
// that is not a function ends the check; applying the update reports it.
func (e *evaluator) selectsPath(current, key value.Value, steps []parser.PathStep) (bool, error) {
	selected, err := current.Lookup(key)
	for _, step := range steps {
		if err != nil || !isFunctionValue(selected) {
			break
		}
		if key, err = e.pathKey(step); err == nil {
			selected, err = selected.LookupWithLazyResult(key, true)
		}
	}
	if errors.Is(err, value.ErrOutsideFunctionDomain) {
		return false, nil
	}
	return err == nil, err
}

func isFunctionValue(v value.Value) bool {
	return v.Kind() == value.FunctionKind || v.Kind() == value.LazyFunctionKind
}

// warnUnknownExceptField reports Java TLC's warning 2144 at the location of
// the function whose EXCEPT path does not exist.
func (e *evaluator) warnUnknownExceptField(function *parser.Expression) {
	if e.program.onWarning == nil {
		return
	}
	r := function.Location.Range
	e.program.onWarning(2144, fmt.Sprintf("The EXCEPT was applied to non-existing fields of the value at\nline %d, col %d to line %d, col %d of module %s", r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module))
}

func (e *evaluator) pathKey(step parser.PathStep) (value.Value, error) {
	if step.Kind == "Field" {
		return e.program.literal(step.Name), nil
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
