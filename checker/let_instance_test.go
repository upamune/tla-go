package checker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestUpstreamLetInstanceMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-let-instance-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	window, err := os.ReadFile("testdata/let-instances/Window.tla")
	if err != nil {
		t.Fatal(err)
	}
	loader := func(_ context.Context, name string) (string, error) {
		if name != "Window" {
			return "", fmt.Errorf("unknown module %s", name)
		}
		return string(window), nil
	}
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var tc struct {
			Name, Source, Config, Status string
			Generated, Distinct, Depth   int
			Trace                        []int32
		}
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			got, err := CheckWithLoader(context.Background(), tc.Source, tc.Config, loader)
			if err != nil || string(got.Status) != tc.Status || got.GeneratedStates != tc.Generated || got.DistinctStates != tc.Distinct || got.Depth != tc.Depth {
				t.Fatalf("got %+v, %v; want %s %d %d %d", got, err, tc.Status, tc.Generated, tc.Distinct, tc.Depth)
			}
			var trace []int32
			if got.Violation != nil {
				for _, step := range got.Violation.Trace {
					x, _ := step.State["x"].Int()
					trace = append(trace, x)
				}
			}
			if !reflect.DeepEqual(trace, tc.Trace) {
				t.Fatalf("trace %v; want %v", trace, tc.Trace)
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

func TestLetInstanceSourceStringOrder(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS TLC
Foo == LET I == INSTANCE Ext IN I!A
ASSUME PrintT({"aaq", "zzq"})
====`
	loader := func(_ context.Context, _ string) (string, error) {
		return "---- MODULE Ext ----\nA == \"zzq\"\nB == \"aaq\"\n====", nil
	}
	var printed []string
	got, err := CheckWithOptions(context.Background(), source, "", Options{Loader: loader, OnPrint: func(s string) { printed = append(printed, s) }})
	if err != nil || got.Status != Success || !reflect.DeepEqual(printed, []string{`{"zzq", "aaq"}`}) {
		t.Fatalf("got %+v, %v, %q", got, err, printed)
	}
}

func TestLetInstanceRejectsStatefulConstantSubstitution(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Foo == LET k == x
           j == k + 1
           I == INSTANCE Stateful WITH c <- j, v <- x
       IN I!Use
Init == x = 0
Next == x' = x
====`
	loader := func(_ context.Context, _ string) (string, error) {
		return "---- MODULE Stateful ----\nCONSTANT c\nVARIABLE v\nUse == v' = c\n====", nil
	}
	_, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next", loader)
	if err == nil || !strings.Contains(err.Error(), "constant-level substitution") {
		t.Fatalf("got %v; want stateful constant substitution error", err)
	}
}

func TestLetInstanceLambdaUsesLocalDefinition(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals
Foo == LET B == 3
           I == INSTANCE Inner WITH Op <- LAMBDA a : a + B
       IN I!Use
ASSUME Foo = 5
====`
	loader := func(_ context.Context, _ string) (string, error) {
		return "---- MODULE Inner ----\nEXTENDS Naturals\nCONSTANT Op(_)\nUse == Op(2)\n====", nil
	}
	got, err := CheckWithLoader(context.Background(), source, "", loader)
	if err != nil || got.Status != Success {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestUpstreamLetInstances(t *testing.T) {
	for _, tc := range []struct {
		model, config                     string
		status                            Status
		generated, distinct, depth, trace int
	}{
		{"LetInstances", "LetInstances", Success, 193, 96, 19, 0},
		{"LetInstances", "LetInstancesViolation", InvariantViolation, 14, 11, 5, 5},
		{"LetParameters", "LetParameters", InvariantViolation, 4, 4, 4, 4},
		{"Symbols", "Symbols", Success, 0, 0, 1, 0},
	} {
		t.Run(tc.config, func(t *testing.T) {
			dir := filepath.Join("testdata", "let-instances")
			source, err := os.ReadFile(filepath.Join(dir, tc.model+".tla"))
			if err != nil {
				t.Fatal(err)
			}
			config, err := os.ReadFile(filepath.Join(dir, tc.config+".cfg"))
			if err != nil {
				t.Fatal(err)
			}
			loader := func(_ context.Context, name string) (string, error) {
				data, err := os.ReadFile(filepath.Join(dir, name+".tla"))
				return string(data), err
			}
			got, err := CheckWithLoader(context.Background(), string(source), string(config), loader)
			if err != nil || got.Status != tc.status || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct || got.Depth != tc.depth {
				t.Fatalf("got %+v, %v; want %s %d %d %d", got, err, tc.status, tc.generated, tc.distinct, tc.depth)
			}
			if tc.trace > 0 && (got.Violation == nil || len(got.Violation.Trace) != tc.trace) {
				t.Fatalf("got violation %+v; want %d trace states", got.Violation, tc.trace)
			}
		})
	}
}

func TestLetActionScopes(t *testing.T) {
	for _, tc := range []struct {
		source                     string
		generated, distinct, depth int
	}{
		{`---- MODULE M ----
EXTENDS Naturals
VARIABLES x, y
Init == /\ x = 0 /\ y = 0
Next == /\ x = 0 /\ (LET F == 3 IN x' = F) /\ (LET F == 7 IN y' = F)
Inv == x # 3 \/ y = 7
====`, 2, 2, 2},
		{`---- MODULE M ----
EXTENDS Naturals
VARIABLES x, y
Init == /\ x = 0 /\ y = 0
Next == /\ x = 0 /\ \E v \in {1,2} : /\ (LET F == v IN x' = F) /\ y' = v
Inv == x = y
====`, 3, 3, 2},
	} {
		got, err := Check(tc.source, "INIT Init NEXT Next INVARIANT Inv CHECK_DEADLOCK FALSE")
		if err != nil || got.Status != Success || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct || got.Depth != tc.depth {
			t.Fatalf("got %+v, %v", got, err)
		}
	}
}
