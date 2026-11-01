package value

import (
	"errors"
	"sync"
	"testing"
)

func TestUnionFirstElementEnumeration(t *testing.T) {
	one, err := Integer(1)
	if err != nil {
		t.Fatal(err)
	}
	base, err := Set([]Value{one})
	if err != nil {
		t.Fatal(err)
	}
	union, err := Union(base)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			called := false
			err := union.Each(func(Value) (bool, error) { called = true; return true, nil })
			var first *UnionFirstElementError
			var unexpected *UnexpectedError
			if !errors.As(err, &first) || !errors.As(err, &unexpected) || !unexpected.TLCAssert || called {
				t.Errorf("Each: called=%t, error=%v", called, err)
			}
		})
	}
	wg.Wait()
	_, err = union.Key()
	var first *UnionFirstElementError
	if !errors.As(err, &first) {
		t.Fatalf("Key error = %v", err)
	}
	_, err = TLCFingerprint([]Value{union})
	if !errors.As(err, &first) {
		t.Fatalf("TLCFingerprint error = %v", err)
	}
}

func TestUnionEnumerableControl(t *testing.T) {
	base, err := Set([]Value{Interval(1, 2), Interval(2, 3)})
	if err != nil {
		t.Fatal(err)
	}
	union, err := Union(base)
	if err != nil {
		t.Fatal(err)
	}
	elements, err := union.Elements()
	if err != nil || len(elements) != 3 {
		t.Fatalf("Elements = %v, %v", elements, err)
	}
	explicit, err := Set(elements)
	if err != nil {
		t.Fatal(err)
	}
	if key(t, union) != key(t, explicit) {
		t.Fatal("union and enumerated set have different keys")
	}
	got, err := TLCFingerprint([]Value{union})
	if err != nil {
		t.Fatal(err)
	}
	want, err := TLCFingerprint([]Value{explicit})
	if err != nil || got != want {
		t.Fatalf("fingerprints: union %d, explicit %d, error %v", got, want, err)
	}
}

func TestUnionFirstElementFilterEvaluation(t *testing.T) {
	calls := 0
	base, err := Filter(Interval(1, 3), func(Value) (bool, error) {
		calls++
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	union, err := Union(base)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("predicate called during construction: %d", calls)
	}
	err = union.Each(func(Value) (bool, error) { t.Fatal("unexpected visitor call"); return false, nil })
	var first *UnionFirstElementError
	if !errors.As(err, &first) || calls != 1 {
		t.Fatalf("Each error %v, predicate calls %d", err, calls)
	}
}
