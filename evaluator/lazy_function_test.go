package evaluator

import (
	"sync"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestLazyFunctionCapturesState(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nf[n \\in Nat] == n + x\nResult == f\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	two, _ := value.Integer(2)
	three, _ := value.Integer(3)
	state := State{"x": two}
	f, err := Evaluate(module, module.Definitions[1].Body, Context{Current: state})
	if err != nil {
		t.Fatal(err)
	}
	state["x"] = three
	got, err := f.Lookup(three)
	if err != nil || got.String() != "5" {
		t.Fatalf("got %s, %v", got, err)
	}
}

func TestLazyFunctionSurvivesSubsequentEvaluations(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nf[n \\in Nat] == n + x\nResult == f\nCurrent == x\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	program := New(module)
	two, _ := value.Integer(2)
	three, _ := value.Integer(3)
	f, err := program.Evaluate(module.Definitions[1].Body, Context{Current: State{"x": two}})
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		if _, err := program.Evaluate(module.Definitions[2].Body, Context{Current: State{"x": three}}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := f.Lookup(three)
	if err != nil || got.String() != "5" {
		t.Fatalf("got %s, %v; want 5", got, err)
	}
}

func TestConcurrentLazyFunctionsRetainTheirStates(t *testing.T) {
	module, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nf[n \\in Nat] == n + x\nResult == f\n====")
	if err != nil {
		t.Fatal(err)
	}
	program := New(module)
	var workers sync.WaitGroup
	for i := range 16 {
		workers.Go(func() {
			x, _ := value.Integer(int64(i))
			for range 20 {
				f, err := program.Evaluate(module.Definitions[1].Body, Context{Current: State{"x": x}})
				if err != nil {
					t.Error(err)
					return
				}
				got, err := f.Lookup(x)
				want, _ := value.Integer(int64(2 * i))
				equal, cmpErr := value.Equal(got, want)
				if err != nil || cmpErr != nil || !equal {
					t.Errorf("got %v, %v; comparison %v; want %v", got, err, cmpErr, want)
					return
				}
			}
		})
	}
	workers.Wait()
}

func TestNonTerminatingFunctionReturnsError(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nf[n \\in Nat] == f[n]\nResult == f[0]\n===="
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Evaluate(module, module.Definitions[1].Body, Context{}); err == nil {
		t.Fatal("nonterminating function accepted")
	}
}

func TestLazyFunctionErrorPropagation(t *testing.T) {
	for _, expression := range []string{"[n \\in Nat |-> n][(-1)]", "[n \\in {0,1} |-> 1 \\div n][0]"} {
		module, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Integers\nResult == " + expression + "\n====")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Evaluate(module, module.Definitions[0].Body, Context{}); err == nil {
			t.Errorf("accepted %s", expression)
		}
	}
}
