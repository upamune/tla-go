package checker

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/upamune/tla-go/value"
)

func TestUpstreamRandomElementXandY(t *testing.T) {
	source, err := os.ReadFile("testdata/randomization/RandomElementXandY.tla")
	if err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile("testdata/randomization/RandomElementXandY.cfg")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		seed                int64
		status              Status
		generated, distinct int
		trace               [][2]int32
	}{
		{8006642976694192746, InvariantViolation, 3, 3, [][2]int32{{0, 0}, {1, 1}, {0, 1}}},
		{2, Success, 3, 2, nil},
	} {
		got, err := CheckWithOptions(context.Background(), string(source), string(config), Options{Seed: tc.seed})
		if err != nil || got.Status != tc.status || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct {
			t.Fatalf("seed %d: got %+v, %v", tc.seed, got, err)
		}
		var trace [][2]int32
		if got.Violation != nil {
			for _, step := range got.Violation.Trace {
				x, _ := step.State["x"].Int()
				y, _ := step.State["y"].Int()
				trace = append(trace, [2]int32{x, y})
			}
		}
		if !reflect.DeepEqual(trace, tc.trace) {
			t.Fatalf("seed %d: got trace %v; want %v", tc.seed, trace, tc.trace)
		}
	}
}

func TestUpstreamRandomElementTraces(t *testing.T) {
	for _, tc := range []struct {
		name, variable      string
		seed                int64
		generated, distinct int
		trace               []int32
	}{
		{"RandomElement", "x", 8006803340504660123, 932, 855, []int32{843, 920, 483, 173, 590, 104, 785, 463, 443, 151, 767}},
		{"RandomOperator", "alpha", 1, 7, 7, []int32{0, 80, 94, 100, 32, 12, 34}},
		{"RandomLet", "alpha", 1, 7, 7, []int32{0, 100, 77, 50, 66, 68, 12}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/randomization/" + tc.name + ".tla")
			if err != nil {
				t.Fatal(err)
			}
			config, err := os.ReadFile("testdata/randomization/" + tc.name + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			got, err := CheckWithOptions(context.Background(), string(source), string(config), Options{Seed: tc.seed})
			if err != nil || got.Status != InvariantViolation || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct {
				t.Fatalf("got %+v, %v", got, err)
			}
			var trace []int32
			for _, step := range got.Violation.Trace {
				v, _ := step.State[tc.variable].Int()
				trace = append(trace, v)
			}
			if !reflect.DeepEqual(trace, tc.trace) {
				t.Fatalf("trace %v; want %v", trace, tc.trace)
			}
		})
	}
}

func TestUpstreamRandomModelValue(t *testing.T) {
	source, err := os.ReadFile("testdata/randomization/RandomModelValue.tla")
	if err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile("testdata/randomization/RandomModelValue.cfg")
	if err != nil {
		t.Fatal(err)
	}
	got, err := CheckWithOptions(context.Background(), string(source), string(config), Options{Seed: 1})
	if err != nil || got.Status != InvariantViolation {
		t.Fatalf("got %+v, %v", got, err)
	}
	var trace []string
	for _, step := range got.Violation.Trace {
		owner, _ := step.State["owner"].Text()
		trace = append(trace, owner)
	}
	want := []string{"p1", "p3", "p2", "p1", "p1", "p2", "p3"}
	if !reflect.DeepEqual(trace, want) {
		t.Fatalf("trace %v; want %v", trace, want)
	}
}

func TestUpstreamRandomElementCompositeValues(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals, TLC
ASSUME PrintT(RandomElement(1..100))
ASSUME PrintT(RandomElement({"zz", "aa", "mm"}))
ASSUME PrintT(RandomElement([1..3 -> 0..9]))
ASSUME PrintT(RandomElement([b: 1..5, a: 1..5]))
ASSUME PrintT(RandomElement((1..5) \X (6..9)))
====`
	var printed []string
	got, err := CheckWithOptions(context.Background(), source, "", Options{Seed: 1, OnPrint: func(s string) { printed = append(printed, s) }})
	want := []string{"74", `"aa"`, "<<2, 3, 9>>", "[b |-> 1, a |-> 5]", "<<5, 9>>"}
	if err != nil || got.Status != Success || !reflect.DeepEqual(printed, want) {
		t.Fatalf("got %+v, %v, %q; want %q", got, err, printed, want)
	}
}

func TestUpstreamRandomSubsetOperators(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals, TLC, Randomization
ASSUME PrintT(RandomSubset(3, 1..20))
ASSUME PrintT(RandomSetOfSubsets(4, 2, 1..5))
ASSUME PrintT(TestRandomSetOfSubsets(4, 2, 1..5))
ASSUME PrintT(RandomSubset(4, [1..3 -> {0,1}]))
ASSUME PrintT(RandomSubset(4, [b: 1..3, a: 1..2]))
====`
	var printed []string
	got, err := CheckWithOptions(context.Background(), source, "", Options{Seed: 1, ModernRandomSubsets: true, OnPrint: func(s string) { printed = append(printed, s) }})
	want := []string{"{7, 13, 15}", "{{1}, {1, 2}, {2, 5}}", "<<4, 4, 4, 4, 4>>", "{<<0, 0, 0>>, <<0, 0, 1>>, <<0, 1, 0>>, <<1, 0, 1>>}", "{[b |-> 1, a |-> 1], [b |-> 1, a |-> 2], [b |-> 3, a |-> 1], [b |-> 3, a |-> 2]}"}
	if err != nil || got.Status != Success || !reflect.DeepEqual(printed, want) {
		t.Fatalf("got %+v, %v, %q; want %q", got, err, printed, want)
	}
}

