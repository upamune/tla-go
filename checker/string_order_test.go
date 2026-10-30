package checker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamImportedStringOrder(t *testing.T) {
	modules := map[string]string{
		"Base": "---- MODULE Base ----\nA == \"zzq\"\nB == \"aaq\"\n====",
		"Mid":  "---- MODULE Mid ----\nEXTENDS Base\nC == \"mmq\"\n====",
	}
	for name, imports := range map[string]string{
		"extend": "EXTENDS TLC, Base", "instance": "EXTENDS TLC\nI == INSTANCE Base", "chain": "EXTENDS TLC, Mid",
	} {
		t.Run(name, func(t *testing.T) {
			source := "---- MODULE M ----\n" + imports + "\nASSUME PrintT({\"aaq\", \"zzq\", \"mmq\"})\n===="
			var printed []string
			options := Options{Loader: func(_ context.Context, name string) (string, error) {
				if source, ok := modules[name]; ok {
					return source, nil
				}
				return "", fmt.Errorf("unknown module %s", name)
			}, OnPrint: func(s string) { printed = append(printed, s) }}
			got, err := CheckWithOptions(context.Background(), source, "", options)
			want := []string{`{"zzq", "aaq", "mmq"}`}
			if err != nil || got.Status != Success || !reflect.DeepEqual(printed, want) {
				t.Fatalf("got %+v, %v, %q; want %q", got, err, printed, want)
			}
		})
	}
}

func TestUpstreamNestedModuleStringOrder(t *testing.T) {
	for _, instance := range []string{"", "J == INSTANCE Inner\n"} {
		source := "---- MODULE M ----\nEXTENDS TLC\n---- MODULE Inner ----\nI == INSTANCE Ext\nK == I!A\n====\n" + instance + "ASSUME PrintT({\"aaq\", \"zzq\"})\n===="
		var printed []string
		options := Options{Loader: func(_ context.Context, name string) (string, error) {
			if name == "Ext" {
				return "---- MODULE Ext ----\nA == \"zzq\"\nB == \"aaq\"\n====", nil
			}
			return "", fmt.Errorf("unknown module %s", name)
		}, OnPrint: func(s string) { printed = append(printed, s) }}
		got, err := CheckWithOptions(context.Background(), source, "", options)
		if err != nil || got.Status != Success || !reflect.DeepEqual(printed, []string{`{"zzq", "aaq"}`}) {
			t.Fatalf("instance %q: got %+v, %v, %q", instance, got, err, printed)
		}
	}
}

func TestUpstreamPreinternedStringOrder(t *testing.T) {
	for _, tc := range []struct{ items, want string }{
		{`"TRUE", "FALSE"`, `{"FALSE", "TRUE"}`},
		{`"Nat", "BOOLEAN", "STRING"`, `{"STRING", "BOOLEAN", "Nat"}`},
		{`"pause", "exit"`, `{"pause", "exit"}`},
		{`"N_ActionExpr", "N_ActDecl"`, `{"N_ActDecl", "N_ActionExpr"}`},
	} {
		source := "---- MODULE M ----\nEXTENDS TLC\nASSUME PrintT({" + tc.items + "})\n===="
		var printed []string
		got, err := CheckWithOptions(context.Background(), source, "", Options{OnPrint: func(s string) { printed = append(printed, s) }})
		if err != nil || got.Status != Success || !reflect.DeepEqual(printed, []string{tc.want}) {
			t.Fatalf("%s: got %+v, %v, %q; want %s", tc.items, got, err, printed, tc.want)
		}
	}
}

