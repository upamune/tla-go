package evaluator

import (
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// scope is one lexical frame. Parameters and bound variables use the small
// bound slice; values holds caller-supplied maps such as checker locals.
type scope struct {
	parent      *scope
	bound       []binding
	values      map[string]value.Value
	definitions map[string]parser.Definition
	operators   map[string]operatorRef
}

// binding names a value. An operator argument evaluated on demand has arg set
// and a placeholder value, which identifier lookup never returns.
type binding struct {
	name  string
	value value.Value
	arg   *primeArgument
}

// lookup returns the binding of name. A later duplicate shadows an earlier one,
// as an assignment to a map would.
func (s *scope) lookup(name string) (*binding, bool) {
	for i := len(s.bound) - 1; i >= 0; i-- {
		if s.bound[i].name == name {
			return &s.bound[i], true
		}
	}
	return nil, false
}

func (s *scope) value(name string) (value.Value, bool) {
	if bound, ok := s.lookup(name); ok {
		return bound.value, true
	}
	v, ok := s.values[name]
	return v, ok
}

func (s *scope) hasValue(name string) bool {
	_, ok := s.value(name)
	return ok
}

// evalInScope evaluates expr in s and then restores every field, so callers
// observe the same receiver as if a copy had evaluated expr. Values that
// outlive the call capture snapshots rather than the receiver.
func (e *evaluator) evalInScope(s *scope, expr *parser.Expression) (value.Value, error) {
	saved := *e
	e.scope = s
	result, err := e.eval(expr)
	*e = saved
	return result, err
}

func valueScope(parent *scope, names []string, values []value.Value) *scope {
	bound := make([]binding, len(names))
	for i, name := range names {
		bound[i] = binding{name: name, value: values[i]}
	}
	return &scope{parent: parent, bound: bound}
}

func singleValueScope(parent *scope, name string, v value.Value) *scope {
	frame := &struct {
		scope   scope
		binding [1]binding
	}{}
	frame.binding[0] = binding{name: name, value: v}
	frame.scope = scope{parent: parent, bound: frame.binding[:]}
	return &frame.scope
}
