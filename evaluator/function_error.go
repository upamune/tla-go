package evaluator

import (
	"errors"
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func functionLookup(expr *parser.Expression, fn, key value.Value, keepLazy bool) (value.Value, error) {
	result, err := fn.LookupWithLazyResult(key, keepLazy)
	if err == nil {
		return result, nil
	}
	if fn.Kind() != value.FunctionKind && fn.Kind() != value.LazyFunctionKind {
		// Java TLC's Tool.evalAppl reports this through Assert.fail.
		r := expr.Location.Range
		message := fmt.Sprintf("A non-function (%s) was applied as a function.\nline %d, col %d to line %d, col %d of module %s", fn.TLCKindDescription(), r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
		return value.Value{}, &value.UnexpectedError{Message: message}
	}
	if errors.Is(err, value.ErrOutsideFunctionDomain) {
		return value.Value{}, &RuntimeError{Message: domainFailureMessage(fn, key), Legacy: err.Error(), TrackSource: fn.IsTuple()}
	}
	return value.Value{}, err
}

func domainFailureMessage(fn, key value.Value) string {
	if fn.IsTuple() {
		if n, ok := key.Int(); ok {
			return fmt.Sprintf("Attempted to apply tuple\n%s\nto integer %d which is out of domain.", fn, n)
		}
		return "Attempted to apply tuple to a non-integer argument."
	}
	if fn.Kind() == value.LazyFunctionKind {
		return fmt.Sprintf("In applying the function\n%s,\nthe first argument is:\n%s\nwhich is not in its domain.\n", fn, key)
	}
	return fmt.Sprintf("Attempted to apply function %s to %s, which is not in its domain", fn, key)
}
