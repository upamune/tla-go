package value

import "fmt"

// FunctionSet represents the set of all functions with the given domain and
// values in the codomain. Membership does not enumerate the Cartesian product.
func FunctionSet(domain, codomain Value) (Value, error) {
	if !isSet(domain) || !isSet(codomain) {
		return Value{}, fmt.Errorf("function set requires a set domain and codomain")
	}
	return Value{kind: FunctionSetKind, data: &payload{domain: domain, codomain: codomain}}, nil
}

// FunctionSetParts returns the domain and codomain of a function set.
func (v Value) FunctionSetParts() (Value, Value, error) {
	if v.kind != FunctionSetKind {
		return Value{}, Value{}, fmt.Errorf("expected a function set")
	}
	return v.data.domain, v.data.codomain, nil
}

func (v Value) containsFunction(element Value) (bool, error) {
	if element.kind == ModelKind {
		return false, nil
	}
	if element.kind == LazyFunctionKind {
		fn, err := element.materializedFunction()
		if err != nil {
			return false, err
		}
		element = fn
	}
	return v.containsFiniteFunction(element)
}

func (v Value) containsFiniteFunction(element Value) (bool, error) {
	if element.kind != FunctionKind {
		return false, fmt.Errorf("expected a function in function set membership")
	}
	keys := make([]Value, len(element.data.pairs))
	for i, pair := range element.data.pairs {
		keys[i] = pair.Key
	}
	domain, err := Set(keys)
	if err != nil {
		return false, err
	}
	equal, err := Equal(v.data.domain, domain)
	if err != nil || !equal {
		return false, err
	}
	return v.containsRange(element.data.pairs)
}

func (v Value) containsRange(pairs []Pair) (bool, error) {
	for _, pair := range pairs {
		found, err := v.data.codomain.Contains(pair.Value)
		if err != nil || !found {
			return false, err
		}
	}
	return true, nil
}

func (v Value) eachFunction(visit func(Value) (bool, error)) error {
	empty, err := v.isEmpty()
	if err != nil || empty {
		return err
	}
	domainEmpty, err := v.data.domain.isEmpty()
	if err != nil {
		return err
	}
	if domainEmpty {
		f, err := Function(nil)
		if err != nil {
			return err
		}
		_, err = visit(f)
		return err
	}
	keys, err := v.data.domain.Elements()
	if err != nil {
		return err
	}
	pairs := make([]Pair, len(keys))
	for i, key := range keys {
		pairs[i].Key = key
	}
	_, err = enumerateFunctions(v.data.domain, v.data.codomain, pairs, 0, visit)
	return err
}

func enumerateFunctions(domain, codomain Value, pairs []Pair, index int, visit func(Value) (bool, error)) (bool, error) {
	if index == len(pairs) {
		function, err := FunctionWithDomain(pairs, domain)
		if err != nil {
			return false, err
		}
		return visit(function)
	}
	more := true
	err := codomain.Each(func(item Value) (bool, error) {
		pairs[index].Value = item
		var err error
		more, err = enumerateFunctions(domain, codomain, pairs, index+1, visit)
		return more, err
	})
	return more, err
}

func (v Value) finiteSet() (Value, error) {
	var elements []Value
	err := v.Each(func(item Value) (bool, error) {
		elements = append(elements, item)
		return true, nil
	})
	if err != nil {
		return Value{}, err
	}
	return Set(elements)
}
