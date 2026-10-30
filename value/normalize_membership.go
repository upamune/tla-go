package value

// DeepNormalizedMembership reflects the normalization caused by fingerprinting.
// It retains value immutability and only copies payloads that change.
func (v Value) DeepNormalizedMembership() Value {
	if v.data == nil {
		return v
	}
	data := normalizeMembershipPayload(v.data)
	if data == nil {
		return v
	}
	v.data = data
	return v
}

func normalizeMembershipPayload(data *payload) *payload {
	elements, elementsChanged := normalizeMembershipValues(data.elements)
	iteration, iterationChanged := normalizeMembershipValues(data.iteration)
	pairs, pairsChanged := normalizeMembershipPairs(data.pairs)
	domain, codomain := data.domain.DeepNormalizedMembership(), data.codomain.DeepNormalizedMembership()
	if data.membership == nil && !elementsChanged && !iterationChanged && !pairsChanged && domain == data.domain && codomain == data.codomain {
		return nil
	}
	result := *data
	result.membership = nil
	result.elements, result.iteration, result.pairs = elements, iteration, pairs
	result.domain, result.codomain = domain, codomain
	return &result
}

func normalizeMembershipValues(items []Value) ([]Value, bool) {
	var result []Value
	for i, item := range items {
		normalized := item.DeepNormalizedMembership()
		if normalized != item && result == nil {
			result = append([]Value(nil), items...)
		}
		if result != nil {
			result[i] = normalized
		}
	}
	if result == nil {
		return items, false
	}
	return result, true
}

func normalizeMembershipPairs(pairs []Pair) ([]Pair, bool) {
	var result []Pair
	for i, pair := range pairs {
		key, item := pair.Key.DeepNormalizedMembership(), pair.Value.DeepNormalizedMembership()
		if (key != pair.Key || item != pair.Value) && result == nil {
			result = append([]Pair(nil), pairs...)
		}
		if result != nil {
			result[i] = Pair{Key: key, Value: item}
		}
	}
	if result == nil {
		return pairs, false
	}
	return result, true
}
