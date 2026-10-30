package evaluator

import (
	"strings"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func TestTLCOperatorErrorsAndModuleScope(t *testing.T) {
	for _, tc := range []struct{ modules, expr string }{
		{"Integers, TLC", `Assert(FALSE, "failed")`},
		{"Integers, TLC", `Assert(TRUE, 1 \div 0)`},
		{"Integers, TLC", `1 @@ 2`},
		{"Integers, TLC", `Permutations(1)`},
		{"Integers", `ToString(1)`},
	} {
		module, err := parser.ParseModule("---- MODULE M ----\nEXTENDS " + tc.modules + "\nResult == " + tc.expr + "\n====")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Evaluate(module, module.Definitions[0].Body, Context{}); err == nil {
			t.Errorf("accepted %s with %s", tc.expr, tc.modules)
		}
	}
}

func TestTLCAssertEvaluatesMessage(t *testing.T) {
	module, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Integers, TLC\nResult == Assert(TRUE, 1 \\div 0)\n====")
	if err != nil {
		t.Fatal(err)
	}
	_, err = Evaluate(module, module.Definitions[0].Body, Context{})
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Fatalf("got %v", err)
	}
}
