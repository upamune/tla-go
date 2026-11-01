package checker

import (
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestJava17TableauClassificationSkipsLazyConstants(t *testing.T) {
	calls := 0
	lazy, err := value.LazyFunction(value.Interval(1, 2), func(value.Value, value.Value) (value.Value, error) {
		calls++
		return value.Bool(true), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	s := &search{model: &model{constants: map[string]value.Value{"F": lazy}}}
	name := &parser.Expression{Kind: "Identifier", Text: "F"}
	expr := &parser.Expression{Kind: "Apply", Text: "=", Args: []*parser.Expression{name, name}}
	if !s.pureConstantAtom(expr, nil) {
		t.Fatal("test expression is not a candidate constant atom")
	}
	if s.scalarConstantOperands(expr, nil) || calls != 0 {
		t.Fatalf("lazy constant was accepted or evaluated: calls=%d", calls)
	}
}