func TestUpstreamUnreferencedTrailingModuleStringOrder(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS TLC\nASSUME PrintT({\"aaq\", \"zzq\"})\n====\n---- MODULE Other ----\nzzq == 0\n===="
	var printed []string
	got, err := CheckWithOptions(context.Background(), source, "", Options{OnPrint: func(s string) { printed = append(printed, s) }})
	if err != nil || got.Status != Success || !reflect.DeepEqual(printed, []string{`{"aaq", "zzq"}`}) {
		t.Fatalf("got %+v, %v, %q", got, err, printed)
	}
}

func TestUpstreamStandardModuleStringOrder(t *testing.T) {
	for _, tc := range []struct{ module, items, want string }{
		{"Naturals", `"unusedq", "Nat"`, `{"Nat", "unusedq"}`},
		{"Integers", `"unusedq", "Int", "Nat"`, `{"Int", "Nat", "unusedq"}`},
		{"Sequences", `"unusedq", "Append", "Len", "Nat"`, `{"Nat", "Len", "Append", "unusedq"}`},
		{"FiniteSets", `"unusedq", "Cardinality", "Append", "Nat"`, `{"Cardinality", "Nat", "Append", "unusedq"}`},
		{"TLC", `"unusedq", "Print", "Cardinality", "Append", "Nat"`, `{"Print", "Nat", "Append", "Cardinality", "unusedq"}`},
		{"TLCExt", `"unusedq", "AssertEq", "Print", "Int", "Nat"`, `{"AssertEq", "Int", "Print", "Nat", "unusedq"}`},
		{"Bags", `"unusedq", "BagUnion", "Nat"`, `{"Nat", "BagUnion", "unusedq"}`},
	} {
		t.Run(tc.module, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS " + tc.module + ", TLC\nASSUME PrintT({" + tc.items + "})\n===="
			var printed []string
			got, err := CheckWithOptions(context.Background(), source, "", Options{OnPrint: func(s string) { printed = append(printed, s) }})
			if err != nil || got.Status != Success || !reflect.DeepEqual(printed, []string{tc.want}) {
				t.Fatalf("got %+v, %v, %q; want %s", got, err, printed, tc.want)
			}
		})
	}
}

func TestUpstreamStandardStringOrderMatrix(t *testing.T) {
	testUpstreamModuleStringOrderMatrix(t, "testdata/upstream-standard-string-order-matrix.jsonl", 196)
}

func TestUpstreamNestedStringOrderMatrix(t *testing.T) {
	testUpstreamModuleStringOrderMatrix(t, "testdata/upstream-nested-string-order-matrix.jsonl", 72)
}

func testUpstreamModuleStringOrderMatrix(t *testing.T, path string, expected int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var tc struct {
			Name, Source, Config, Status string
			Modules                      map[string]string
			Printed                      []string
		}
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			var printed []string
			options := Options{Loader: func(_ context.Context, name string) (string, error) {
				if source, ok := tc.Modules[name]; ok {
					return source, nil
				}
				return "", fmt.Errorf("unknown module %s", name)
			}, OnPrint: func(s string) { printed = append(printed, s) }}
			got, err := CheckWithOptions(context.Background(), tc.Source, tc.Config, options)
			if err != nil || string(got.Status) != tc.Status || !reflect.DeepEqual(printed, tc.Printed) {
				t.Fatalf("got %+v, %v, %q; want %s %q", got, err, printed, tc.Status, tc.Printed)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != expected {
		t.Fatalf("got %d cases, want %d", count, expected)
	}
}

func TestUpstreamStringOrderFixtures(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		status                     Status
		generated, distinct, depth int
	}{
		{"StringOrder", InvariantViolation, 2, 2, 2},
		{"StringSets", Success, 76, 31, 5},
		{"RuntimeStrings", Success, 115, 97, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/strings/" + tc.name + ".tla")
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := os.ReadFile("testdata/strings/" + tc.name + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			var printed []string
			got, err := CheckWithOptions(context.Background(), string(source), string(cfg), Options{OnPrint: func(s string) { printed = append(printed, s) }})
			if err != nil || got.Status != tc.status || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct || got.Depth != tc.depth {
				t.Fatalf("got %+v, %v", got, err)
			}
			if tc.name != "StringOrder" {
				return
			}
			want := []string{`"zzq"`, `{"zzq", "aaq", "mmq"}`, `[zz |-> 1, aa |-> 2]`, `{[zz |-> 1], [aa |-> 1]}`, `{p2, m, b}`, `p2`, `{"7", "3"}`, `{"zzb", "zza"}`}
			if !reflect.DeepEqual(printed, want) {
				t.Fatalf("printed %q, want %q", printed, want)
			}
			if got.Violation == nil || len(got.Violation.Trace) != 2 || got.Violation.Trace[0].State["pick"].String() != `"zzq"` || got.Violation.Trace[1].State["pick"].String() != `"aaq"` {
				t.Fatalf("violation %+v", got.Violation)
			}
		})
	}
}

func TestUpstreamQuotedLiteralRank(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS TLC
VARIABLE x
A == "b"
B == "a"
ASSUME PrintT({ToString("a"), ToString("b")})
Init == x = 0
Next == UNCHANGED x
====`
	var printed []string
	got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next", Options{OnPrint: func(s string) { printed = append(printed, s) }})
	want := []string{`{"\"b\"", "\"a\""}`}
	if err != nil || got.Status != Success || !reflect.DeepEqual(printed, want) {
		t.Fatalf("got %+v, %v, %q; want %q", got, err, printed, want)
	}
}

func TestStringOrderDoesNotLeakBetweenChecks(t *testing.T) {
	sources := []string{
		`---- MODULE M ----
Picked == CHOOSE s \in {"zzq", "aaq"} : TRUE
ASSUME Picked = "zzq"
====`,
		`---- MODULE M ----
ASSUME "aaq" # "zzq"
ASSUME (CHOOSE s \in {"zzq", "aaq"} : TRUE) = "aaq"
====`,
	}
	for _, source := range sources {
		got, err := Check(source, "")
		if err != nil || got.Status != Success {
			t.Fatalf("got %+v, %v", got, err)
		}
	}
}

func TestUpstreamStringOrderMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-string-order-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var tc struct {
			Name, Source, Config, Status, Error string
			Printed                             []string
			Generated, Distinct, Depth          int
		}
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			var printed []string
			got, err := CheckWithOptions(context.Background(), tc.Source, tc.Config, Options{OnPrint: func(s string) { printed = append(printed, s) }})
			if tc.Error != "" {
				if err == nil || err.Error() != tc.Error {
					t.Fatalf("got %+v, %v; want %q", got, err, tc.Error)
				}
				return
			}
			if err != nil || string(got.Status) != tc.Status || got.GeneratedStates != tc.Generated || got.DistinctStates != tc.Distinct || got.Depth != tc.Depth || !reflect.DeepEqual(printed, tc.Printed) {
				t.Fatalf("got %+v, %v, %q; want %s %d %d %d %q", got, err, printed, tc.Status, tc.Generated, tc.Distinct, tc.Depth, tc.Printed)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 96 {
		t.Fatalf("got %d cases, want 96", count)
	}
}

func TestUpstreamOverriddenSetFixtures(t *testing.T) {
	for _, name := range []string{"StringSet", "OverriddenSets"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/strings/" + name + ".tla")
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := os.ReadFile("testdata/strings/" + name + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			got, err := Check(string(source), string(cfg))
			if err != nil || got.Status != Success {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestUpstreamStringSetComparisonError(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nASSUME STRING # {}\nInit == x = 0\nNext == x' = x\n===="
	got, err := Check(source, "INIT Init NEXT Next")
	want := "Attempted to compare overridden value STRING with non-overridden value:\n{}"
	if err != nil || got.Status != AssumptionViolation || got.Violation == nil || got.Violation.Message != want {
		t.Fatalf("got %+v, %v; want %q", got, err, want)
	}
}

func TestHigherOrderSetOperationsPreserveStringOrder(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS TLC
A == "zz"
B == "aa"
First(Op(_,_)) == CHOOSE s \in Op({"aa"},{"zz"}) : TRUE
ASSUME First(\cup) = "zz"
====`
	got, err := Check(source, "")
	if err != nil || got.Status != Success {
		t.Fatalf("got %+v, %v", got, err)
	}
}
