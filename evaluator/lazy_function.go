package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

type functionCacheKey struct {
	scope *scope
	state string
}

// functionCacheEntry keeps the operator arguments its key names by address
// reachable, so that no later scope can reuse one of those addresses.
type functionCacheEntry struct {
	value     value.Value
	operators map[string]operatorRef
}

func (e *evaluator) functionDefinition(d parser.Definition, args []value.Value, operators map[string]operatorRef, defined *scope) (value.Value, error) {
	if len(args) != len(d.Params) {
		return value.Value{}, fmt.Errorf("function definition %s expects %d operator arguments", d.Name, len(d.Params))
	}
	if e.depth >= 4096 {
		return value.Value{}, fmt.Errorf("evaluation recursion limit exceeded")
	}
	stateKey, keyErr := e.functionCallKey(d, args, operators, defined)
	cacheKey := functionCacheKey{scope: defined, state: stateKey}
	if cached, ok := e.cachedFunction(cacheKey, keyErr); ok {
		return cached, nil
	}
	child, err := e.functionScope(d, args, operators, defined)
	if err != nil {
		return value.Value{}, err
	}
	result, err := child.makeLazyFunction(d.Bounds, d.Body, d.Name)
	if err == nil && keyErr == nil {
		if e.functionCache == nil {
			e.functionCache = make(map[functionCacheKey]functionCacheEntry)
		}
		e.functionCache[cacheKey] = functionCacheEntry{value: result, operators: operators}
	}
	return result, err
}

func (e *evaluator) cachedFunction(key functionCacheKey, keyErr error) (value.Value, bool) {
	if keyErr != nil {
		return value.Value{}, false
	}
	cached, ok := e.functionCache[key]
	return cached.value, ok
}

func (e *evaluator) functionScope(d parser.Definition, args []value.Value, operators map[string]operatorRef, defined *scope) (*evaluator, error) {
	child := *e
	child.depth++
	child.scope = defined
	if len(d.Params) == 0 {
		return &child, nil
	}
	bindings, err := valueBindings(d.Params, args, operators)
	if err != nil {
		return nil, err
	}
	child.scope = &scope{parent: defined, bound: bindings, operators: operators}
	child.hasOperators = child.hasOperators || len(operators) > 0
	return &child, nil
}

func (e *evaluator) functionCallKey(d parser.Definition, args []value.Value, operators map[string]operatorRef, defined *scope) (string, error) {
	if len(operators) == 0 {
		return e.callKey(d, args, defined)
	}
	return e.operatorCallKey(d, args, operators, defined)
}

func (e *evaluator) boundsDomain(bounds []parser.Bound) (value.Value, error) {
	if len(bounds) == 0 {
		return value.Value{}, fmt.Errorf("function has no bounds")
	}
	sets := make([]value.Value, 0, len(bounds))
	for _, bound := range bounds {
		if bound.Set == nil {
			return value.Value{}, fmt.Errorf("function has an unbounded domain")
		}
		set, err := e.eval(bound.Set)
		if err != nil {
			return value.Value{}, err
		}
		sets = append(sets, set)
	}
	if len(sets) == 1 {
		return sets[0], nil
	}
	return value.ProductSet(sets)
}

func (e *evaluator) makeLazyFunction(bounds []parser.Bound, body *parser.Expression, name string) (value.Value, error) {
	domain, err := e.boundsDomain(bounds)
	if err != nil {
		return value.Value{}, err
	}
	return e.lazyFunctionOnDomain(domain, bounds, body, name)
}

func (e *evaluator) lazyFunctionOnDomain(domain value.Value, bounds []parser.Bound, body *parser.Expression, name string) (value.Value, error) {
	captured := e.snapshot()
	description := lazyDescription(domain, bounds, body)
	return value.LazyFunctionWithControl(domain, description, func(argument, self value.Value, keepLazy bool) (value.Value, error) {
		instance := captured
		instance.keepLazy = keepLazy
		if name != "" {
			instance.scope = singleValueScope(instance.scope, name, self)
		}
		child, err := instance.bindFunctionArgument(bounds, argument)
		if err != nil {
			return value.Value{}, err
		}
		return child.eval(body)
	})
}

func (e *evaluator) bindFunctionArgument(bounds []parser.Bound, argument value.Value) (*evaluator, error) {
	if len(bounds) == 1 {
		return e.bind(bounds[0], argument)
	}
	items, err := argument.TupleElements()
	if err != nil {
		return nil, err
	}
	if len(items) != len(bounds) {
		return nil, fmt.Errorf("function argument has wrong arity")
	}
	child := e
	for i, bound := range bounds {
		child, err = child.bind(bound, items[i])
		if err != nil {
			return nil, err
		}
	}
	return child, nil
}

func (e *evaluator) functionOperand(expr *parser.Expression) (value.Value, error) {
	saved := *e
	e.keepLazy = true
	result, err := e.eval(expr)
	*e = saved
	return result, err
}
