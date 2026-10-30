package checker

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamAny(t *testing.T) {
	for _, tc := range []struct{ expression, message string }{
		{`1 \in Any /\ "a" \in Any /\ {} \in Any /\ {1} \subseteq Any /\ ~IsFiniteSet(Any) /\ ToString(<<Any>>) = "<<ANY>>"`, ""},
		{`Any = Any`, "Attempted to compare ANY with the value\nANY"},
		{`{Any} = {Any}`, "Attempted to compare ANY with the value\nANY"},
		{`<<Any>> = <<Any>>`, "Attempted to compare ANY with the value\nANY"},
		{`Any \in {Any}`, "Attempted to compare ANY with the value\nANY"},
		{`Any = {1}`, "Attempted to compare overridden value ANY with non-overridden value:\n{1}"},
		{`Any = Nat`, "Attempted to compare ANY with the value\nNat"},
		{`{1} = Any`, "Attempted to check equality of the set {1} with the value:\nANY"},
		{`Nat = Any`, "Attempted to compare Nat with the value\nANY"},
		{`Cardinality(Any) = 0`, "Attempted to compute cardinality of the value\nANY"},
		{`Any \in Any`, ""},
		{`Cardinality(Any \cap {1,2}) = 2`, ""},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Integers, FiniteSets, TLC\nASSUME " + tc.expression + "\n===="
			result, err := Check(source, "")
			if err != nil {
				t.Fatal(err)
			}
			if tc.message == "" && result.Status == Success {
				return
			}
			if result.Status != AssumptionViolation || result.Violation == nil || result.Violation.Message != tc.message {
				t.Fatalf("got %+v, want %q", result, tc.message)
			}
		})
	}
}

func TestAnyRequiresTLC(t *testing.T) {
	source := "---- MODULE M ----\nASSUME 1 \\in Any\n===="
	if _, err := Check(source, ""); err == nil {
		t.Fatal("Any must not be available without EXTENDS TLC")
	}
}

func TestAnyAndModelValueEquality(t *testing.T) {
	for _, tc := range []struct{ expression, message string }{
		{"Any = C", ""},
		{"C = Any", ""},
		{"Any = M_1", "Attempted to check equality of the typed model value M_1 and the non-model value\nANY"},
		{"M_1 = Any", "Attempted to check equality of typed model value M_1 and non-model value\nANY"},
	} {
		source := "---- MODULE M ----\nEXTENDS TLC\nCONSTANT C, M_1\nASSUME " + tc.expression + "\n===="
		result, err := Check(source, "CONSTANT C = C M_1 = M_1")
		if err != nil || result.Status != AssumptionViolation || result.Violation == nil || result.Violation.Message != tc.message {
			t.Fatalf("%s: got %+v, %v", tc.expression, result, err)
		}
	}
}

func TestUpstreamAnyMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-any.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var sample struct{ Name, Source, Config, Status, Message string }
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(sample.Name, func(t *testing.T) {
			result, err := Check(sample.Source, sample.Config)
			if err != nil {
				t.Fatal(err)
			}
			if string(result.Status) != sample.Status || result.Violation == nil || result.Violation.Message != sample.Message {
				t.Fatalf("got %+v, want status %s message %q", result, sample.Status, sample.Message)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 28 {
		t.Fatalf("got %d cases, want 28", count)
	}
}
