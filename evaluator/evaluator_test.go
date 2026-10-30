package evaluator

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestUpstreamExpressionResults(t *testing.T) {
	f, err := os.Open("testdata/upstream-expressions.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var sample struct{ Expr, Want string }
		if err = json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(sample.Expr, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Integers, FiniteSets, Sequences, TLC\nVARIABLE x\nResult == " + sample.Expr + "\n===="
			m, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			two, _ := value.Integer(2)
			three, _ := value.Integer(3)
			got, err := Evaluate(m, m.Definitions[0].Body, Context{Current: State{"x": two}, Next: State{"x": three}})
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != sample.Want {
				t.Fatalf("got %s, want %s", got, sample.Want)
			}
		})
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 180 {
		t.Fatalf("expected 180 examples, got %d", count)
	}
}

func TestEvaluationErrors(t *testing.T) {
	for _, expr := range []string{"2147483647 + 1", "1 \\div 0", "1 % 0", "0 ^ 0", "2 ^ 31", "x'"} {
		m, err := parser.ParseModule("---- MODULE M ----\nVARIABLE x\nA == " + expr + "\n====")
		if err != nil {
			t.Fatal(err)
		}
		v, _ := value.Integer(1)
		if _, err = Evaluate(m, m.Definitions[0].Body, Context{Current: State{"x": v}}); err == nil {
			t.Errorf("%s was accepted", expr)
		}
	}
}

func TestUnsupportedExpressionsFail(t *testing.T) {
	m, err := parser.ParseModule("---- MODULE M ----\nA == []TRUE\n====")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(m, m.Definitions[0].Body, Context{})
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("got %v", err)
	}
}

func TestLargeIntervalShortCircuits(t *testing.T) {
	m, err := parser.ParseModule("---- MODULE M ----\nA == \\E n \\in 1..2147483647 : n = 1\n====")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(m, m.Definitions[0].Body, Context{})
	if err != nil || result.String() != "TRUE" {
		t.Fatalf("got %s, %v", result, err)
	}
}

func BenchmarkRepeatedEvaluation(b *testing.B) {
	m, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nA == x + 1\n====")
	if err != nil {
		b.Fatal(err)
	}
	p := New(m)
	v, _ := value.Integer(5)
	ctx := Context{Current: State{"x": v}}
	expr := m.Definitions[0].Body
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err = p.Evaluate(expr, ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func TestLargeSubsetComparisonShortCircuits(t *testing.T) {
	m, err := parser.ParseModule("---- MODULE M ----\nA == 1..2147483647 \\subseteq {}\n====")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Evaluate(m, m.Definitions[0].Body, Context{})
	if err != nil || got.String() != "FALSE" {
		t.Fatalf("got %s, %v", got, err)
	}
}

func TestSetOperationsRequireSetOperands(t *testing.T) {
	for _, expr := range []string{"{} \\subseteq 1", "{} \\cap 1", "{} \\ 1"} {
		m, err := parser.ParseModule("---- MODULE M ----\nA == " + expr + "\n====")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Evaluate(m, m.Definitions[0].Body, Context{}); err == nil {
			t.Errorf("accepted %s", expr)
		}
	}
}

func TestFiniteSetsErrorsAndModuleScope(t *testing.T) {
	cases := []struct{ extensions, expression string }{
		{"Integers, FiniteSets", "Cardinality(SUBSET (1..31))"},
		{"Integers, FiniteSets", "Cardinality((1..50000) \\X (1..50000))"},
		{"Integers, FiniteSets", "Cardinality(1)"},
		{"Integers, FiniteSets", "IsFiniteSet(1)"},
		{"Integers", "Cardinality({1})"},
	}
	for _, tc := range cases {
		m, err := parser.ParseModule("---- MODULE M ----\nEXTENDS " + tc.extensions + "\nA == " + tc.expression + "\n====")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Evaluate(m, m.Definitions[0].Body, Context{}); err == nil {
			t.Errorf("accepted %s with %s", tc.expression, tc.extensions)
		}
	}
}

func TestSequenceUnicodeAndErrors(t *testing.T) {
	for _, tc := range []struct {
		expression, want string
		failure          bool
	}{
		{"Len(\"😀\")", "2", false},
		{"Tail(\"a😀\")", "\"😀\"", false},
		{"Tail(\"😀\")", "", true},
		{"Head(<<>>)", "", true},
		{"SubSeq(<<1,2>>, 1, 3)", "", true},
	} {
		m, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Sequences\nA == " + tc.expression + "\n====")
		if err != nil {
			t.Fatal(err)
		}
		got, err := Evaluate(m, m.Definitions[0].Body, Context{})
		if tc.failure {
			if err == nil {
				t.Errorf("%s unexpectedly succeeded", tc.expression)
			}
			continue
		}
		if err != nil || got.String() != tc.want {
			t.Errorf("%s: got %s, %v; want %s", tc.expression, got, err, tc.want)
		}
	}
}
