package evaluator

import (
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
