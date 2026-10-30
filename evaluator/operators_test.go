package evaluator

import (
	"strings"
	"sync"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestHigherOperatorErrors(t *testing.T) {
	for _, expression := range []string{
		"SelectSeq(<<1>>, LAMBDA n : n)",
		"SelectSeq(<<1>>, LAMBDA a,b : a = b)",
		"LET Apply(F(_), v) == F(v) IN Apply(+, 1)",
	} {
		module, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Naturals, Sequences\nResult == " + expression + "\n====")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Evaluate(module, module.Definitions[0].Body, Context{}); err == nil {
			t.Errorf("accepted %s", expression)
		}
	}
}

func TestRecursiveLazyArgumentLoopStops(t *testing.T) {
	source := "---- MODULE M ----\nRECURSIVE F(_)\nF(a) == F(a)\nResult == F(1)\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(module, module.Definitions[len(module.Definitions)-1].Body, Context{})
	if err == nil || !strings.Contains(err.Error(), "recursive call") {
		t.Fatalf("expected a recursive call error, got %v", err)
	}
}

func TestLazyOperatorArgumentInConcurrentFunction(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nMake(a) == [i \\in 1..32 |-> a + i]\nResult == Make(x + 1)\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	one, _ := value.Integer(1)
	function, err := Evaluate(module, module.Definitions[len(module.Definitions)-1].Body, Context{Current: State{"x": one}})
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := int64(1); i <= 32; i++ {
		group.Go(func() {
			key, _ := value.Integer(i)
			got, err := function.Lookup(key)
			n, ok := got.Int()
			if err != nil || !ok || n != int32(i+2) {
				t.Errorf("lookup %d: got %s, %v", i, got, err)
			}
		})
	}
	group.Wait()
}

func TestPrimedOperatorArguments(t *testing.T) {
	for _, tc := range []struct{ expr, want string }{
		{"D(x)", "2"},
		{"E(x)", "2"},
		{"F(x)", "3"},
		{"D(x + 1)", "3"},
		{"G(x)", "4"},
		{"H(x)", "TRUE"},
		{"D(y)", "9"},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES x, y\nD(a) == a'\nE(a) == D(a)\nF(a) == D(a + 1)\nG(a) == (a + 2)'\nH(a) == (a = 2)'\nResult == " + tc.expr + "\n===="
			module, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			one, _ := value.Integer(1)
			two, _ := value.Integer(2)
			seven, _ := value.Integer(7)
			nine, _ := value.Integer(9)
			ctx := Context{Current: State{"x": one, "y": seven}, Next: State{"x": two, "y": nine}}
			got, err := Evaluate(module, module.Definitions[len(module.Definitions)-1].Body, ctx)
			if err != nil || got.String() != tc.want {
				t.Fatalf("got %s, %v; want %s", got, err, tc.want)
			}
		})
	}
}

func BenchmarkOperatorArgument(b *testing.B) {
	module, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nF(a) == a + a\nResult == F(x + 1)\n====")
	if err != nil {
		b.Fatal(err)
	}
	program := New(module)
	one, _ := value.Integer(1)
	context := Context{Current: State{"x": one}}
	expression := module.Definitions[len(module.Definitions)-1].Body
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := program.Evaluate(expression, context); err != nil {
			b.Fatal(err)
		}
	}
}
