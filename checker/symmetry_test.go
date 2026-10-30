package checker

import (
	"bufio"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestUpstreamSymmetryFixtures(t *testing.T) {
	for _, tc := range []struct {
		dir, name                  string
		status                     Status
		generated, distinct, depth int
	}{
		{"symmetry", "Symmetry", Success, 42, 10, 7},
		{"symmetry", "SymmetricMessages", Success, 641, 80, 9},
		{"view", "ViewSymmetry", Success, 7, 4, 4},
		{"view", "ViewSymmetryOrder", Success, 139, 14, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prefix := "testdata/" + tc.dir + "/" + tc.name
			source, err := os.ReadFile(prefix + ".tla")
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := os.ReadFile(prefix + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			got, err := Check(string(source), string(cfg))
			if err != nil || got.Status != tc.status || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct || got.Depth != tc.depth {
				t.Fatalf("got %+v, %v; want %s %d %d %d", got, err, tc.status, tc.generated, tc.distinct, tc.depth)
			}
		})
	}
	source, err := os.ReadFile("testdata/symmetry/SymmetricLiveness.tla")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile("testdata/symmetry/SymmetricLiveness.cfg")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Check(string(source), string(cfg))
	if err != nil || got.Status != LivenessViolation || got.Violation == nil || got.Violation.Name != "Live" || len(got.Violation.Trace) != 1 || got.Violation.Trace[0].State["x"].String() != "m1" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func BenchmarkSymmetricMessages(b *testing.B) {
	source, err := os.ReadFile("testdata/symmetry/SymmetricMessages.tla")
	if err != nil {
		b.Fatal(err)
	}
	cfg, err := os.ReadFile("testdata/symmetry/SymmetricMessages.cfg")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		got, err := Check(string(source), string(cfg))
		if err != nil || got.Status != Success || got.DistinctStates != 80 {
			b.Fatalf("got %+v, %v", got, err)
		}
	}
}

func TestUpstreamSymmetryMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-symmetry-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var tc struct {
			Name, Source, Config, Status, Error string
			Order                               []string
			Generated, Distinct, Depth          int
		}
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			module, err := parser.ParseModule(tc.Source)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse(tc.Config)
			if err != nil {
				t.Fatal(err)
			}
			model, err := prepareWithOutput(module, cfg, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(model.symmetryOrder, tc.Order) {
				t.Fatalf("variable order: got %q, want %q", model.symmetryOrder, tc.Order)
			}
			got, err := Check(tc.Source, tc.Config)
			if tc.Error != "" {
				if err == nil || err.Error() != tc.Error {
					t.Fatalf("got %+v, %v; want error %q", got, err, tc.Error)
				}
				return
			}
			if err != nil || string(got.Status) != tc.Status || got.GeneratedStates != tc.Generated || got.DistinctStates != tc.Distinct || got.Depth != tc.Depth {
				t.Fatalf("got %+v, %v; want %s %d %d %d", got, err, tc.Status, tc.Generated, tc.Distinct, tc.Depth)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 156 {
		t.Fatalf("got %d cases, want 156", count)
	}
}

func TestSymmetryClosureAndNestedValues(t *testing.T) {
	a, b, c := value.Model("a"), value.Model("b"), value.Model("c")
	cycle := permutation{images: map[string]value.Value{"a": b, "b": c, "c": a}}
	swap := permutation{images: map[string]value.Value{"a": b, "b": a}}
	group := symmetryClosure([]permutation{cycle, swap})
	if len(group) != 5 {
		t.Fatalf("got %d non-identity permutations, want 5", len(group))
	}
	nested, err := value.Function([]value.Pair{{Key: value.String("who"), Value: a}, {Key: b, Value: c}})
	if err != nil {
		t.Fatal(err)
	}
	set, err := value.Set([]value.Value{nested, a})
	if err != nil {
		t.Fatal(err)
	}
	got, err := permuteValue(set, cycle)
	if err != nil {
		t.Fatal(err)
	}
	wantNested, err := value.Function([]value.Pair{{Key: value.String("who"), Value: b}, {Key: c, Value: a}})
	if err != nil {
		t.Fatal(err)
	}
	want, err := value.Set([]value.Value{wantNested, b})
	if err != nil {
		t.Fatal(err)
	}
	equal, err := value.Equal(got, want)
	if err != nil || !equal {
		t.Fatalf("got %s, want %s, err %v", got, want, err)
	}
}

func TestSymmetryRejectsInvalidDefinitions(t *testing.T) {
	for _, tc := range []struct{ definition, config, message string }{
		{"CONSTANT P", "CONSTANT P = {p1,p2} SYMMETRY P", "must specify a set of permutations"},
		{"Bad == {1,2}", "SYMMETRY Bad", "must be functions"},
		{"Bad == {1 :> 2}", "SYMMETRY Bad", "must have model values as domain and range"},
	} {
		source := "---- MODULE M ----\nEXTENDS TLC\nVARIABLE x\n" + tc.definition + "\nInit == x = 0\nNext == x' = x\n===="
		_, err := Check(source, "INIT Init NEXT Next "+tc.config)
		if err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Fatalf("%s: got %v, want %q", tc.definition, err, tc.message)
		}
	}
}
