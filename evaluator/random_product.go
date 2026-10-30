package evaluator

import "github.com/upamune/tla-go/value"

func (e *evaluator) drawFunction(set value.Value, random *JavaRandom) (value.Value, error) {
	domain, codomain, err := set.FunctionSetParts()
	if err != nil {
		return value.Value{}, err
	}
	keys, err := e.randomElements(domain)
	if err != nil {
		return value.Value{}, err
	}
	pairs := make([]value.Pair, len(keys))
	for i, key := range keys {
		v, err := e.drawRandomElement(codomain, random)
		if err != nil {
			return value.Value{}, err
		}
		pairs[i] = value.Pair{Key: key, Value: v}
	}
	return value.Function(pairs)
}

func (e *evaluator) drawRecord(set value.Value, random *JavaRandom) (value.Value, error) {
	fields, err := set.RecordSetFields()
	if err != nil {
		return value.Value{}, err
	}
	ordered, err := e.orderedRecord(fields)
	if err != nil {
		return value.Value{}, err
	}
	fields, err = e.orderedPairs(ordered)
	if err != nil {
		return value.Value{}, err
	}
	for i := range fields {
		fields[i].Value, err = e.drawRandomElement(fields[i].Value, random)
		if err != nil {
			return value.Value{}, err
		}
	}
	return e.orderedRecord(fields)
}

func (e *evaluator) drawProduct(set value.Value, random *JavaRandom) (value.Value, error) {
	factors, err := set.ProductFactors()
	if err != nil {
		return value.Value{}, err
	}
	items := make([]value.Value, len(factors))
	for i, factor := range factors {
		items[i], err = e.drawRandomElement(factor, random)
		if err != nil {
			return value.Value{}, err
		}
	}
	return value.Tuple(items)
}
