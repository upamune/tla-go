package value

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"weak"

	"github.com/upamune/tla-go/syntax/parser"
)

func TestUpdateExistingNoopReleasesExcessPairs(t *testing.T) {
	updated, discarded := updateDuplicateKeyFunction(t)
	if len(updated.data.pairs) != 1 || cap(updated.data.pairs) != 1 {
		t.Fatalf("updated pair slice: len=%d cap=%d", len(updated.data.pairs), cap(updated.data.pairs))
	}
	runtime.GC()
	if discarded.Value() != nil {
		t.Fatal("the updated function retains a discarded duplicate's value")
	}
	runtime.KeepAlive(updated)
}

func updateDuplicateKeyFunction(t *testing.T) (Value, weak.Pointer[payload]) {
	t.Helper()
	key, _ := Integer(1)
	pairs := make([]Pair, 100000)
	for i := range pairs {
		pairs[i] = Pair{Key: key, Value: Bool(false)}
	}
	discarded := String("discarded duplicate")
	pairs[1].Value = discarded
	ref := weak.Make(discarded.data)
	original, err := Function(pairs)
	if err != nil {
		t.Fatal(err)
	}
	if len(original.data.pairs) != 1 || cap(original.data.pairs) < len(pairs) {
		t.Fatalf("original pair slice: len=%d cap=%d", len(original.data.pairs), cap(original.data.pairs))
	}
	updated, found, err := original.UpdateExisting(key, func(v Value) (Value, error) { return v, nil })
	if err != nil || !found || updated == original {
		t.Fatalf("update: found=%v error=%v", found, err)
	}
	return updated, ref
}

func TestUpdateExistingNoopSharesOnlySafeSpareCapacity(t *testing.T) {
	key, _ := Integer(1)
	for _, tc := range []struct {
		name  string
		size  int
		stale bool
		share bool
	}{
		{"no spare", 1, false, true},
		{"bounded and empty", 2, false, true},
		{"bounded but occupied", 2, true, false},
		{"unbounded and empty", 3, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storage := make([]Pair, tc.size)
			storage[0] = Pair{Key: key, Value: Bool(false)}
			if tc.stale {
				storage[1] = Pair{Value: String("discarded")}
			}
			original := Value{kind: FunctionKind, data: &payload{pairs: storage[:1]}}
			updated, found, err := original.UpdateExisting(key, func(v Value) (Value, error) { return v, nil })
			if err != nil || !found || updated == original {
				t.Fatalf("update: found=%v error=%v", found, err)
			}
			if shared := &updated.data.pairs[0] == &original.data.pairs[0]; shared != tc.share {
				t.Fatalf("shared pair storage = %v; want %v", shared, tc.share)
			}
		})
	}
}

