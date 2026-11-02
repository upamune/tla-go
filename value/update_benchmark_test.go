package value

import (
	"fmt"
	"testing"
)

func BenchmarkFunctionNoopUpdate(b *testing.B) {
	for _, tc := range []struct {
		size     int
		deferred bool
	}{{2, false}, {32, false}, {256, false}, {256, true}} {
		b.Run(fmt.Sprintf("size-%d-deferred-%t", tc.size, tc.deferred), func(b *testing.B) {
			pairs := make([]Pair, tc.size)
			for i := range pairs {
				pairs[i] = Pair{Key: Value{kind: IntegerKind, number: int32(i + 1)}, Value: Bool(false)}
			}
			if tc.deferred {
				var err error
				pairs[len(pairs)-1].Value, err = LazyFunction(Interval(1, 1), func(Value, Value) (Value, error) { return Bool(false), nil })
				if err != nil {
					b.Fatal(err)
				}
			}
			function, err := FunctionWithDomain(pairs, Interval(1, int32(tc.size)))
			if err != nil {
				b.Fatal(err)
			}
			key := Value{kind: IntegerKind, number: 1}
			for b.Loop() {
				_, found, err := function.UpdateExisting(key, func(v Value) (Value, error) { return v, nil })
				if err != nil || !found {
					b.Fatalf("update: found=%v error=%v", found, err)
				}
			}
		})
	}
}

func BenchmarkFunctionNoopUpdateDuplicateKeys(b *testing.B) {
	key := Value{kind: IntegerKind, number: 1}
	pairs := make([]Pair, 100000)
	for i := range pairs {
		pairs[i] = Pair{Key: key, Value: Bool(false)}
	}
	function, err := Function(pairs)
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		_, found, err := function.UpdateExisting(key, func(v Value) (Value, error) { return v, nil })
		if err != nil || !found {
			b.Fatalf("update: found=%v error=%v", found, err)
		}
	}
}

func BenchmarkFunctionNoopUpdateSpareCapacity(b *testing.B) {
	for _, spare := range []int{0, 64, 65} {
		b.Run(fmt.Sprintf("spare-%d", spare), func(b *testing.B) {
			pairs := make([]Pair, 65, 65+spare)
			key := Value{kind: IntegerKind, number: 1}
			for i := range pairs {
				pairs[i] = Pair{Key: Value{kind: IntegerKind, number: int32(i + 1)}, Value: Bool(false)}
			}
			function := Value{kind: FunctionKind, data: &payload{pairs: pairs}}
			for b.Loop() {
				_, found, err := function.UpdateExisting(key, func(v Value) (Value, error) { return v, nil })
				if err != nil || !found {
					b.Fatalf("update: found=%v error=%v", found, err)
				}
			}
		})
	}
}
