// Package value implements runtime values used by the TLA+ evaluator.
// Its semantics follow tla-ts; see UPSTREAM-LICENSE.
package value

import (
	"fmt"
	"math"

	"github.com/upamune/tla-go/syntax/parser"
)

type Kind uint8

const (
	Invalid Kind = iota
	IntegerKind
	BooleanKind
	StringKind
	ModelKind
	SetKind
	FunctionKind
	IntervalKind
	FunctionSetKind
	RecordSetKind
	ProductSetKind
	SubsetKind
	UnionKind
	SequenceSetKind
	InfiniteSetKind
	SetOperationKind
	FilteredSetKind
	LazyFunctionKind
)

// Value is immutable. Constructors copy their inputs and accessors return copies.
type Value struct {
	kind         Kind
	number, high int32
	data         *payload
}
type payload struct {
	text             string
	elements         []Value
	iteration        []Value
	pairs            []Pair
	displayOrder     []string
	domain, codomain Value
	test             func(Value) (bool, error)
	lazy             *lazyFunction
	source           *parser.Expression
}
type Pair struct{ Key, Value Value }

func (v Value) Kind() Kind { return v.kind }

// IsEnumerable reports whether this set can be enumerated.
func (v Value) IsEnumerable() bool { return v.enumerable() }

// IsSet reports whether this value represents a set.
func (v Value) IsSet() bool { return isSet(v) }

func Integer(n int64) (Value, error) {
	if n < math.MinInt32 || n > math.MaxInt32 {
		return Value{}, fmt.Errorf("integer overflow: %d is outside TLC's 32-bit range", n)
	}
	return Value{kind: IntegerKind, number: int32(n)}, nil
}

func Bool(b bool) Value {
	if b {
		return Value{kind: BooleanKind, number: 1}
	}
	return Value{kind: BooleanKind}
}
func String(s string) Value          { return Value{kind: StringKind, data: &payload{text: s}} }
func Model(name string) Value        { return Value{kind: ModelKind, data: &payload{text: name}} }
func Interval(low, high int32) Value { return Value{kind: IntervalKind, number: low, high: high} }
func (v Value) Int() (int32, bool)   { return v.number, v.kind == IntegerKind }
func (v Value) Bool() (bool, bool)   { return v.number != 0, v.kind == BooleanKind }
func (v Value) Text() (string, bool) {
	if v.kind != StringKind && v.kind != ModelKind {
		return "", false
	}
	return v.data.text, true
}
func (v Value) Range() (int32, int32, bool) { return v.number, v.high, v.kind == IntervalKind }
func (v Value) Elements() ([]Value, error) {
	switch v.kind {
	case SetKind:
		return append([]Value(nil), v.setIteration()...), nil
	case IntervalKind:
		return intervalElements(v.number, v.high), nil
	case FunctionSetKind, RecordSetKind, ProductSetKind, SubsetKind, UnionKind, SetOperationKind, FilteredSetKind:
		set, err := v.finiteSet()
		if err != nil {
			return nil, err
		}
		return set.Elements()
	default:
		return nil, fmt.Errorf("expected an enumerable set")
	}
}

func intervalElements(low, high int32) []Value {
	if high < low {
		return nil
	}
	values := make([]Value, 0, int64(high)-int64(low)+1)
	for n := int64(low); n <= int64(high); n++ {
		values = append(values, Value{kind: IntegerKind, number: int32(n)})
	}
	return values
}

func (v Value) Pairs() ([]Pair, error) {
	if v.kind == LazyFunctionKind {
		return v.lazyPairs()
	}
	if v.kind != FunctionKind {
		return nil, fmt.Errorf("expected a function")
	}
	return append([]Pair(nil), v.data.pairs...), nil
}
