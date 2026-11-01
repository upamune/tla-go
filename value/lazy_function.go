package value

import (
	"fmt"
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
