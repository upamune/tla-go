package checker

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUpstreamViewFixtures(t *testing.T) {
	for _, name := range []string{"View", "Recover"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/view/" + name + ".tla")
			if err != nil {
				t.Fatal(err)
			}
			config, err := os.ReadFile("testdata/view/" + name + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			got, err := Check(string(source), string(config))
			if name == "Recover" {
				if err == nil || !strings.Contains(err.Error(), "Failed to recover the next state from its fingerprint.") {
					t.Fatalf("got %+v, %v", got, err)
				}
				return
			}
			if err != nil || got.Status != Success || got.GeneratedStates != 5 || got.DistinctStates != 4 || got.Depth != 4 {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestUpstreamViewMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-view-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var tc struct {
			Name, Source, Config, Status, Error string
			Generated, Distinct, Depth          int
		}
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
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
	if count != 96 {
		t.Fatalf("got %d cases, want 96", count)
	}
}

func TestViewEvaluationOrder(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x \in {0,1}
Next == x < 2 /\ x' \in {x, x+1}
View == Print(<<"view",x>>,x)
Bound == PrintT(<<"constraint",x>>)
Inv == PrintT(<<"invariant",x>>)
====`
	var printed []string
	got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next VIEW View CONSTRAINT Bound INVARIANT Inv CHECK_DEADLOCK FALSE", Options{OnPrint: func(s string) { printed = append(printed, s) }})
	want := []string{`<<"constraint", 0>>`, `<<"view", 0>>  0`, `<<"invariant", 0>>`, `<<"constraint", 1>>`, `<<"view", 1>>  1`, `<<"invariant", 1>>`, `<<"constraint", 0>>`, `<<"view", 0>>  0`, `<<"constraint", 1>>`, `<<"view", 1>>  1`, `<<"constraint", 1>>`, `<<"view", 1>>  1`, `<<"constraint", 2>>`, `<<"view", 2>>  2`, `<<"invariant", 2>>`}
	if err != nil || got.Status != Success || got.GeneratedStates != 6 || got.DistinctStates != 3 || got.Depth != 2 || !reflect.DeepEqual(printed, want) {
		t.Fatalf("got %+v, %v, %q; want %q", got, err, printed, want)
	}
}

func BenchmarkViewReduction(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES x, y\nInit == x = 0 /\\ y = 0\nNext == x < 999 /\\ x' = x + 1 /\\ y' = (y + 1) % 2\nView == x\n===="
	b.ReportAllocs()
	for b.Loop() {
		result, err := Check(source, "INIT Init NEXT Next VIEW View CHECK_DEADLOCK FALSE")
		if err != nil || result.Status != Success || result.DistinctStates != 1000 {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}

func TestViewRecoveryEvaluationOrder(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x = 0
Next == x < 2 /\ x' = x + 1
View == Print(<<"view",x>>,x)
Inv == x < 2
====`
	var printed []string
	got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next VIEW View INVARIANT Inv", Options{OnPrint: func(s string) { printed = append(printed, s) }})
	want := []string{`<<"view", 0>>  0`, `<<"view", 1>>  1`, `<<"view", 2>>  2`, `<<"view", 0>>  0`, `<<"view", 1>>  1`}
	if err != nil || got.Status != InvariantViolation || !reflect.DeepEqual(printed, want) {
		t.Fatalf("got %+v, %v, %q; want %q", got, err, printed, want)
	}
}

func TestViewAliasAndCounterExample(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals, TLC, TLCExt
VARIABLES x, y
Init == x = 0 /\ y = 10
Next == x < 2 /\ x' = x + 1 /\ y' = y + 1
View == x
Inv == x < 2
Alias == [shown |-> x, previous |-> y, future |-> x']
Post == PrintT(ToTrace(CounterExample))
====`
	var printed []string
	got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next VIEW View INVARIANT Inv ALIAS Alias POSTCONDITION Post", Options{OnPrint: func(s string) { printed = append(printed, s) }})
	want := []string{"<< [shown |-> 0, previous |-> 10, future |-> 1],\n   [shown |-> 1, previous |-> 11, future |-> 2],\n   [shown |-> 2, previous |-> 12, future |-> 2] >>"}
	if err != nil || got.Status != InvariantViolation || !reflect.DeepEqual(printed, want) {
		t.Fatalf("got %+v, %v, %q; want %q", got, err, printed, want)
	}
}

func TestViewLivenessReevaluatesInitialState(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x = 0
Next == x' = x
View == Print(<<"view",x>>,x)
Prop == <> (x = 1)
====`
	var printed []string
	got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next VIEW View PROPERTY Prop CHECK_DEADLOCK FALSE", Options{OnPrint: func(s string) { printed = append(printed, s) }})
	want := []string{`<<"view", 0>>  0`, `<<"view", 0>>  0`, `<<"view", 0>>  0`, `<<"view", 0>>  0`}
	if err != nil || got.Status != LivenessViolation || !reflect.DeepEqual(printed, want) {
		t.Fatalf("got %+v, %v, %q; want %q", got, err, printed, want)
	}
}

func TestViewLivenessFingerprintEvaluationOrder(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x = 0
Next == x < 2 /\ x' = x + 1
View == Print(<<"view",x>>,x)
Prop == <> (x = 5)
====`
	var printed []string
	got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next VIEW View PROPERTY Prop CHECK_DEADLOCK FALSE", Options{OnPrint: func(s string) { printed = append(printed, s) }})
	want := []string{`<<"view", 0>>  0`, `<<"view", 1>>  1`, `<<"view", 1>>  1`, `<<"view", 0>>  0`, `<<"view", 2>>  2`, `<<"view", 2>>  2`, `<<"view", 0>>  0`, `<<"view", 1>>  1`, `<<"view", 2>>  2`}
	if err != nil || got.Status != LivenessViolation || !reflect.DeepEqual(printed, want) {
		t.Fatalf("got %+v, %v, %q; want %q", got, err, printed, want)
	}
}

func TestViewLevelDuringRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, property, config string
		status                 Status
		printed                []string
	}{
		{"safety", "Inv == x < 2", "INVARIANT Inv", InvariantViolation, []string{`<<"view", 0, 1>>  0`, `<<"view", 1, 2>>  1`, `<<"view", 2, 3>>  2`, `<<"view", 0, 3>>  0`, `<<"view", 1, 3>>  1`}},
		{"liveness", "Prop == <> (x = 5)", "PROPERTY Prop CHECK_DEADLOCK FALSE", LivenessViolation, []string{`<<"view", 0, 1>>  0`, `<<"view", 1, 2>>  1`, `<<"view", 1, 2>>  1`, `<<"view", 0, 1>>  0`, `<<"view", 2, 3>>  2`, `<<"view", 2, 3>>  2`, `<<"view", 0, 0>>  0`, `<<"view", 1, 0>>  1`, `<<"view", 2, 0>>  2`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nNext == x < 2 /\\ x' = x+1\nView == Print(<<\"view\",x,TLCGet(\"level\")>>,x)\n" + tc.property + "\n===="
			var printed []string
			got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next VIEW View "+tc.config, Options{OnPrint: func(s string) { printed = append(printed, s) }})
			if err != nil || got.Status != tc.status || !reflect.DeepEqual(printed, tc.printed) {
				t.Fatalf("got %+v, %v, %q; want %q", got, err, printed, tc.printed)
			}
		})
	}
}
