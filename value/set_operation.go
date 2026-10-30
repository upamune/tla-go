package value

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

// SetOperation reduces intersections of explicit sets or intervals and keeps
// other set operations lazy.
func SetOperation(operator string, left, right Value) (Value, error) {
	if !isSet(left) || !isSet(right) {
		return Value{}, fmt.Errorf("set operation requires two sets")
	}
	if !validSetOperator(operator) {
		return Value{}, fmt.Errorf("unknown set operation %s", operator)
	}
	if operator == "\\cup" && emptyEnumeratedSet(left) {
		return right, nil
	}
	if candidate, other, ok := reducibleIntersection(operator, left, right); ok {
		return reduceIntersection(candidate, other)
	}
	return lazySetOperation(operator, left, right)
}

// SetOperationWithSource records the source of a newly reduced intersection.
func SetOperationWithSource(operator string, left, right Value, source *parser.Expression) (Value, error) {
	result, err := SetOperation(operator, left, right)
	if err == nil && operator == "\\cap" && result.kind == SetKind {
		result.data.source = source
	}
	return result, err
}

func lazySetOperation(operator string, left, right Value) (Value, error) {
	result := Value{kind: SetOperationKind, data: &payload{text: operator, domain: left, codomain: right}}
	if left.kind == SetKind && right.kind == SetKind {
		return result.finiteSet()
	}
	return result, nil
}

func reducibleSet(v Value) bool { return v.kind == SetKind || v.kind == IntervalKind }

func reducibleIntersection(operator string, left, right Value) (Value, Value, bool) {
	if operator == "\\cap" {
		if reducibleSet(left) {
			return left, right, true
		}
		if reducibleSet(right) {
			return right, left, true
		}
	}
	return Value{}, Value{}, false
}

func reduceIntersection(candidate, other Value) (Value, error) {
	if candidate.kind == IntervalKind && other.kind == IntervalKind {
		return intersectIntervals(candidate, other), nil
	}
	var members []Value
	err := candidate.Each(func(item Value) (bool, error) {
		included, err := other.Contains(item)
		if err != nil {
			return false, err
		}
		if included {
			members = append(members, item)
		}
		return true, nil
	})
	if err != nil {
		if candidate.kind == SetKind {
			err = candidate.withFingerprintSource(err)
		}
		return Value{}, err
	}
	return Set(members)
}

func intersectIntervals(left, right Value) Value {
	low := max(int64(left.number), int64(right.number))
	high := min(int64(left.high), int64(right.high))
	capacity := min(max(high-low+1, 0), 1024)
	members := make([]Value, 0, int(capacity))
	for n := low; n <= high; n++ {
		members = append(members, Value{kind: IntegerKind, number: int32(n)})
	}
	return Value{kind: SetKind, data: &payload{elements: members}}
}

func validSetOperator(operator string) bool {
	return operator == "\\cup" || operator == "\\cap" || operator == "\\"
}

func emptyEnumeratedSet(v Value) bool {
	return v.kind == SetKind && len(v.data.elements) == 0
}

func (v Value) containsOperation(element Value) (bool, error) {
	left, err := v.data.domain.Contains(element)
	if err != nil {
		return false, err
	}
	if v.data.text == "\\cup" && left {
		return true, nil
	}
	if v.data.text != "\\cup" && !left {
		return false, nil
	}
	right, err := v.data.codomain.Contains(element)
	if err != nil {
		return false, err
	}
	if v.data.text == "\\" {
		return !right, nil
	}
	return right, nil
}

func (v Value) eachOperation(visit func(Value) (bool, error)) error {
	if v.data.text == "\\cup" {
		return v.eachOperationUnion(visit)
	}
	candidate, test := v.data.domain, v.data.codomain
	if v.data.text == "\\cap" && !candidate.enumerable() {
		candidate, test = test, candidate
	}
	return candidate.Each(func(item Value) (bool, error) {
		included, err := test.Contains(item)
		if err != nil {
			return false, err
		}
		if included == (v.data.text == "\\cap") {
			return visit(item)
		}
		return true, nil
	})
}

func (v Value) eachOperationUnion(visit func(Value) (bool, error)) error {
	left, err := v.data.domain.Elements()
	if err != nil {
		return err
	}
	right, err := v.data.codomain.Elements()
	if err != nil {
		return err
	}
	merged, err := Set(append(left, right...))
	if err != nil {
		return err
	}
	return merged.Each(visit)
}

func (v Value) enumerable() bool {
	switch v.kind {
	case SetKind, IntervalKind:
		return true
	case SubsetKind, FilteredSetKind:
		return v.data.domain.enumerable()
	case FunctionSetKind:
		return v.functionEnumerable()
	case RecordSetKind, ProductSetKind:
		return v.productEnumerable()
	case SetOperationKind:
		return v.operationEnumerable()
	case UnionKind:
		return v.unionEnumerable()
	default:
		return false
	}
}

func (v Value) unionEnumerable() bool {
	if !v.data.domain.enumerable() {
		return false
	}
	all := true
	err := v.data.domain.Each(func(set Value) (bool, error) {
		all = set.enumerable()
		return all, nil
	})
	return err == nil && all
}

func (v Value) functionEnumerable() bool {
	empty, err := v.isEmpty()
	if err != nil {
		return false
	}
	domainEmpty, err := v.data.domain.isEmpty()
	return err == nil && (empty || domainEmpty || v.data.domain.enumerable() && v.data.codomain.enumerable())
}

func (v Value) productEnumerable() bool {
	empty, err := v.productIsEmpty()
	if err != nil {
		return false
	}
	if empty {
		return true
	}
	for _, pair := range v.data.pairs {
		if !pair.Value.enumerable() {
			return false
		}
	}
	return true
}

func (v Value) operationEnumerable() bool {
	left := v.data.domain.enumerable()
	if v.data.text == "\\" {
		return left
	}
	right := v.data.codomain.enumerable()
	if v.data.text == "\\cap" {
		return left || right
	}
	return left && right
}

func (v Value) operationIsFinite() (bool, error) {
	left, err := v.data.domain.IsFinite()
	if err != nil {
		return false, err
	}
	if v.data.text == "\\" && left {
		return true, nil
	}
	right, err := v.data.codomain.IsFinite()
	if err != nil {
		return false, err
	}
	return operationFiniteResult(v.data.text, left, right)
}

func operationFiniteResult(operator string, left, right bool) (bool, error) {
	switch operator {
	case "\\cup":
		return left && right, nil
	case "\\cap":
		if !left && !right {
			return false, fmt.Errorf("cannot determine finiteness of intersection")
		}
		return true, nil
	default:
		if !right {
			return false, fmt.Errorf("cannot determine finiteness of difference")
		}
		return false, nil
	}
}
