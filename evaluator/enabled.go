package evaluator

import (
	"fmt"
	"maps"
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// LexicalScope is an immutable snapshot used by an ENABLED callback.
type LexicalScope struct {
	scope         *scope
	hasOperators  bool
	enabledScopes []string
}

type EnabledCallback func(action *parser.Expression, current State, lexical *LexicalScope) (bool, error)

// ExtendScope creates a lexical scope containing LET definitions.
func (p *Program) ExtendScope(parent *LexicalScope, definitions []parser.Definition, values map[string]value.Value) *LexicalScope {
	base := &p.root
	hasOperators := false
	if parent != nil {
		base = parent.scope
		hasOperators = parent.hasOperators
	}
	if len(values) > 0 {
		base = &scope{parent: base, values: maps.Clone(values)}
	}
	defs := make(map[string]parser.Definition, len(definitions))
	for _, definition := range definitions {
		defs[definition.Name] = definition
	}
	var enabledScopes []string
	if parent != nil {
		enabledScopes = parent.enabledScopes
	}
	return &LexicalScope{scope: &scope{parent: base, definitions: defs}, hasOperators: hasOperators, enabledScopes: enabledScopes}
}

func (e *evaluator) enabled(expr *parser.Expression) (value.Value, error) {
	if e.context.Enabled == nil {
		return value.Value{}, fmt.Errorf("ENABLED requires a model checker")
	}
	if len(expr.Args) != 1 {
		return value.Value{}, fmt.Errorf("invalid ENABLED expression")
	}
	lexical := &LexicalScope{scope: snapshotScope(e.scope), hasOperators: e.hasOperators, enabledScopes: expr.EnabledScopes}
	result, err := e.context.Enabled(expr.Args[0], e.context.Current, lexical)
	return value.Bool(result), err
}

// EnabledVariable resolves a primed variable at the ENABLED scope boundary.
func (s *LexicalScope) EnabledVariable(expr *parser.Expression) (string, bool) {
	if s == nil {
		return "", false
	}
	if len(expr.EnabledBindings) > 0 {
		// If the innermost instance is entered after ENABLED, TLC follows its
		// substitution through outer instances instead of stopping at them.
		binding := expr.EnabledBindings[0]
		return binding.Name, slices.Contains(s.enabledScopes, binding.Scope)
	}
	return "", false
}

// Argument returns a lazy operator argument and its original lexical scope.
func (s *LexicalScope) Argument(name string) (*parser.Expression, *LexicalScope, bool) {
	if s == nil {
		return nil, nil, false
	}
	arg, ok := lookupPrimeArgument(s.scope, name)
	if !ok {
		return nil, nil, false
	}
	caller := &LexicalScope{scope: arg.scope, hasOperators: s.hasOperators, enabledScopes: arg.enabledScopes}
	return arg.expression, caller, true
}

// BindActionOperator expands an operator argument in its captured lexical scope.
func (s *LexicalScope) BindActionOperator(name string, args []*parser.Expression) (*parser.Expression, *LexicalScope, bool, error) {
	if s == nil {
		return nil, nil, false, nil
	}
	op, ok := findScopedOperator(s.scope, name)
	if !ok || op.definition == nil && op.lambda == nil {
		return nil, nil, false, nil
	}
	params, body := actionOperatorBody(op)
	if len(params) != len(args) {
		return nil, nil, true, fmt.Errorf("operator %s expects %d arguments", name, len(params))
	}
	values := make(map[string]value.Value, len(params))
	for _, param := range params {
		values[param.Name] = value.Bool(false)
	}
	bound := &scope{parent: op.scope, values: values, primeArgs: primeArgumentBindings(params, args, s.scope, s.enabledScopes)}
	lexical := &LexicalScope{scope: bound, hasOperators: s.hasOperators, enabledScopes: op.enabledScopes}
	return body, lexical, true, nil
}

func actionOperatorBody(op operatorRef) ([]parser.Declaration, *parser.Expression) {
	if op.definition != nil {
		return op.definition.Params, op.definition.Body
	}
	params := make([]parser.Declaration, len(op.lambda.Names))
	for i, name := range op.lambda.Names {
		params[i].Name = name
	}
	return params, op.lambda.Args[0]
}

func (e *evaluator) activeEnabledScopes() []string {
	if e.context.Lexical == nil {
		return nil
	}
	return e.context.Lexical.enabledScopes
}

// Definition looks up a visible definition while respecting shadowing.
func (s *LexicalScope) Definition(name string) (parser.Definition, bool) {
	if s == nil {
		return parser.Definition{}, false
	}
	for current := s.scope; current != nil; current = current.parent {
		if _, ok := current.values[name]; ok {
			return parser.Definition{}, false
		}
		if _, ok := current.operators[name]; ok {
			return parser.Definition{}, false
		}
		if d, ok := current.definitions[name]; ok {
			return d, true
		}
	}
	return parser.Definition{}, false
}

// HasValue reports whether a visible identifier denotes a value binding.
func (s *LexicalScope) HasValue(name string) bool {
	if s == nil {
		return false
	}
	for current := s.scope; current != nil; current = current.parent {
		if _, ok := current.values[name]; ok {
			return true
		}
		if _, ok := current.operators[name]; ok {
			return false
		}
		if _, ok := current.definitions[name]; ok {
			return false
		}
	}
	return false
}
