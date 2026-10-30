package evaluator

import (
	"errors"
	"fmt"

	"github.com/upamune/tla-go/value"
)

func functionLookup(fn, key value.Value) (value.Value, error) {
	result, err := fn.Lookup(key)
	if err == nil {
		return result, nil
	}
	if fn.Kind() != value.FunctionKind && fn.Kind() != value.LazyFunctionKind {
		message := fmt.Sprintf("Attempted to apply %s, which is not a function", fn)
		return value.Value{}, &RuntimeError{Message: message, Legacy: err.Error()}
	}
	if errors.Is(err, value.ErrOutsideFunctionDomain) {
		return value.Value{}, &RuntimeError{Message: domainFailureMessage(fn, key), Legacy: err.Error()}
	}
	return value.Value{}, err
}

func domainFailureMessage(fn, key value.Value) string {
	if fn.Kind() == value.LazyFunctionKind {
		return fmt.Sprintf("In applying the function\n%s,\nthe first argument is:\n%s\nwhich is not in its domain.\n", fn, key)
	}
	return fmt.Sprintf("Attempted to apply function %s to %s, which is not in its domain", fn, key)
}
