package evaluator

import (
	"fmt"
	"maps"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// LexicalScope is an immutable snapshot used by an ENABLED callback.
type LexicalScope struct {
	scope        *scope
	hasOperators bool
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
	return &LexicalScope{scope: &scope{parent: base, definitions: defs}, hasOperators: hasOperators}
}

func (e *evaluator) enabled(expr *parser.Expression) (value.Value, error) {
	if e.context.Enabled == nil {
		return value.Value{}, fmt.Errorf("ENABLED requires a model checker")
	}
	if len(expr.Args) != 1 {
		return value.Value{}, fmt.Errorf("invalid ENABLED expression")
	}
	lexical := &LexicalScope{scope: snapshotScope(e.scope), hasOperators: e.hasOperators}
	result, err := e.context.Enabled(expr.Args[0], e.context.Current, lexical)
	return value.Bool(result), err
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
