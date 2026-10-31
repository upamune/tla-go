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

// Cases from printing.test.ts at the pinned tla-ts revision. Where tla-ts
// and Java TLC 1.8.0 print SUBSET (1..7), Java TLC 1.7.4 prints SUBSET 1..7,
// which is expected here.
func TestUpstreamPrinting(t *testing.T) {
	for _, tc := range []struct{ expression, output string }{
		{"3..1", "{}"},
		{"<<3..1>>", "<<{}>>"},
		{"SUBSET {1}", "{{}, {1}}"},
		{"SUBSET (1..7)", "SUBSET 1..7"},
		{"SUBSET SUBSET (1..3)", "SUBSET {{}, {1}, {2}, {3}, {1, 2}, {1, 3}, {2, 3}, {1, 2, 3}}"},
		{"SUBSET (3..1)", "{{}}"},
		{"[{1} -> {2, 3}]", "{<<2>>, <<3>>}"},
		{"[1..2 -> 1..45]", "[1..2 -> 1..45]"},
		{"[{} -> {1}]", "{<<>>}"},
		{"[a : {1, 2}]", "{[a |-> 1], [a |-> 2]}"},
		{"[a : 1..2000]", "[a: 1..2000]"},
		{"{1} \\X {2, 3}", "{<<1, 2>>, <<1, 3>>}"},
		{"(1..40) \\X (1..50)", "(1..40 \\X 1..50)"},
		{"{} \\X Nat", "{}"},
		{"<<SUBSET (1..7)>>", "<<SUBSET 1..7>>"},
		{"{} \\cup (0..0)", "0..0"},
		{"(0..1) \\cup {}", "{0, 1}"},
		{"DOMAIN [i \\in 0..1 |-> 0]", "0..1"},
		{"DOMAIN <<1, 2>>", "1..2"},
		{"DOMAIN ((1 :> 1) @@ (2 :> 2))", "{1, 2}"},
		{"DOMAIN [[i \\in 0..1 |-> 0] EXCEPT ![0] = 1]", "0..1"},
		{"{DOMAIN f : f \\in [0..1 -> {0, 1}]}", "{0..1}"},
		{"ToString(SUBSET (1..7))", `"SUBSET 1..7"`},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Integers, TLC\nASSUME PrintT(" + tc.expression + ")\n===="
			var lines []string
			result, err := CheckWithOptions(context.Background(), source, "", Options{OnPrint: func(s string) { lines = append(lines, s) }})
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != Success || !reflect.DeepEqual(lines, []string{tc.output}) {
				t.Fatalf("got %+v and %#v, want %q", result, lines, tc.output)
			}
		})
	}
}

func TestPrintReturnsSecondValue(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS TLC\nASSUME Print(1, TRUE)\nASSUME PrintT(2)\n===="
	var lines []string
	result, err := CheckWithOptions(context.Background(), source, "", Options{OnPrint: func(s string) { lines = append(lines, s) }})
	if err != nil || result.Status != Success || !reflect.DeepEqual(lines, []string{"1  TRUE", "2"}) {
		t.Fatalf("result: %+v; error: %v; output: %#v", result, err, lines)
	}
}

func TestInvalidPrintedSetFailsAssumption(t *testing.T) {
	for _, expr := range []string{"PrintT(SUBSET Nat)", "Print([Nat -> {1}], TRUE)", `ToString(Nat \X {1}) = ""`} {
		source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nASSUME " + expr + "\n===="
		result, err := CheckWithOptions(context.Background(), source, "", Options{})
		if err != nil || result.Status != AssumptionViolation {
			t.Fatalf("%s: got %+v, %v", expr, result, err)
		}
	}
}

