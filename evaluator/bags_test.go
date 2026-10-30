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

func TestUpstreamBagsExpressions(t *testing.T) {
	f, err := os.Open("testdata/upstream-bags.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var sample struct{ Expr, Want string }
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(sample.Expr, func(t *testing.T) {
			m, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Bags, Integers, FiniteSets, TLC\nResult == " + sample.Expr + "\n====")
			if err != nil {
				t.Fatal(err)
			}
			got, err := Evaluate(m, m.Definitions[0].Body, Context{})
			if err != nil {
				t.Fatal(err)
			}
			wantModule, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Bags, Integers, FiniteSets, TLC\nResult == " + sample.Want + "\n====")
			if err != nil {
				t.Fatal(err)
			}
			want, err := Evaluate(wantModule, wantModule.Definitions[0].Body, Context{})
			if err != nil {
				t.Fatal(err)
			}
			equal, err := value.Equal(got, want)
			if err != nil || !equal {
				t.Fatalf("got %s, want %s (comparison: %v)", got, sample.Want, err)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 32 {
		t.Fatalf("got %d cases, want 32", count)
	}
}

func TestBagsRejectInvalidArguments(t *testing.T) {
	for _, tc := range []struct{ expr, message string }{
		{"BagCardinality(<<0>>)", "which is not a bag"},
		{"<<0>> (+) EmptyBag", "first argument of (+) should be a bag"},
		{"EmptyBag (-) 1", "second argument of (-) should be a bag"},
		{"BagUnion({1, 2})", "element is not a bag"},
		{"BagUnion(Nat)", "not a finite enumerable set"},
		{"SubBag(<<0>>)", "which is not in its domain"},
		{"BagOfAll(LAMBDA n : 1, <<1>>)", "first argument of BagOfAll should be an operator"},
		{"BagOfAll(LAMBDA n : n, 1)", "function with a finite domain"},
		{"<<2147483647>> (+) <<1>>", "Overflow when computing 2147483647+1"},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			m, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Bags, Naturals\nResult == " + tc.expr + "\n====")
			if err != nil {
				t.Fatal(err)
			}
			_, err = Evaluate(m, m.Definitions[0].Body, Context{})
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("got %v, want %q", err, tc.message)
			}
		})
	}
}

func TestBagJavaCastErrors(t *testing.T) {
	for _, tc := range []struct{ expr, signature string }{
		{`<<"a">> (-) EmptyBag`, "tlc2.value.impl.Value tlc2.module.Bags.BagDiff"},
		{`<<"a">> \sqsubseteq EmptyBag`, "tlc2.value.IBoolValue tlc2.module.Bags.SqSubseteq"},
		{`BagOfAll(LAMBDA e : TRUE, <<"a", 1>>)`, "tlc2.value.impl.Value tlc2.module.Bags.BagOfAll"},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			m, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Bags\nResult == " + tc.expr + "\n====")
			if err != nil {
				t.Fatal(err)
			}
			_, err = Evaluate(m, m.Definitions[0].Body, Context{})
			want := "Attempted to apply the operator overridden by the Java method\npublic static " + tc.signature + "(tlc2.value.impl.Value,tlc2.value.impl.Value),\nbut it produced the following error:\nclass tlc2.value.impl.StringValue cannot be cast to class tlc2.value.impl.IntValue (tlc2.value.impl.StringValue and tlc2.value.impl.IntValue are in unnamed module of loader 'app')"
			if err == nil || err.Error() != want {
				t.Fatalf("got %v, want %s", err, want)
			}
		})
	}
}

func BenchmarkBagUnion(b *testing.B) {
	m, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Bags\nResult == BagUnion({<<1,2,3,4>>, <<4,3,2,1>>, <<2,2,2,2>>})\n====")
	if err != nil {
		b.Fatal(err)
	}
	program := New(m)
	expr := m.Definitions[0].Body
	b.ReportAllocs()
	for b.Loop() {
		if _, err := program.Evaluate(expr, Context{}); err != nil {
			b.Fatal(err)
		}
	}
}
