package value

import (
	"fmt"
	"math"
	"sync"
)

type lazyFunction struct {
	compute     func(Value, Value, bool) (Value, error)
	description string
	mu          sync.RWMutex
	cache       map[lazyFunctionKey]Value
}

type lazyFunctionKey struct {
	key      string
	keepLazy bool
}

// LazyFunction constructs a function whose body is evaluated on application.
// The callback receives a self reference for recursive definitions and must be
// concurrency-safe and capture immutable state.
func LazyFunction(domain Value, compute func(Value, Value) (Value, error)) (Value, error) {
	return LazyFunctionDescribed(domain, "", compute)
}

// LazyFunctionDescribed retains a symbolic definition for non-enumerable domains.
func LazyFunctionDescribed(domain Value, description string, compute func(Value, Value) (Value, error)) (Value, error) {
	if !domain.IsSet() || compute == nil {
		return Value{}, fmt.Errorf("lazy function requires a domain and body")
	}
	return LazyFunctionWithControl(domain, description, func(argument, self Value, _ bool) (Value, error) {
		return compute(argument, self)
	})
}

// LazyFunctionWithControl passes the caller's requested laziness to its body.
func LazyFunctionWithControl(domain Value, description string, compute func(Value, Value, bool) (Value, error)) (Value, error) {
	if !domain.IsSet() || compute == nil {
		return Value{}, fmt.Errorf("lazy function requires a domain and body")
	}
	return Value{kind: LazyFunctionKind, data: &payload{domain: domain, lazy: &lazyFunction{compute: compute, description: description, cache: map[lazyFunctionKey]Value{}}}}, nil
}

func (v Value) hasLazyDescription() bool {
	return v.kind == LazyFunctionKind && !v.data.domain.IsEnumerable() && v.data.lazy.description != ""
}

// FunctionDomain returns the mathematical domain of a function.
func (v Value) FunctionDomain() (Value, error) {
	if v.kind == LazyFunctionKind || (v.kind == FunctionKind && v.data.domain.kind != Invalid) {
		return v.data.domain, nil
	}
	pairs, err := v.Pairs()
	if err != nil {
		return Value{}, err
	}
	keys := make([]Value, len(pairs))
	for i, pair := range pairs {
		keys[i] = pair.Key
	}
	return Set(keys)
}

func (v Value) lazyLookup(key Value, keepLazy bool) (Value, error) {
	found, err := v.data.domain.Contains(key)
	if err != nil {
		return Value{}, err
	}
	if !found {
		return Value{}, ErrOutsideFunctionDomain
	}
	if v.number >= 4096 {
		return Value{}, fmt.Errorf("evaluation recursion limit exceeded")
	}
	return v.lazyCompute(key, keepLazy)
}

func (v Value) lazyCompute(key Value, keepLazy bool) (Value, error) {
	keyText, keyErr := key.Key()
	cacheKey := lazyFunctionKey{key: keyText, keepLazy: keepLazy}
	if keyErr == nil {
		v.data.lazy.mu.RLock()
		result, ok := v.data.lazy.cache[cacheKey]
		v.data.lazy.mu.RUnlock()
		if ok {
			return result, nil
		}
	}
	self := v
	self.number++
	result, err := v.data.lazy.compute(key, self, keepLazy)
	if err == nil && keyErr == nil {
		v.data.lazy.mu.Lock()
		v.data.lazy.cache[cacheKey] = result
		v.data.lazy.mu.Unlock()
	}
	return result, err
}

func (v Value) lazyPairs() ([]Pair, error) {
	// ANY accepts every lookup key, but is still a TLC overridden set, not
	// an enumerable wildcard domain when a function is materialized.
	if v.data.domain.kind == InfiniteSetKind {
		return nil, &TLCError{Message: fmt.Sprintf("Attempted to compute the number of elements in the overridden value %s.", v.data.domain)}
	}
	if v.data.domain.kind == ProductSetKind || v.data.domain.kind == FunctionSetKind {
		_, _, err := lazyProductSize(v.data.domain)
		if err != nil {
			return nil, err
		}
	}
	var pairs []Pair
	err := v.data.domain.Each(func(key Value) (bool, error) {
		item, err := v.Lookup(key)
		if err != nil {
			return false, err
		}
		pairs = append(pairs, Pair{key, item})
		return true, nil
	})
	return pairs, err
}

// TLC sizes function parameters before enumerating or evaluating their bodies.
// Only inspect sizes that are cheap and have no evaluation side effects; an
// unknown factor leaves the existing enumeration path intact.
func lazyProductSize(v Value) (int64, bool, error) {
	switch v.kind {
	case InfiniteSetKind:
		return 0, true, &TLCError{Message: fmt.Sprintf("Attempted to compute the number of elements in the overridden value %s.", v)}
	case SetKind:
		return int64(len(v.data.elements)), true, nil
	case IntervalKind:
		n, err := equalityIntervalSize(v)
		return n, true, err
	case SubsetKind:
		return lazySubsetSize(v)
	case FunctionSetKind:
		return lazyFunctionSetSize(v)
	case ProductSetKind:
		return lazyProductFactorsSize(v)
	case RecordSetKind:
		return lazyRecordSetSize(v)
	default:
		return 0, false, nil
	}
}

