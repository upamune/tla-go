package value

import (
	"errors"
	"strings"
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

func TestLazyFunctionEqualityMaterializesInOrderAndCaches(t *testing.T) {
	var calls []string
	makeFunction := func(name string) Value {
		fn, err := LazyFunction(Interval(1, 2), func(k, _ Value) (Value, error) {
			calls = append(calls, name+k.String())
			return k, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return fn
	}
	a, b := makeFunction("a"), makeFunction("b")
	for range 2 {
		equal, err := Equal(a, b)
		if err != nil || !equal {
			t.Fatalf("equal = %v, error = %v", equal, err)
		}
	}
	if got := strings.Join(calls, ","); got != "a1,a2,b1,b2" {
		t.Fatalf("calls: %s", got)
	}
}

func TestLazyFunctionSelfEqualityEvaluatesFiniteBody(t *testing.T) {
	calls := 0
	fn, err := LazyFunction(Interval(1, 2), func(k, _ Value) (Value, error) {
		calls++
		return k, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if equal, err := Equal(fn, fn); err != nil || !equal || calls != 2 {
		t.Fatalf("equal = %v, error = %v, calls = %d", equal, err, calls)
	}
}

func TestLazyFunctionSelfEqualityPropagatesBodyError(t *testing.T) {
	want := errors.New("body failed")
	fn, err := LazyFunction(Interval(1, 1), func(_, _ Value) (Value, error) {
		return Value{}, want
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Equal(fn, fn); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestLazyFunctionAnyMaterialization(t *testing.T) {
	calls := 0
	fn, err := LazyFunction(Any(), func(k, _ Value) (Value, error) {
		calls++
		return k, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "Attempted to compute the number of elements in the overridden value ANY."
	checks := map[string]func() error{
		"equal":       func() error { _, err := Equal(fn, fn); return err },
		"pairs":       func() error { _, err := fn.Pairs(); return err },
		"key":         func() error { _, err := fn.Key(); return err },
		"fingerprint": func() error { _, err := TLCFingerprint([]Value{fn}); return err },
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			var tlcErr *TLCError
			if err := check(); !errors.As(err, &tlcErr) || err.Error() != want {
				t.Fatalf("error = %v, want TLCError %q", err, want)
			}
		})
	}
	if calls != 0 {
		t.Fatalf("body called %d times during materialization", calls)
	}
	key := String("key")
	got, err := fn.Lookup(key)
	if err != nil || got != key || calls != 1 {
		t.Fatalf("lookup = %v, %v; calls = %d", got, err, calls)
	}
}

func TestLazyFunctionFiniteMaterializationConsumers(t *testing.T) {
	fn, err := LazyFunction(Interval(1, 2), func(k, _ Value) (Value, error) { return k, nil })
	if err != nil {
		t.Fatal(err)
	}
	one, _ := Integer(1)
	two, _ := Integer(2)
	explicit, err := Tuple([]Value{one, two})
	if err != nil {
		t.Fatal(err)
	}
	pairs, err := fn.Pairs()
	if err != nil || len(pairs) != 2 {
		t.Fatalf("pairs = %v, %v", pairs, err)
	}
	lazyKey, err := fn.Key()
	if err != nil || lazyKey != key(t, explicit) {
		t.Fatalf("key = %q, %v", lazyKey, err)
	}
	got, err := TLCFingerprint([]Value{fn})
	if err != nil {
		t.Fatal(err)
	}
	want, err := TLCFingerprint([]Value{explicit})
	if err != nil || got != want {
		t.Fatalf("fingerprint = %d, want %d; error = %v", got, want, err)
	}
}
