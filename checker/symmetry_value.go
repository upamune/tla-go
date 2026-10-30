package checker

import "github.com/upamune/tla-go/value"

func permuteValue(v value.Value, p permutation) (value.Value, error) {
	if p.cache != nil && (v.Kind() == value.SetKind || v.Kind() == value.FunctionKind) {
		return cachedPermutation(v, p)
	}
	return computePermutedValue(v, p)
}

func cachedPermutation(v value.Value, p permutation) (value.Value, error) {
	if cached, ok := p.cache[v]; ok {
		return cached, nil
	}
	result, err := computePermutedValue(v, p)
	if err == nil && len(p.cache) < 4096 {
		p.cache[v] = result
	}
	return result, err
}

func computePermutedValue(v value.Value, p permutation) (value.Value, error) {
	switch v.Kind() {
	case value.ModelKind:
		name, _ := v.Text()
		if image, ok := p.images[name]; ok {
			return image, nil
		}
		return v, nil
	case value.FunctionKind, value.LazyFunctionKind:
		return permuteFunction(v, p)
	case value.IntervalKind:
		return v, nil
	default:
		if v.IsSet() && v.IsEnumerable() {
			return permuteSet(v, p)
		}
		return v, nil
	}
}

func permuteSet(v value.Value, p permutation) (value.Value, error) {
	elements, err := v.Elements()
	if err != nil {
		return value.Value{}, err
	}
	changed := false
	for i, element := range elements {
		elements[i], err = permuteValue(element, p)
		if err != nil {
			return value.Value{}, err
		}
		changed = changed || elements[i] != element
	}
	if !changed {
		return v, nil
	}
	return value.Set(elements)
}

func permuteFunction(v value.Value, p permutation) (value.Value, error) {
	if v.Kind() == value.LazyFunctionKind {
		domain, err := v.FunctionDomain()
		if err != nil {
			return value.Value{}, err
		}
		if !domain.IsEnumerable() {
			return v, nil
		}
	}
	pairs, err := v.Pairs()
	if err != nil {
		return value.Value{}, err
	}
	changed, err := permutePairs(pairs, p)
	if err != nil {
		return value.Value{}, err
	}
	if !changed {
		return v, nil
	}
	return value.Function(pairs)
}

func permutePairs(pairs []value.Pair, p permutation) (bool, error) {
	changed := false
	for i, pair := range pairs {
		key, err := permuteValue(pair.Key, p)
		if err != nil {
			return false, err
		}
		val, err := permuteValue(pair.Value, p)
		if err != nil {
			return false, err
		}
		changed = changed || key != pair.Key || val != pair.Value
		pairs[i] = value.Pair{Key: key, Value: val}
	}
	return changed, nil
}
