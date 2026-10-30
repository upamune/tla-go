package value

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestLazyFunctionMemoizesRecursiveCalls(t *testing.T) {
	var calls atomic.Int32
	fib, err := LazyFunction(NaturalNumbers(), func(argument, self Value) (Value, error) {
		calls.Add(1)
		n, _ := argument.Int()
		if n < 2 {
			return Integer(int64(n))
		}
		aKey, _ := Integer(int64(n - 1))
		bKey, _ := Integer(int64(n - 2))
		a, err := self.Lookup(aKey)
		if err != nil {
			return Value{}, err
		}
		b, err := self.Lookup(bKey)
		if err != nil {
			return Value{}, err
		}
		av, _ := a.Int()
		bv, _ := b.Int()
		return Integer(int64(av) + int64(bv))
	})
	if err != nil {
		t.Fatal(err)
	}
	input, _ := Integer(30)
	result, err := fib.Lookup(input)
	if err != nil || result.String() != "832040" {
		t.Fatalf("got %s, %v", result, err)
	}
	if got := calls.Load(); got != 31 {
		t.Fatalf("computed %d values, want 31", got)
	}
	var group sync.WaitGroup
	for range 16 {
		group.Go(func() {
			got, err := fib.Lookup(input)
			if err != nil || got.String() != "832040" {
				t.Errorf("got %s, %v", got, err)
			}
		})
	}
	group.Wait()
	if got := calls.Load(); got != 31 {
		t.Fatalf("cached result recomputed %d times", got)
	}
}

func TestLazyFunctionMatchesFiniteFunctionKey(t *testing.T) {
	lazy, err := LazyFunction(Interval(1, 2), func(k, _ Value) (Value, error) { return k, nil })
	if err != nil {
		t.Fatal(err)
	}
	one, _ := Integer(1)
	two, _ := Integer(2)
	explicit, _ := Tuple([]Value{one, two})
	equal, err := Equal(lazy, explicit)
	if err != nil || !equal || key(t, lazy) != key(t, explicit) {
		t.Fatalf("equality: %v, %v", equal, err)
	}
	outside, _ := Integer(3)
	if _, err := lazy.Lookup(outside); err == nil {
		t.Fatal("out-of-domain argument accepted")
	}
}

func TestLazyFunctionCachesEvaluationModesSeparately(t *testing.T) {
	var calls atomic.Int32
	fn, err := LazyFunctionWithControl(Interval(1, 1), "", func(_, _ Value, keepLazy bool) (Value, error) {
		calls.Add(1)
		return Bool(keepLazy), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	key, _ := Integer(1)
	for _, keepLazy := range []bool{true, false, true, false} {
		got, err := fn.LookupWithLazyResult(key, keepLazy)
		if result, ok := got.Bool(); err != nil || !ok || result != keepLazy {
			t.Fatalf("keepLazy=%v: got %v, %v", keepLazy, got, err)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("computed %d times, want 2", got)
	}
}
