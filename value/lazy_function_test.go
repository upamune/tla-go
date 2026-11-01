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

func TestLazyFunctionProductMaterialization(t *testing.T) {
	empty, _ := Set(nil)
	nested, _ := ProductSet([]Value{empty, Integers(), NaturalNumbers()})
	domain, _ := ProductSet([]Value{Interval(1, 2000000000), nested})
	calls := 0
	makeFunction := func() Value {
		f, err := LazyFunction(domain, func(k, _ Value) (Value, error) {
			calls++
			return k, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	f, g := makeFunction(), makeFunction()
	checks := map[string]func() error{
		"equal":       func() error { _, err := Equal(f, f); return err },
		"compare":     func() error { _, err := Compare(f, g); return err },
		"pairs":       func() error { _, err := f.Pairs(); return err },
		"key":         func() error { _, err := f.Key(); return err },
		"fingerprint": func() error { _, err := TLCFingerprint([]Value{f}); return err },
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			var tlcErr *TLCError
			err := check()
			if !errors.As(err, &tlcErr) || err.Error() != "Attempted to compute the number of elements in the overridden value Int." {
				t.Fatalf("error = %v", err)
			}
		})
	}
	if calls != 0 {
		t.Fatalf("body called %d times", calls)
	}
}

func TestLazyProductSizeStopsAtUnknown(t *testing.T) {
	unknown := Sequences(Interval(1, 1))
	domain, _ := ProductSet([]Value{unknown, NaturalNumbers()})
	if _, known, err := lazyProductSize(domain); known || err != nil {
		t.Fatalf("size known = %v, error = %v", known, err)
	}
}

func TestLazyFunctionSymbolicDomainOverflow(t *testing.T) {
	one, _ := Integer(1)
	field, _ := RecordSet([]Pair{{String("k"), Interval(1, 1)}})
	product, _ := ProductSet([]Value{field, Interval(1, 50000), Interval(1, 50000)})
	functionSet, _ := FunctionSet(Interval(1, 31), Interval(1, 2))
	for _, domain := range []Value{product, functionSet} {
		calls := 0
		fn, _ := LazyFunction(domain, func(_, _ Value) (Value, error) { calls++; return one, nil })
		checks := []func() error{
			func() error { _, err := fn.Pairs(); return err },
			func() error { _, err := fn.Key(); return err },
			func() error { _, err := TLCFingerprint([]Value{fn}); return err },
			func() error { _, err := Equal(fn, fn); return err },
		}
		for _, check := range checks {
			if allocs := testing.AllocsPerRun(3, func() {
				if err := check(); err == nil || !strings.Contains(err.Error(), "Overflow when computing the number of elements") {
					t.Fatalf("error = %v", err)
				}
			}); allocs > 100 {
				t.Fatalf("overflow used %.0f allocations", allocs)
			}
		}
		if calls != 0 {
			t.Fatalf("body called %d times", calls)
		}
	}
}

func TestLazyRecordSetSizeConservative(t *testing.T) {
	for _, tc := range []struct {
		fields []Pair
		size   int64
		known  bool
	}{
		{nil, 1, true},
		{[]Pair{{String("a"), Interval(1, 2)}, {String("b"), Interval(1, 3)}}, 6, true},
		{[]Pair{{String("a"), Interval(1, 0)}, {String("b"), Interval(1, 50000)}, {String("c"), Interval(1, 50000)}}, 0, false},
		{[]Pair{{String("a"), NaturalNumbers()}, {String("b"), Integers()}}, 0, false},
		{[]Pair{{String("a"), NaturalNumbers()}, {String("b"), Interval(1, 50000)}, {String("c"), Interval(1, 50000)}}, 0, false},
	} {
		record, err := RecordSet(tc.fields)
		if err != nil {
			t.Fatal(err)
		}
		size, known, err := lazyProductSize(record)
		if size != tc.size || known != tc.known || err != nil {
			t.Fatalf("size = %d, known = %v, error = %v", size, known, err)
		}
	}
}

func TestLazyRecordSetOrderIndependentErrors(t *testing.T) {
	inner, _ := RecordSet([]Pair{{String("a"), Interval(1, 50000)}, {String("b"), Interval(1, 50000)}})
	for _, fields := range [][]Pair{
		{{String("a"), Interval(1, 0)}, {String("b"), NaturalNumbers()}},
		{{String("a"), Interval(1, 50000)}, {String("b"), Interval(1, 50000)}},
		{{String("a"), inner}, {String("b"), Interval(1, 1)}},
	} {
		record, _ := RecordSet(fields)
		domain, _ := ProductSet([]Value{record, Interval(1, 1)})
		calls := 0
		fn, _ := LazyFunction(domain, func(k, _ Value) (Value, error) { calls++; return k, nil })
		if allocs := testing.AllocsPerRun(3, func() {
			if _, err := fn.Pairs(); err == nil {
				t.Fatal("expected size error")
			}
		}); allocs > 100 || calls != 0 {
			t.Fatalf("allocations = %.0f, body calls = %d", allocs, calls)
		}
	}
}

func TestLazyProductOverflowDoesNotEnumerate(t *testing.T) {
	empty, _ := Set(nil)
	for _, last := range []Value{empty, Interval(1, 1), NaturalNumbers()} {
		domain, _ := ProductSet([]Value{Interval(1, 50000), Interval(1, 50000), last})
		calls := 0
		fn, _ := LazyFunction(domain, func(k, _ Value) (Value, error) { calls++; return k, nil })
		_, err := fn.Pairs()
		if err == nil || !strings.Contains(err.Error(), "Overflow when computing the number of elements") || calls != 0 {
			t.Fatalf("error = %v, body calls = %d", err, calls)
		}
	}
	nested, _ := ProductSet([]Value{Interval(1, 50000), Interval(1, 50000)})
	domain, _ := ProductSet([]Value{nested, NaturalNumbers()})
	fn, _ := LazyFunction(domain, func(k, _ Value) (Value, error) { t.Fatal("body evaluated"); return k, nil })
	if allocs := testing.AllocsPerRun(3, func() {
		if _, err := fn.Pairs(); err == nil {
			t.Fatal("expected overflow")
		}
	}); allocs > 100 {
		t.Fatalf("overflow used %.0f allocations", allocs)
	}
}

func TestLazyProductOverflowDoesNotRenderLaterFactors(t *testing.T) {
	calls := 0
	item, _ := LazyFunction(Interval(1, 1), func(k, _ Value) (Value, error) { calls++; return k, nil })
	// The payload is built directly so constructing the set does not itself
	// normalize the lazy function before the overflow check.
	later := Value{kind: SetKind, data: &payload{elements: []Value{item}}}
	domain, _ := ProductSet([]Value{Interval(1, 50000), Interval(1, 50000), later})
	fn, _ := LazyFunction(domain, func(k, _ Value) (Value, error) { calls++; return k, nil })
	if _, err := fn.Pairs(); err == nil || calls != 0 {
		t.Fatalf("error = %v, body calls = %d", err, calls)
	}
}
