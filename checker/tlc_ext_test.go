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

func TestUpstreamTLCExtFixtures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		printed []string
	}{
		{"Defer", nil},
		{"AssertEq", []string{`"Assertion failed"`, "1", "0", `"Assertion failed"`, "2", "0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/tlc-ext/" + tc.name + ".tla")
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := os.ReadFile("testdata/tlc-ext/" + tc.name + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			var lines []string
			got, err := CheckWithOptions(context.Background(), string(source), string(cfg), Options{OnPrint: func(s string) { lines = append(lines, s) }})
			if err != nil || got.Status != Success || got.GeneratedStates != 3 || got.DistinctStates != 3 || got.Depth != 3 || !reflect.DeepEqual(lines, tc.printed) {
				t.Fatalf("result: %+v, error: %v, output: %q", got, err, lines)
			}
		})
	}
}

func TestUpstreamConstraintEvaluationOrder(t *testing.T) {
	source, err := os.ReadFile("testdata/tlc-constraint-order/Order.tla")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile("testdata/tlc-constraint-order/Order.cfg")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/tlc-constraint-order/Order.out")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	got, err := CheckWithOptions(context.Background(), string(source), string(cfg), Options{OnPrint: func(s string) { lines = append(lines, s) }})
	if err != nil || got.Status != Success || got.GeneratedStates != 8 || got.DistinctStates != 4 || got.Depth != 3 || strings.Join(lines, "\n") != strings.TrimSpace(string(want)) {
		t.Fatalf("result: %+v, error: %v, output: %q", got, err, lines)
	}
}

