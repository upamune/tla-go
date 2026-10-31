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

func TestJavaRandomAdvanceDoubles(t *testing.T) {
	for _, seed := range []int64{0, 1, -42, 1<<62 - 1} {
		for _, count := range []uint64{0, 1, 2, 7, 1000, 123456} {
			want := NewJavaRandom(seed)
			for range count {
				want.Float64()
			}
			got := NewJavaRandom(seed)
			got.AdvanceDoubles(count)
			if a, b := got.Long(), want.Long(); a != b {
				t.Errorf("seed %d, advance %d: got %d, want %d", seed, count, a, b)
			}
		}
	}
}

func TestJavaRandomAdvanceDoublesPeriod(t *testing.T) {
	baseline := NewJavaRandom(42)
	baseline.AdvanceDoubles(12345)
	advanced := NewJavaRandom(42)
	advanced.AdvanceDoubles((uint64(1) << 47) + 12345)
	if got, want := advanced.Long(), baseline.Long(); got != want {
		t.Fatalf("advanced generator produced %d; want %d", got, want)
	}
	large := NewJavaRandom(-1)
	large.AdvanceDoubles(1<<63 - 1)
	periodic := NewJavaRandom(-1)
	periodic.AdvanceDoubles(1<<47 - 1)
	if got, want := large.Long(), periodic.Long(); got != want {
		t.Fatalf("large advance produced %d; want %d", got, want)
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

// Java TLC 1.7.4 seeds each simulation worker with the master RNG's nextLong.
// These pairs are nextDouble()*7 and nextPrime from that worker, respectively.
func TestJavaRandomSimulationWorkerSequence(t *testing.T) {
	master := NewJavaRandom(1)
	worker := NewJavaRandom(master.Long())
	want := [][2]int64{
		{0, 1292309},
		{0, 1290593},
		{1, 1282163},
		{4, 1277093},
		{3, 1278047},
		{0, 1285429},
		{3, 1283237},
		{0, 1293367},
		{1, 1279787},
		{6, 1288951},
		{1, 1282343},
		{0, 1287589},
	}
	for i, expected := range want {
		got := [2]int64{int64(worker.Float64() * 7), worker.Prime()}
		if got != expected {
			t.Fatalf("draw %d: got %v; want %v", i, got, expected)
		}
	}
}
