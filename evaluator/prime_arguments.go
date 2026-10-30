package evaluator

import (
	"fmt"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func primeArgumentBindings(params []parser.Declaration, expressions []*parser.Expression, caller *scope, enabledScopes []string) map[string]*primeArgument {
	if len(expressions) == 0 {
		return nil
	}
	bindings := make(map[string]*primeArgument, len(expressions))
	for i, param := range params {
		if param.Arity == 0 {
			bindings[param.Name] = &primeArgument{expression: expressions[i], scope: caller, enabledScopes: enabledScopes}
		}
	}
	return bindings
}

func lookupPrimeArgument(s *scope, name string) (*primeArgument, bool) {
	for ; s != nil; s = s.parent {
		if arg, ok := s.primeArgs[name]; ok {
			return arg, true
		}
		if _, ok := s.values[name]; ok {
			return nil, false
		}
		if _, ok := s.definitions[name]; ok {
			return nil, false
		}
		if _, ok := s.operators[name]; ok {
			return nil, false
		}
	}
	return nil, false
}

func placeholderArguments(count int) []value.Value {
	values := make([]value.Value, count)
	for i := range values {
		values[i] = value.Bool(false)
	}
	return values
}

func lazyRecursiveCallKey(key string, expressions []*parser.Expression, caller *scope) string {
	var result strings.Builder
	result.WriteString(key)
	for _, expression := range expressions {
		resolved, scope := resolveLazyAlias(expression, caller)
		fmt.Fprintf(&result, ":%p:%p", resolved, scope)
	}
	return result.String()
}

func resolveLazyAlias(expression *parser.Expression, current *scope) (*parser.Expression, *scope) {
	for range 256 {
		if expression.Kind != "Identifier" {
			break
		}
		arg, ok := lookupPrimeArgument(current, expression.Text)
		if !ok {
			break
		}
		expression, current = arg.expression, arg.scope
	}
	return expression, current
}

func (e *evaluator) evaluateArgument(arg *primeArgument) (value.Value, error) {
	cacheable := !e.context.UncachedArguments
	index := 0
	if e.primeArguments {
		index = 1
	}
	if cacheable {
		if cached, ok := arg.cachedValue(index); ok {
			return cached, nil
		}
	}
	child := *e
	child.scope = arg.scope
	result, err := child.eval(arg.expression)
	if err == nil && cacheable {
		arg.cacheValue(index, result)
	}
	return result, err
}

func (arg *primeArgument) cachedValue(index int) (value.Value, bool) {
	arg.cacheMu.Lock()
	defer arg.cacheMu.Unlock()
	return arg.values[index], arg.cached[index]
}

func (arg *primeArgument) cacheValue(index int, result value.Value) {
	arg.cacheMu.Lock()
	arg.values[index] = result
	arg.cached[index] = true
	arg.cacheMu.Unlock()
}

// PrimedArgumentVariable resolves a parameter back to its caller's variable.
// ENABLED needs this identity when it constructs a partial successor state.
func (s *LexicalScope) PrimedArgumentVariable(name string) (string, bool) {
	if s == nil {
		return "", false
	}
	arg, ok := lookupPrimeArgument(s.scope, name)
	if !ok || arg.expression.Kind != "Identifier" {
		return "", false
	}
	return resolvePrimeIdentifier(arg.scope, arg.expression.Text, 0)
}

func resolvePrimeIdentifier(s *scope, name string, depth int) (string, bool) {
	if depth >= 128 {
		return "", false
	}
	if arg, ok := lookupPrimeArgument(s, name); ok {
		if arg.expression.Kind != "Identifier" {
			return "", false
		}
		return resolvePrimeIdentifier(arg.scope, arg.expression.Text, depth+1)
	}
	return resolvePrimeDefinition(s, name, depth)
}

func resolvePrimeDefinition(s *scope, name string, depth int) (string, bool) {
	for current := s; current != nil; current = current.parent {
		if shadowsPrimeIdentifier(current, name) {
			return "", false
		}
		if definition, ok := current.definitions[name]; ok {
			return resolvePrimeDefinitionBody(current, definition, depth)
		}
	}
	return name, true
}

func shadowsPrimeIdentifier(s *scope, name string) bool {
	_, value := s.values[name]
	_, operator := s.operators[name]
	return value || operator
}

func resolvePrimeDefinitionBody(s *scope, definition parser.Definition, depth int) (string, bool) {
	if len(definition.Params) != 0 || definition.Body.Kind != "Identifier" {
		return "", false
	}
	return resolvePrimeIdentifier(s, definition.Body.Text, depth+1)
}
