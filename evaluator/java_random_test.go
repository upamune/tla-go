package evaluator

import (
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func TestJavaRandomMatchesUpstream(t *testing.T) {
	for _, tc := range []struct {
		seed    int64
		doubles []float64
		ints    []int32
		longs   []int64
	}{
		{0, []float64{0.730967787376657, 0.24053641567148587, 0.6374174253501083}, []int32{360, 948, 29}, []int64{-4962768465676381896, 4437113781045784766, -6688467811848818630}},
		{1, []float64{0.7308781907032909, 0.41008081149220166, 0.20771484130971707}, []int32{985, 588, 847}, []int64{-4964420948893066024, 7564655870752979346, 3831662765844904176}},
		{-42, []float64{0.2726154686397476, 0.06094973837072859, 0.2798902062508173}, []int32{265, 546, 552}, []int64{5028867820122015268, 1124324099246659001, 5163062962508109336}},
	} {
		r := NewJavaRandom(tc.seed)
		for _, want := range tc.doubles {
			if got := r.Float64(); got != want {
				t.Errorf("seed %d: got %v, want %v", tc.seed, got, want)
			}
		}
		r.SetSeed(tc.seed)
		for _, want := range tc.ints {
			if got := r.Int(1000); got != want {
				t.Errorf("seed %d: got %v, want %v", tc.seed, got, want)
			}
		}
		r.SetSeed(tc.seed)
		for _, want := range tc.longs {
			if got := r.Long(); got != want {
				t.Errorf("seed %d: got %v, want %v", tc.seed, got, want)
			}
		}
	}
}

func TestStandaloneRandomSeed(t *testing.T) {
	module, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Naturals, TLC\nDraw == RandomElement(1..100)\n====")
	if err != nil {
		t.Fatal(err)
	}
	program := New(module)
	program.SetRandomSeed(1)
	for _, want := range []int32{74, 42, 21} {
		v, err := program.Evaluate(module.Definitions[0].Body, Context{})
		got, ok := v.Int()
		if err != nil || !ok || got != want {
			t.Fatalf("got %d, %v; want %d", got, err, want)
		}
	}
}