func TestUpdateExistingNoopEvaluatesCallback(t *testing.T) {
	one, _ := Integer(1)
	original, err := FunctionWithDomain([]Pair{{Key: one, Value: Bool(false)}}, Interval(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	updated, found, err := original.UpdateExisting(one, func(v Value) (Value, error) {
		calls++
		return v, nil
	})
	if err != nil || !found || calls != 1 {
		t.Fatalf("update: found=%v calls=%d error=%v", found, calls, err)
	}
	if updated == original {
		t.Fatal("no-op update must retain its distinct public Value identity")
	}
	if same, err := Equal(original, updated); err != nil || !same {
		t.Fatalf("unchanged function: %v, %v", same, err)
	}
	changed, found, err := updated.UpdateExisting(one, func(Value) (Value, error) { return Bool(true), nil })
	if err != nil || !found {
		t.Fatalf("subsequent update: found=%v error=%v", found, err)
	}
	pairs, err := updated.Pairs()
	if err != nil {
		t.Fatal(err)
	}
	pairs[0].Value = Bool(true)
	for _, fn := range []Value{original, updated} {
		got, err := fn.Lookup(one)
		if err != nil || got != Bool(false) {
			t.Fatalf("shared function mutated: %v, %v", got, err)
		}
	}
	if got, err := changed.Lookup(one); err != nil || got != Bool(true) {
		t.Fatalf("updated function: %v, %v", got, err)
	}
	want := errors.New("update failed")
	_, found, err = original.UpdateExisting(one, func(Value) (Value, error) {
		calls++
		return Value{}, want
	})
	if !found || calls != 2 || !errors.Is(err, want) {
		t.Fatalf("failure: found=%v calls=%d error=%v", found, calls, err)
	}
	two, _ := Integer(2)
	_, found, err = original.UpdateExisting(two, func(v Value) (Value, error) {
		calls++
		return v, nil
	})
	if found || calls != 2 || err != nil {
		t.Fatalf("out-of-domain: found=%v calls=%d error=%v", found, calls, err)
	}
}

func TestUpdateExistingNoopConcurrentBranches(t *testing.T) {
	one, _ := Integer(1)
	original, err := FunctionWithDomain([]Pair{{Key: one, Value: Bool(false)}}, Interval(1, 1))
	if err != nil {
		t.Fatal(err)
	}
	shared, found, err := original.UpdateExisting(one, func(v Value) (Value, error) { return v, nil })
	if err != nil || !found || shared == original {
		t.Fatalf("update: found=%v err=%v", found, err)
	}
	var group sync.WaitGroup
	errorsOut := make(chan error, 8)
	for range 8 {
		group.Go(func() {
			current := shared
			for i := range 200 {
				var updateErr error
				current, _, updateErr = current.UpdateExisting(one, func(Value) (Value, error) { return Bool(i%2 == 0), nil })
				if updateErr != nil {
					errorsOut <- updateErr
					return
				}
				for _, base := range []Value{original, shared} {
					got, readErr := base.Lookup(one)
					if readErr != nil || got != Bool(false) {
						errorsOut <- fmt.Errorf("shared value changed: %v, %v", got, readErr)
						return
					}
				}
			}
		})
	}
	group.Wait()
	close(errorsOut)
	for err := range errorsOut {
		t.Error(err)
	}
}

// Identity-based checker caches must see a new function even when the
// selected update is a no-op, including for deferred and source-bearing values.
func TestUpdateExistingNoopRetainsSensitiveIdentity(t *testing.T) {
	one, _ := Integer(1)
	two, _ := Integer(2)
	lazy, err := LazyFunction(Interval(1, 1), func(Value, Value) (Value, error) { return one, nil })
	if err != nil {
		t.Fatal(err)
	}
	nested, err := Function([]Pair{{Key: one, Value: Bool(false)}, {Key: two, Value: lazy}})
	if err != nil {
		t.Fatal(err)
	}
	withSource, err := TupleWithSource([]Value{Bool(false)}, &parser.Expression{Kind: "Tuple"})
	if err != nil {
		t.Fatal(err)
	}
	deferred, err := Filter(Interval(1, 1), func(Value) (bool, error) { return true, nil })
	if err != nil {
		t.Fatal(err)
	}
	withDeferredDomain, err := FunctionWithDomain([]Pair{{Key: one, Value: Bool(false)}}, deferred)
	if err != nil {
		t.Fatal(err)
	}
	stringKey := String("a")
	withStringKey, err := Function([]Pair{{Key: stringKey, Value: Bool(false)}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		original Value
		key      Value
	}{
		{"nested lazy", nested, one},
		{"source", withSource, one},
		{"deferred domain", withDeferredDomain, one},
		{"string key", withStringKey, stringKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			updated, found, err := tc.original.UpdateExisting(tc.key, func(v Value) (Value, error) { return v, nil })
			if err != nil || !found || updated == tc.original {
				t.Fatalf("unsafe identity reuse: found=%v error=%v", found, err)
			}
		})
	}
}