// Record fields are sorted in Go, but Java can size them in a different
// order. For multiple fields only use the result when every ordering is safe.
func lazyRecordSetSize(v Value) (int64, bool, error) {
	if len(v.data.pairs) <= 1 {
		return lazyProductFactorsSize(v)
	}
	sizes := recordSizeProduct{size: 1}
	for _, field := range v.data.pairs {
		if !sizes.addField(field.Value) {
			return 0, false, nil
		}
	}
	return sizes.result()
}

func (p recordSizeProduct) result() (int64, bool, error) {
	if p.overflow {
		if p.empty || p.fieldErr != nil {
			return 0, false, nil
		}
		return 0, true, lazySizeOverflow()
	}
	if p.fieldErr != nil {
		return 0, true, p.fieldErr
	}
	if p.empty {
		return 0, true, nil
	}
	return p.size, true, nil
}

type recordSizeProduct struct {
	size            int64
	empty, overflow bool
	fieldErr        error
}

func (p *recordSizeProduct) addField(field Value) bool {
	n, known, err := lazyProductSize(field)
	if !known {
		return false
	}
	if err != nil {
		if p.fieldErr != nil {
			return false
		}
		p.fieldErr = err
	} else {
		p.add(n)
	}
	return true
}

func (p *recordSizeProduct) add(n int64) {
	if n == 0 {
		p.empty = true
	} else if !p.overflow && p.size > math.MaxInt32/n {
		p.overflow = true
	} else if !p.overflow {
		p.size *= n
	}
}

func lazySubsetSize(v Value) (int64, bool, error) {
	n, known, err := lazyProductSize(v.data.domain)
	if err != nil || !known {
		return 0, known, err
	}
	if n >= 31 {
		message := "Overflow when computing the number of elements"
		if v.data.domain.kind == IntervalKind {
			message += " in:\nSUBSET " + v.data.domain.String()
		}
		return 0, true, &UnexpectedError{Message: message, TLCAssert: true, ToolCode: 2178}
	}
	return int64(1) << uint(n), true, nil
}

func lazyFunctionSetSize(v Value) (int64, bool, error) {
	domain, known, err := lazyProductSize(v.data.domain)
	if err != nil || !known {
		return 0, known, err
	}
	base, known, err := lazyProductSize(v.data.codomain)
	if err != nil || !known {
		return 0, known, err
	}
	return lazyFunctionSetPower(base, domain)
}

func lazyFunctionSetPower(base, domain int64) (int64, bool, error) {
	if domain == 0 || base == 1 {
		return 1, true, nil
	}
	if base == 0 {
		return 0, true, nil
	}
	n, err := checkedPower(base, domain)
	if err != nil {
		return 0, true, lazySizeOverflow()
	}
	return n, true, nil
}

func lazyProductFactorsSize(product Value) (int64, bool, error) {
	size := int64(1)
	for _, factor := range product.data.pairs {
		n, known, err := lazyProductSize(factor.Value)
		if err != nil || !known {
			return 0, known, err
		}
		if n != 0 && size > math.MaxInt32/n {
			return 0, true, lazySizeOverflow()
		}
		size *= n
	}
	return size, true, nil
}

func lazySizeOverflow() error {
	// Rendering arbitrary set elements can materialize lazy functions and run
	// their bodies while constructing the error itself.
	return &TLCError{Message: "Overflow when computing the number of elements"}
}

func (v Value) materializedFunction() (Value, error) {
	if v.kind != LazyFunctionKind {
		return v, nil
	}
	pairs, err := v.lazyPairs()
	if err != nil {
		return Value{}, err
	}
	return FunctionWithDomain(pairs, v.data.domain)
}

func compareLazyFunctions(a, b Value) (int, error) {
	if a.kind == LazyFunctionKind && b.kind == LazyFunctionKind && a.data == b.data {
		return 0, nil
	}
	left, err := a.materializedFunction()
	if err != nil {
		return 0, err
	}
	right, err := b.materializedFunction()
	if err != nil {
		return 0, err
	}
	return Compare(left, right)
}

func (v Value) lazyUpdate(key Value, update func(Value) (Value, error)) (Value, error) {
	found, err := v.data.domain.Contains(key)
	if err != nil || !found {
		return v, err
	}
	return LazyFunctionWithControl(v.data.domain, v.data.lazy.description, func(item, _ Value, keepLazy bool) (Value, error) {
		previous, err := v.LookupWithLazyResult(item, keepLazy)
		if err != nil {
			return Value{}, err
		}
		same, err := Equal(key, item)
		if err != nil || !same {
			return previous, err
		}
		return update(previous)
	})
}
