package evaluator

import "github.com/upamune/tla-go/syntax/parser"

func primeArgumentBindings(params []parser.Declaration, expressions []*parser.Expression, caller *scope) map[string]primeArgument {
	if len(expressions) == 0 {
		return nil
	}
	bindings := make(map[string]primeArgument, len(expressions))
	for i, param := range params {
		if param.Arity == 0 {
			bindings[param.Name] = primeArgument{expression: expressions[i], scope: caller}
		}
	}
	return bindings
}

func lookupPrimeArgument(s *scope, name string) (primeArgument, bool) {
	for ; s != nil; s = s.parent {
		if arg, ok := s.primeArgs[name]; ok {
			return arg, true
		}
		if _, ok := s.values[name]; ok {
			return primeArgument{}, false
		}
		if _, ok := s.definitions[name]; ok {
			return primeArgument{}, false
		}
		if _, ok := s.operators[name]; ok {
			return primeArgument{}, false
		}
	}
	return primeArgument{}, false
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
