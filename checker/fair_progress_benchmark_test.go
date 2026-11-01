package checker

import (
	"context"
	"testing"
)

func BenchmarkEffectfulFairnessProgress(b *testing.B) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x=0
Next == x'=(x+1)%1000 /\ PrintT("effect")
Spec == Init /\ [][Next]_x /\ WF_x(Next)
P == []<>(x>=0)
====`
	opts := Options{OnPrint: func(string) {}, OnSearch: func(SearchEvent) {}}
	b.ReportAllocs()
	for b.Loop() {
		result, err := CheckWithOptions(context.Background(), source, "SPECIFICATION Spec PROPERTY P", opts)
		if err != nil || result.Status != Success {
			b.Fatalf("result %+v: %v", result, err)
		}
	}
}
