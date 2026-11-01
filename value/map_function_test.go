package value

import (
	"errors"
	"testing"
)

func TestMapFunctionPairs(t *testing.T) {
	one, two, three := number(t, 1), number(t, 2), number(t, 3)
	original, err := Tuple([]Value{one, two, three})
	if err != nil {
		t.Fatal(err)
	}
	var order []int32
	unchanged, err := original.MapFunctionPairs(func(p Pair) (Pair, error) {
		i, _ := p.Key.Int()
		order = append(order, i)
		return p, nil
	})
	if err != nil || unchanged != original || !unchanged.IsTuple() {
		t.Fatalf("unchanged map: %v, %v", unchanged, err)
	}
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("callback order: %v", order)
	}
	changed, err := original.MapFunctionPairs(func(p Pair) (Pair, error) {
		if p.Key == two {
			p.Key = one // Two original keys map to one; retain the first.
		}
		if p.Key == three {
			p.Value = one
		}
		return p, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := function(t, Pair{one, one}, Pair{three, one})
	if equal, err := Equal(changed, want); err != nil || !equal {
		t.Fatalf("mapped function: got %v, want %v; %v", changed, want, err)
	}
	if changed.IsTuple() || original.String() != "<<1, 2, 3>>" {
		t.Fatalf("presentation or source changed: original %v, mapped %v", original, changed)
	}
}

func TestMapFunctionPairsError(t *testing.T) {
	one, two := number(t, 1), number(t, 2)
	original := function(t, Pair{one, one}, Pair{two, two})
	failure := errors.New("mapping failed")
	calls := 0
	_, err := original.MapFunctionPairs(func(p Pair) (Pair, error) {
		calls++
		if p.Key == two {
			return Pair{}, failure
		}
		return Pair{p.Key, two}, nil
	})
	if !errors.Is(err, failure) || calls != 2 {
		t.Fatalf("callback failure: %v after %d calls", err, calls)
	}
	if got, err := original.Lookup(one); err != nil || got != one {
		t.Fatalf("original was modified: %v, %v", got, err)
	}
	if _, err := one.MapFunctionPairs(func(p Pair) (Pair, error) { return p, nil }); err == nil {
		t.Fatal("mapping a non-function should fail")
	}
	lazyCalls := 0
	lazy, err := LazyFunction(Interval(1, 2), func(_, _ Value) (Value, error) {
		lazyCalls++
		return one, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lazy.MapFunctionPairs(func(p Pair) (Pair, error) { return p, nil }); err == nil || lazyCalls != 0 {
		t.Fatalf("lazy function should be rejected without evaluation: err=%v calls=%d", err, lazyCalls)
	}
}
