package evaluator

import (
	"math"
	"math/big"

	"github.com/upamune/tla-go/value"
)

type randomProduct struct {
	kind  value.Kind
	keys  []value.Value
	lists [][]value.Value
}

func (e *evaluator) subsetProduct(count int, set value.Value, random *JavaRandom) ([]value.Value, error) {
	product, err := e.randomProduct(set)
	if err != nil {
		return nil, err
	}
	size := big.NewInt(1)
	for _, list := range product.lists {
		size.Mul(size, big.NewInt(int64(len(list))))
	}
	if size.Sign() == 0 {
		return nil, nil
	}
	indices := randomProductIndices(count, size, random)
	result := make([]value.Value, len(indices))
	for i, index := range indices {
		result[i], err = e.productValue(product, index)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (e *evaluator) randomProduct(set value.Value) (randomProduct, error) {
	product := randomProduct{kind: set.Kind()}
	if set.Kind() == value.FunctionSetKind {
		domain, codomain, err := set.FunctionSetParts()
		if err != nil {
			return product, err
		}
		product.keys, err = e.randomElements(domain)
		if err != nil {
			return product, err
		}
		items, err := e.randomElements(codomain)
		if err != nil {
			return product, err
		}
		for range product.keys {
			product.lists = append(product.lists, items)
		}
		return product, nil
	}
	return e.randomCartesianProduct(set, product)
}

func (e *evaluator) randomCartesianProduct(set value.Value, product randomProduct) (randomProduct, error) {
	fields, err := e.randomCartesianFields(set)
	if err != nil {
		return product, err
	}
	for _, field := range fields {
		items, err := e.randomElements(field.Value)
		if err != nil {
			return product, err
		}
		product.keys = append(product.keys, field.Key)
		product.lists = append(product.lists, items)
	}
	return product, nil
}

func (e *evaluator) randomCartesianFields(set value.Value) ([]value.Pair, error) {
	if set.Kind() == value.RecordSetKind {
		return e.randomRecordFields(set)
	}
	factors, err := set.ProductFactors()
	if err != nil {
		return nil, err
	}
	fields := make([]value.Pair, len(factors))
	for i, factor := range factors {
		fields[i] = value.Pair{Value: factor}
	}
	return fields, nil
}

func (e *evaluator) randomRecordFields(set value.Value) ([]value.Pair, error) {
	fields, err := set.RecordSetFields()
	if err != nil {
		return nil, err
	}
	ordered, err := e.orderedRecord(fields)
	if err != nil {
		return nil, err
	}
	return e.orderedPairs(ordered)
}

func randomProductIndices(count int, size *big.Int, random *JavaRandom) []*big.Int {
	result := make([]*big.Int, count)
	if size.IsInt64() && size.Int64() <= math.MaxInt32 {
		for i, n := range subsetIndices(count, size.Int64(), random) {
			result[i] = big.NewInt(n)
		}
		return result
	}
	offset := random.Long()
	if offset < 0 {
		offset = -offset
	}
	current := big.NewInt(offset)
	stride := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 63), big.NewInt(25))
	for i := range result {
		result[i] = new(big.Int).Mod(current, size)
		current.Add(current, stride)
	}
	return result
}

func (e *evaluator) productValue(product randomProduct, index *big.Int) (value.Value, error) {
	remaining := new(big.Int).Set(index)
	items := make([]value.Value, len(product.lists))
	for i := len(items) - 1; i >= 0; i-- {
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(remaining, big.NewInt(int64(len(product.lists[i]))), remainder)
		items[i] = product.lists[i][remainder.Int64()]
		remaining = quotient
	}
	if product.kind == value.ProductSetKind {
		return value.Tuple(items)
	}
	pairs := make([]value.Pair, len(items))
	for i := range items {
		pairs[i] = value.Pair{Key: product.keys[i], Value: items[i]}
	}
	if product.kind == value.RecordSetKind {
		return e.orderedRecord(pairs)
	}
	return value.Function(pairs)
}