func TestUpstreamPrintOrder(t *testing.T) {
	f, err := os.Open("testdata/upstream-printing.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var sample struct {
			Name, Source, Config string
			Printed              []string
			Want                 expectedCheck
		}
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(sample.Name, func(t *testing.T) {
			if sample.Name == "assumptions" || sample.Name == "short-circuit" || sample.Name == "operator-argument" {
				// These no-state models report depth 1 in Java TLC.
				sample.Want.Depth = 1
			}
			var printed []string
			result, err := CheckWithOptions(context.Background(), sample.Source, sample.Config, Options{OnPrint: func(s string) { printed = append(printed, s) }})
			if err != nil {
				t.Fatal(err)
			}
			got := project(result)
			got.Trace = nil
			if !reflect.DeepEqual(got, sample.Want) || !reflect.DeepEqual(printed, sample.Printed) {
				t.Fatalf("result: %#v, want %#v; printed: %#v, want %#v", got, sample.Want, printed, sample.Printed)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 6 {
		t.Fatalf("got %d cases, want 6", count)
	}
}

func TestUpstreamPrettyPrinting(t *testing.T) {
	f, err := os.Open("testdata/upstream-pretty.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	count := 0
	for scanner.Scan() {
		var sample struct {
			Expr, Status string
			Printed      []string
		}
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(sample.Expr, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nASSUME PrintT(" + sample.Expr + ")\n===="
			var printed []string
			result, err := CheckWithOptions(context.Background(), source, "", Options{OnPrint: func(s string) { printed = append(printed, s) }})
			if err != nil || string(result.Status) != sample.Status || !reflect.DeepEqual(printed, sample.Printed) {
				t.Fatalf("result: %+v, error: %v\nprinted: %#v\nwant: %#v", result, err, printed, sample.Printed)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 8 {
		t.Fatalf("got %d cases, want 8", count)
	}
}

func TestUpstreamAssertPrettyPrinting(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nASSUME Assert(FALSE, [x \\in 1..10 |-> [y \\in 1..10 |-> y]])\n===="
	result, err := Check(source, "")
	if err != nil || result.Status != AssumptionViolation || result.Violation == nil {
		t.Fatalf("got %+v, %v", result, err)
	}
	line := "<<1, 2, 3, 4, 5, 6, 7, 8, 9, 10>>"
	want := "The first argument of Assert evaluated to FALSE; the second argument was:\n<< " + strings.TrimSuffix(strings.Repeat(line+",\n   ", 10), ",\n   ") + " >>"
	if result.Violation.Message != want {
		t.Fatalf("got %q, want %q", result.Violation.Message, want)
	}
}

// These are the non-enumerable function cases from upstream printing.test.ts.
func TestUpstreamInfiniteFunctionPrinting(t *testing.T) {
	for _, tc := range []struct{ body, output string }{
		{`ASSUME PrintT([n \in Nat |-> n])`, `[n \in Nat |-> <expression line 3, col 30 to line 3, col 30 of module M>]`},
		{`ASSUME PrintT([n, m \in Nat |-> n])`, `[n \in Nat |-> <expression line 3, col 33 to line 3, col 33 of module M>]`},
		{`ASSUME PrintT([n \in Nat, m \in 1..2 |-> n])`, `[n \in Nat, m \in 1..2 |-> <expression line 3, col 42 to line 3, col 42 of module M>]`},
		{`ASSUME PrintT([<<a, b>> \in Nat \X Nat |-> a])`, `[<<a, b>> \in (Nat \X Nat) |-> <expression line 3, col 44 to line 3, col 44 of module M>]`},
		{"ASSUME PrintT([n \\in Nat |->\n  n + 1])", `[n \in Nat |-> <expression line 4, col 3 to line 4, col 7 of module M>]`},
		{`ASSUME PrintT([[n \in Int |-> n] EXCEPT ![1] = 5])`, `[n \in Int |-> <expression line 3, col 31 to line 3, col 31 of module M>]`},
		{"f[n \\in Nat] == IF n = 0 THEN 0 ELSE f[n - 1]\nASSUME PrintT(f)", `[n \in Nat |-> <expression line 3, col 17 to line 3, col 45 of module M>]`},
		{`ASSUME PrintT(ToString([n \in Nat \ {1} |-> n]))`, `"[n \\in Nat \\ {1} |-> <expression line 3, col 45 to line 3, col 45 of module M>]"`},
	} {
		t.Run(tc.body, func(t *testing.T) {
			var lines []string
			source := "---- MODULE M ----\nEXTENDS Integers, TLC\n" + tc.body + "\n===="
			result, err := CheckWithOptions(context.Background(), source, "", Options{OnPrint: func(s string) { lines = append(lines, s) }})
			if err != nil || result.Status != Success || !reflect.DeepEqual(lines, []string{tc.output}) {
				t.Fatalf("result: %+v, err: %v, got %#v, want %q", result, err, lines, tc.output)
			}
		})
	}
}