func TestUpstreamRandomSubsetFixtures(t *testing.T) {
	for _, tc := range []struct {
		name                string
		seed                int64
		status              Status
		generated, distinct int
	}{
		{"RandomSubsetNext", 15041980, InvariantViolation, 67291, 7729},
		{"RandomSubsetNextTuples", 1, InvariantViolation, 5461, 5461},
		{"RandomSubsetSetOfFcns", 1, Success, 2000, 1000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/randomization/" + tc.name + ".tla")
			if err != nil {
				t.Fatal(err)
			}
			config, err := os.ReadFile("testdata/randomization/" + tc.name + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			got, err := CheckWithOptions(context.Background(), string(source), string(config), Options{Seed: tc.seed, ModernRandomSubsets: true})
			if err != nil || got.Status != tc.status || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct {
				t.Fatalf("got %+v, %v; want %s %d %d", got, err, tc.status, tc.generated, tc.distinct)
			}
			if tc.name == "RandomSubsetNext" {
				var trace []int32
				for _, step := range got.Violation.Trace {
					x, _ := step.State["x"].Int()
					trace = append(trace, x)
				}
				want := []int32{23, 26, 18, 29, 189, 19, 92, 250, 41, 52, 78}
				if !reflect.DeepEqual(trace, want) {
					t.Fatalf("trace %v; want %v", trace, want)
				}
			}
		})
	}
}

func TestUpstreamRandomSubsetLargeProducts(t *testing.T) {
	source, err := os.ReadFile("testdata/randomization/RandomSubset.tla")
	if err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile("testdata/randomization/RandomSubset.cfg")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		seed int64
		x, y int32
		p, q []int32
	}{
		{15041980, 129202, 100000008, []int32{113, 155, 35}, []int32{2708, 3048, 3471}},
		{918347981374, 604, 100000000, []int32{15, 152, 5}, []int32{86, 170, 1612}},
	} {
		got, err := CheckWithOptions(context.Background(), string(source), string(config), Options{Seed: tc.seed, ModernRandomSubsets: true})
		if err != nil || got.Status != InvariantViolation || got.GeneratedStates != 8009 || got.DistinctStates != 8009 {
			t.Fatalf("seed %d: got %+v, %v", tc.seed, got, err)
		}
		initial := got.Violation.Trace[0].State
		x, _ := initial["x"].Int()
		y, _ := initial["y"].Int()
		if x != tc.x || y != tc.y {
			t.Fatalf("seed %d: got x=%d y=%d; want %d %d", tc.seed, x, y, tc.x, tc.y)
		}
		checkRandomTuple(t, initial["p"], tc.p)
		checkRandomTuple(t, initial["q"], tc.q)
	}
}

func checkRandomTuple(t *testing.T, v value.Value, want []int32) {
	t.Helper()
	items, err := v.TupleElements()
	if err != nil {
		t.Fatal(err)
	}
	got := make([]int32, len(items))
	for i, item := range items {
		got[i], _ = item.Int()
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tuple %v; want %v", got, want)
	}
}

func TestUpstreamRandomConstant(t *testing.T) {
	source, err := os.ReadFile("testdata/randomization/Github648.tla")
	if err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile("testdata/randomization/Github648.cfg")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		seed                int64
		generated, distinct int
	}{{1, 1332, 36}, {-2252017213236477395, 4160, 64}, {8006642976694192746, 90, 9}} {
		got, err := CheckWithOptions(context.Background(), string(source), string(config), Options{Seed: tc.seed})
		if err != nil || got.Status != Success || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct {
			t.Fatalf("seed %d: got %+v, %v; want success %d %d", tc.seed, got, err, tc.generated, tc.distinct)
		}
	}
}
