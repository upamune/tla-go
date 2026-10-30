package value

import "fmt"

func isLazySet(v Value) bool {
	switch v.kind {
	case FunctionSetKind, RecordSetKind, ProductSetKind, SubsetKind, UnionKind, SetOperationKind, FilteredSetKind:
		return true
	default:
		return false
	}
}

// RecordSet constructs a set of records, where each pair maps a field name to
// its set of permitted values. Fields are copied and sorted by name.
func RecordSet(fields []Pair) (Value, error) {
	for _, field := range fields {
		if field.Key.kind != StringKind || !isSet(field.Value) {
			return Value{}, fmt.Errorf("record set requires field names and sets")
		}
	}
	f, err := Function(fields)
	if err != nil {
		return Value{}, err
	}
	if len(f.data.pairs) != len(fields) {
		return Value{}, fmt.Errorf("duplicate record set field")
	}
	return Value{kind: RecordSetKind, data: f.data}, nil
}

// RecordSetFields returns the field names and corresponding value sets.
func (v Value) RecordSetFields() ([]Pair, error) {
	if v.kind != RecordSetKind {
		return nil, fmt.Errorf("expected a record set")
	}
	return append([]Pair(nil), v.data.pairs...), nil
}

// ProductSet constructs a Cartesian product of finite sets.
func ProductSet(factors []Value) (Value, error) {
	pairs := make([]Pair, len(factors))
	for i, factor := range factors {
		if !isSet(factor) {
			return Value{}, fmt.Errorf("Cartesian product requires set factors")
		}
		key, err := Integer(int64(i) + 1)
		if err != nil {
			return Value{}, err
		}
		pairs[i] = Pair{key, factor}
	}
	return Value{kind: ProductSetKind, data: &payload{pairs: pairs}}, nil
}

func (v Value) containsCartesian(element Value) (bool, error) {
	if element.kind == ModelKind {
		return false, nil
	}
	if element.kind != FunctionKind {
		return false, fmt.Errorf("expected a function in Cartesian set membership")
	}
	pairs := element.data.pairs
	if v.kind == RecordSetKind && !recordDomain(pairs) {
		return false, fmt.Errorf("expected a record in record set membership")
	}
	if v.kind == ProductSetKind && !tupleDomain(pairs) {
		return nonTupleMembership(pairs)
	}
	return v.cartesianPairsMatch(pairs)
}

func nonTupleMembership(pairs []Pair) (bool, error) {
	for _, pair := range pairs {
		if pair.Key.kind != IntegerKind {
			return false, fmt.Errorf("expected a tuple in Cartesian product membership")
		}
	}
	return false, nil
}

func (v Value) cartesianPairsMatch(pairs []Pair) (bool, error) {
	if len(pairs) != len(v.data.pairs) {
		return false, nil
	}
	for i, pair := range pairs {
		factor := v.data.pairs[i]
		equal, err := Equal(pair.Key, factor.Key)
		if err != nil || !equal {
			return false, err
		}
		found, err := factor.Value.Contains(pair.Value)
		if err != nil || !found {
			return false, err
		}
	}
	return true, nil
}

func (v Value) eachCartesian(visit func(Value) (bool, error)) error {
	empty, err := v.isEmpty()
	if err != nil || empty {
		return err
	}
	pairs := make([]Pair, len(v.data.pairs))
	for i, factor := range v.data.pairs {
		pairs[i].Key = factor.Key
	}
	_, err = enumerateCartesian(v.data.pairs, pairs, 0, visit)
	return err
}

func enumerateCartesian(factors, pairs []Pair, index int, visit func(Value) (bool, error)) (bool, error) {
	if index == len(pairs) {
		function, err := Function(pairs)
		if err != nil {
			return false, err
		}
		return visit(function)
	}
	more := true
	err := factors[index].Value.Each(func(item Value) (bool, error) {
		pairs[index].Value = item
		var err error
		more, err = enumerateCartesian(factors, pairs, index+1, visit)
		return more, err
	})
	return more, err
}
