package checker

import (
	"bufio"
	"context"
	"encoding/json"
	"maps"
	"os"
	"reflect"
	"slices"
	"testing"
)

func TestUpstreamAliasModels(t *testing.T) {
	source, err := os.ReadFile("testdata/alias/Alias.tla")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		status Status
		loop   *int
		delta  []string
	}{
		{"Alias", InvariantViolation, nil, []string{"1", "1", "1", "0"}},
		{"AliasLasso", LivenessViolation, new(0), []string{"1", "1", "1", "-3"}},
		{"AliasStuttering", LivenessViolation, new(0), []string{"0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := os.ReadFile("testdata/alias/" + tc.name + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			got, err := Check(string(source), string(config))
			if err != nil || got.Status != tc.status || got.Violation == nil || !reflect.DeepEqual(got.Violation.LoopStart, tc.loop) || len(got.Violation.Trace) != len(tc.delta) {
				t.Fatalf("got %+v, %v", got, err)
			}
			for i, step := range got.Violation.Trace {
				if step.State["a"].String() != tc.delta[i] || !reflect.DeepEqual(step.StateOrder, []string{"yy", "x", "a", "b", "anim", "te", "lvl", "trc"}) {
					t.Fatalf("step %d: %+v", i, step)
				}
			}
		})
	}
}

func TestUpstreamAliasMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-alias-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	count := 0
	for scanner.Scan() {
		var tc struct {
			Name, Source, Config, Status string
			Printed                      []string
			Trace                        [][][2]string
			Generated, Distinct, Depth   int
			Loop                         *int
		}
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			var lines []string
			got, err := CheckWithOptions(context.Background(), tc.Source, tc.Config, Options{OnPrint: func(s string) { lines = append(lines, s) }})
			if err != nil || string(got.Status) != tc.Status || got.GeneratedStates != tc.Generated || got.DistinctStates != tc.Distinct || got.Depth != tc.Depth || !reflect.DeepEqual(lines, tc.Printed) {
				t.Fatalf("got %+v, %v, %q; want %s %d %d %d %q", got, err, lines, tc.Status, tc.Generated, tc.Distinct, tc.Depth, tc.Printed)
			}
			if got.Violation == nil {
				if len(tc.Trace) != 0 {
					t.Fatal("missing trace")
				}
				return
			}
			if !reflect.DeepEqual(got.Violation.LoopStart, tc.Loop) || len(got.Violation.Trace) != len(tc.Trace) {
				t.Fatalf("trace: %+v, want %v loop %v", got.Violation, tc.Trace, tc.Loop)
			}
			for i, step := range got.Violation.Trace {
				names := step.StateOrder
				if names == nil {
					names = slices.Sorted(maps.Keys(step.State))
				}
				actual := make([][2]string, 0, len(names))
				for _, name := range names {
					actual = append(actual, [2]string{name, step.State[name].String()})
				}
				if !reflect.DeepEqual(actual, tc.Trace[i]) {
					t.Fatalf("step %d: got %q, want %q", i, actual, tc.Trace[i])
				}
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 72 {
		t.Fatalf("got %d cases, want 72", count)
	}
}

func BenchmarkEventuallyLinear(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x < 999 /\\ x' = x + 1\nProp == <>(x = 2000)\n===="
	b.ReportAllocs()
	for b.Loop() {
		result, err := Check(source, "INIT Init NEXT Next PROPERTY Prop CHECK_DEADLOCK FALSE")
		if err != nil || result.Status != LivenessViolation || result.DistinctStates != 1000 {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}