func TestTLCExtOperators(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals, TLC, TLCExt
VARIABLE x
ASSUME /\ TLCDefer(PrintT("not evaluated") /\ 1 \div 0 = 0)
       /\ TLCNoOp(3) = 3
       /\ TLCModelValue("a") = TLCModelValue("a")
       /\ AssertError("The second argument of \\div is 0.", 1 \div 0)
       /\ AssertError("Attempted to check if the value:\n{}\nis in the integer interval 1..4", {} \in 1..4)
       /\ ~AssertError("other", 1 \div 0)
       /\ ~AssertError("anything", 2)
       /\ TLCGetOrDefault(7, 99) = 99
       /\ TLCSet(7, 42)
       /\ TLCGetOrDefault(7, 99) = 42
       /\ TLCGetOrDefault("level", 55) = 55
       /\ TLCGetOrDefault("s:test", 12) = 12
       /\ TLCSet("s:test", 21)
       /\ TLCGetOrDefault("s:test", 12) = 21
       /\ ToTrace([action |-> {}, state |-> {<<2, [x |-> 2]>>, <<1, [x |-> 1]>>}]) = <<[x |-> 1], [x |-> 2]>>
Init == x = 0 /\ TLCDefer(x' = 1) /\ TLCDefer(<>(x = 1))
Next == x' = x
====`
	var lines []string
	got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next CHECK_DEADLOCK FALSE", Options{OnPrint: func(s string) { lines = append(lines, s) }})
	if err != nil || got.Status != Success || got.DistinctStates != 1 || len(lines) != 0 {
		t.Fatalf("result: %+v, error: %v, output: %q", got, err, lines)
	}
}

func TestUpstreamTLCGetAndSetStopsSearch(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLCExt\nVARIABLE x\nInit == x = 0\nNext == x < 5 /\\ x' = x + 1 /\\ (x = 2 => TLCGetAndSet(\"exit\", LAMBDA old, new : new, TRUE, FALSE) = FALSE)\n===="
	result, err := Check(source, "INIT Init NEXT Next")
	if err != nil || result.Status != Success || result.GeneratedStates != 4 || result.DistinctStates != 4 || result.QueuedStates != 1 {
		t.Fatalf("got %+v, %v", result, err)
	}
}

func TestUpstreamTLCExtMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-tlc-ext-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var tc struct {
			Name, Source, Config, Status string
			Printed                      []string
			Generated, Distinct, Depth   int
		}
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			var lines []string
			got, err := CheckWithOptions(context.Background(), tc.Source, tc.Config, Options{OnPrint: func(s string) { lines = append(lines, s) }})
			if err != nil || string(got.Status) != tc.Status || got.GeneratedStates != tc.Generated || got.DistinctStates != tc.Distinct || got.Depth != tc.Depth || !reflect.DeepEqual(lines, tc.Printed) {
				t.Fatalf("result: %+v, error: %v, output: %q; expected %s %d %d %d %q", got, err, lines, tc.Status, tc.Generated, tc.Distinct, tc.Depth, tc.Printed)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 48 {
		t.Fatalf("got %d cases, want 48", count)
	}
}

func TestTLCModelValueWithWideString(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS TLCExt\nVARIABLES x, y\nInit == /\\ x = TLCModelValue(\"a\") /\\ y = \"あ\"\nNext == UNCHANGED <<x, y>>\n===="
	result, err := Check(source, "INIT Init NEXT Next CHECK_DEADLOCK FALSE")
	if err != nil || result.Status != Success || result.DistinctStates != 1 {
		t.Fatalf("got %+v, %v", result, err)
	}
}

func TestUpstreamTLCExtTrace(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		status                     Status
		generated, distinct, depth int
	}{
		{"TLCExtTrace", Deadlock, 10, 10, 10},
		{"TraceNext", Success, 5, 5, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/tlc-ext/" + tc.name + ".tla")
			if err != nil {
				t.Fatal(err)
			}
			config, err := os.ReadFile("testdata/tlc-ext/" + tc.name + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			result, err := Check(string(source), string(config))
			if err != nil || result.Status != tc.status || result.GeneratedStates != tc.generated || result.DistinctStates != tc.distinct || result.Depth != tc.depth {
				t.Fatalf("got %+v, %v", result, err)
			}
		})
	}
	source, err := os.ReadFile("testdata/tlc-ext/TraceAssume.tla")
	if err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile("testdata/tlc-ext/TraceAssume.cfg")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Check(string(source), string(config))
	want := "Attempted to apply the operator overridden by the Java method\npublic static tlc2.value.impl.TupleValue tlc2.module.TLCExt.getTrace(tlc2.tool.impl.Tool,tla2sany.semantic.ExprOrOpArgNode[],tlc2.util.Context,tlc2.tool.TLCState,tlc2.tool.TLCState,int,tlc2.tool.coverage.CostModel) throws java.io.IOException,\nbut it produced the following error:\nIn evaluating TLCExt!Trace, the state is not completely specified yet (variables x, y undefined)."
	if err != nil || result.Status != AssumptionViolation || result.Violation == nil || result.Violation.Message != want {
		t.Fatalf("got %+v, %v", result, err)
	}
}

func TestUpstreamTLCTraceMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-tlc-trace-matrix.jsonl")
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
			Generated, Distinct, Depth   int
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
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 36 {
		t.Fatalf("got %d cases, want 36", count)
	}
}

func TestTraceWithoutState(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS TLCExt\nASSUME Trace = <<>>\n===="
	result, err := Check(source, "")
	if err != nil || result.Status != AssumptionViolation || result.Violation == nil || !strings.Contains(result.Violation.Message, "variable  undefined") {
		t.Fatalf("got %+v, %v", result, err)
	}
}

func BenchmarkTraceNext(b *testing.B) {
	source, err := os.ReadFile("testdata/tlc-ext/TraceNext.tla")
	if err != nil {
		b.Fatal(err)
	}
	config, err := os.ReadFile("testdata/tlc-ext/TraceNext.cfg")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		result, err := Check(string(source), string(config))
		if err != nil || result.Status != Success || result.DistinctStates != 5 {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}

func TestUpstreamTLCTraceLiveness(t *testing.T) {
	f, err := os.Open("testdata/upstream-tlc-trace-liveness.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var tc struct {
			Name, Source, Config, Status string
			Generated, Distinct, Depth   int
		}
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			got, err := Check(tc.Source, tc.Config)
			if err != nil || string(got.Status) != tc.Status || got.GeneratedStates != tc.Generated || got.DistinctStates != tc.Distinct || got.Depth != tc.Depth {
				t.Fatalf("got %+v, %v; want %s %d %d %d", got, err, tc.Status, tc.Generated, tc.Distinct, tc.Depth)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 24 {
		t.Fatalf("got %d cases, want 24", count)
	}
}

func TestUpstreamCounterExample(t *testing.T) {
	source, err := os.ReadFile("testdata/tlc-ext/CounterExample.tla")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		status Status
		steps  string
	}{
		{"CounterExampleInvariant", InvariantViolation, `{<<1, "Up", 2>>, <<2, "Up", 3>>}`},
		{"CounterExampleDeadlock", Deadlock, `{<<1, "Up", 2>>, <<2, "Up", 3>>}`},
		{"CounterExampleCycle", LivenessViolation, `{<<1, "Up", 2>>, <<2, "Up", 3>>, <<3, "Back", 2>>}`},
		{"CounterExampleStutter", LivenessViolation, `{<<1, "Up", 2>>, <<2, "Up", 3>>, <<3, "Up", 3>>}`},
		{"CounterExampleFails", InvariantViolation, `{<<1, "Up", 2>>, <<2, "Up", 3>>}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := os.ReadFile("testdata/tlc-ext/" + tc.name + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			var lines []string
			got, err := CheckWithOptions(context.Background(), string(source), string(config), Options{OnPrint: func(s string) { lines = append(lines, s) }})
			want := []string{`<<[x |-> 0], [x |-> 1], [x |-> 2]>>`, tc.steps}
			if err != nil || got.Status != tc.status || !reflect.DeepEqual(lines, want) {
				t.Fatalf("got %+v, %v, %q; want %s %q", got, err, lines, tc.status, want)
			}
		})
	}
}

func TestCounterExampleInitiallyEmpty(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS TLCExt
ASSUME CounterExample = [action |-> {}, state |-> {}]
ASSUME ToTrace(CounterExample) = <<>>
====`
	result, err := Check(source, "")
	if err != nil || result.Status != Success {
		t.Fatalf("got %+v, %v", result, err)
	}
}

func TestUpstreamCounterExampleMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-counter-example-matrix.jsonl")
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
			Generated, Distinct, Depth   int
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
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 24 {
		t.Fatalf("got %d cases, want 24", count)
	}
}
