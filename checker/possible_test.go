package checker

import (
	"bufio"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUpstreamPossibleFailures(t *testing.T) {
	source, err := os.ReadFile("testdata/possible/PossibleFail.tla")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, predicate, counts, location string }{
		{"PossibleFailStateTest", "Unreachable", "[Unreachable |-> 0]", "line 12, col 16 to line 12, col 20"},
		{"PossibleFailActionTest", "BigJump", "[BigJump |-> 0]", "line 16, col 15 to line 16, col 24"},
		{"PossibleFailMixedTest", "Unreachable", "[AtOne |-> 1, AllDone |-> 1, Unreachable |-> 0, WrapAround |-> 1, BigJump |-> 0]", "line 12, col 16 to line 12, col 20"},
		{"PossibleFailNoTransTest", "BigJump", "<<>>", "line 16, col 15 to line 16, col 24"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := os.ReadFile("testdata/possible/" + tc.name + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			result, err := Check(string(source), string(cfg)+"\nCHECK_DEADLOCK FALSE")
			if err != nil {
				t.Fatal(err)
			}
			wantMessage := "The _POSSIBLE predicate " + tc.predicate + " at " + tc.location + " of module PossibleFail was never witnessed.\nOthers may be unwitnessed too; only witnessed predicates appear with a\nnon-zero count in the record printed above.\nSee https://explain.tlapl.us/possible-conditions for additional details."
			if result.Status != PostconditionViolation || result.Violation == nil || result.Violation.Name != tc.predicate || result.Violation.Counts != tc.counts || result.Violation.Message != wantMessage {
				t.Fatalf("got %+v violation=%+v", result, result.Violation)
			}
		})
	}
}

func TestPossibleDefinitionOrder(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nZeta == x = 5\nAlpha == x = 6\nMid == x = 7\n===="
	result, err := Check(source, "INIT Init NEXT Next _POSSIBLE Mid Alpha Zeta")
	if err != nil || result.Violation == nil || result.Violation.Counts != "[Zeta |-> 0, Alpha |-> 0, Mid |-> 0]" || result.Violation.Name != "Mid" {
		t.Fatalf("got %+v, %v", result, err)
	}
}

func TestPossibleAfterInvariantViolation(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nUnreachable == x = 5\nInv == x < 2\n===="
	result, err := Check(source, "INIT Init NEXT Next _POSSIBLE Unreachable INVARIANT Inv")
	if err != nil || result.Status != InvariantViolation || result.Violation == nil || result.Violation.Unwitnessed == nil {
		t.Fatalf("got %+v, %v", result, err)
	}
	want := "The _POSSIBLE predicate Unreachable at line 6, col 16 to line 6, col 20 of module M was never witnessed.\nOthers may be unwitnessed too; only witnessed predicates appear with a\nnon-zero count in the record printed above.\nSee https://explain.tlapl.us/possible-conditions for additional details."
	got := result.Violation.Unwitnessed
	if got.Name != "Unreachable" || got.Message != want || got.Counts != "[Unreachable |-> 0]" {
		t.Fatalf("got %+v", got)
	}
}

func TestInvalidPossiblePredicates(t *testing.T) {
	body := "AtOne == x = 1\nConstant == TRUE\nArgument(n) == x = n\nTemporal == <>(x = 1)"
	for _, tc := range []struct{ possible, message string }{
		{"Undefined", "The possible Undefined specified in the configuration file\nis not defined in the specification."},
		{"x", "The possible x specified in the configuration file\nis not defined in the specification."},
		{"Constant", "The possible Constant cannot be a constant."},
		{"Argument", "TLC requires possible not to take any argument, but one was given: Argument"},
		{"Temporal", "The possible of Temporal is equal to a temporal formula; only state- and action-level predicates are supported"},
		{"AtOne AtOne", "_POSSIBLE lists AtOne more than once"},
	} {
		source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\n" + body + "\n===="
		_, err := Check(source, "INIT Init NEXT Next _POSSIBLE "+tc.possible)
		if err == nil || err.Error() != tc.message {
			t.Fatalf("%s: got %v, want %q", tc.possible, err, tc.message)
		}
	}
}

func TestUpstreamPossibleMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-possible-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var sample struct {
			Name, Source, Config, Status, NameOf, Message, Counts string
			Unwitnessed                                           *PossibleViolation
			Generated, Distinct, Depth                            int
		}
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(sample.Name, func(t *testing.T) {
			result, err := Check(sample.Source, sample.Config)
			if err != nil {
				t.Fatal(err)
			}
			if string(result.Status) != sample.Status || result.GeneratedStates != sample.Generated || result.DistinctStates != sample.Distinct || result.Depth != sample.Depth {
				t.Fatalf("got %+v, want %+v", result, sample)
			}
			if result.Violation == nil {
				if sample.NameOf != "" {
					t.Fatalf("missing violation: %+v", sample)
				}
				return
			}
			got := result.Violation
			if got.Name != sample.NameOf || got.Message != sample.Message || got.Counts != sample.Counts || !reflect.DeepEqual(got.Unwitnessed, sample.Unwitnessed) {
				t.Fatalf("got %+v unwitnessed=%+v, want name=%s message=%q counts=%q unwitnessed=%+v", got, got.Unwitnessed, sample.NameOf, sample.Message, sample.Counts, sample.Unwitnessed)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 240 {
		t.Fatalf("got %d cases, want 240", count)
	}
}

func TestPossibleCountsInNamedRegisters(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nASSUME TLCSet(\"s:_possible\", [ignored |-> 0])\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nFirst == x = 1\nPost == /\\ TLCGet(\"s:_possible\")[\"First\"] = 1 /\\ TLCGet(\"all:named\")[\"s:_possible\"][1][\"First\"] = 1\n===="
	result, err := Check(source, "INIT Init NEXT Next _POSSIBLE First POSTCONDITION Post")
	if err != nil || result.Status != Success || result.DistinctStates != 3 {
		t.Fatalf("got %+v, %v", result, err)
	}
}

func TestUpstreamPossiblePostconditions(t *testing.T) {
	source, err := os.ReadFile("testdata/postconditions/Postconditions.tla")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		status    Status
		violation string
	}{
		{"Holds", Success, ""},
		{"Unwitnessed", PostconditionViolation, "AtTen"},
		{"False", PostconditionViolation, "FiveStates"},
	} {
		cfg, err := os.ReadFile("testdata/postconditions/" + tc.name + ".cfg")
		if err != nil {
			t.Fatal(err)
		}
		result, err := Check(string(source), string(cfg))
		if err != nil || result.Status != tc.status || result.DistinctStates != 4 {
			t.Fatalf("%s: got %+v, %v", tc.name, result, err)
		}
		if tc.violation != "" && (result.Violation == nil || result.Violation.Name != tc.violation) {
			t.Fatalf("%s: got violation %+v", tc.name, result.Violation)
		}
	}
}

func BenchmarkPossibleModel(b *testing.B) {
	source, err := os.ReadFile("testdata/tlc-set/TLCSet.tla")
	if err != nil {
		b.Fatal(err)
	}
	cfg, err := os.ReadFile("testdata/tlc-set/TLCSet.cfg")
	if err != nil {
		b.Fatal(err)
	}
	module := strings.Replace(string(source), "=============================================================================", "Seen == x > 0\n=============================================================================", 1)
	configuration := string(cfg) + "\nCHECK_DEADLOCK FALSE\n_POSSIBLE Seen"
	b.ReportAllocs()
	for b.Loop() {
		result, err := Check(module, configuration)
		if err != nil || result.Status != Success {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}
