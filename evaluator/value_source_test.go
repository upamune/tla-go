package evaluator

import (
	"errors"
	"sync"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestValueSourceEvaluationContext(t *testing.T) {
	module, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Naturals\nDirect == UNION {1}\nLazy == [n \\in Nat |-> UNION {1}]\n====")
	if err != nil {
		t.Fatal(err)
	}
	program := New(module)
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			without := i%2 == 0
			ctx := Context{WithoutValueSource: without}
			for _, definition := range module.Definitions {
				v, err := program.Evaluate(definition.Body, ctx)
				if err != nil {
					t.Errorf("%s: %v", definition.Name, err)
					continue
				}
				if definition.Name == "Lazy" {
					zero, _ := value.Integer(0)
					v, err = v.Lookup(zero)
					if err != nil {
						t.Errorf("lazy lookup: %v", err)
						continue
					}
				}
				_, err = v.Key()
				var fp *value.FingerprintError
				var first *value.UnionFirstElementError
				if !errors.As(err, &first) || errors.As(err, &fp) == without {
					t.Errorf("%s without source=%t: %v", definition.Name, without, err)
				}
			}
		})
	}
	wg.Wait()
}

func TestValueSourceContextPreservesStateAlias(t *testing.T) {
	module, err := parser.ParseModule("---- MODULE M ----\nVARIABLE x\nRead == x\n====")
	if err != nil {
		t.Fatal(err)
	}
	one, _ := value.Integer(1)
	base, _ := value.Set([]value.Value{one})
	source := &parser.Expression{Kind: "Apply", Text: "UNION"}
	union, err := value.UnionWithSource(base, source)
	if err != nil {
		t.Fatal(err)
	}
	v, err := New(module).Evaluate(module.Definitions[0].Body, Context{Current: State{"x": union}, WithoutValueSource: true})
	if err != nil || v != union {
		t.Fatalf("alias = %v, %v", v, err)
	}
	_, err = v.Key()
	var fp *value.FingerprintError
	if !errors.As(err, &fp) || len(fp.Sources) != 1 || fp.Sources[0] != source {
		t.Fatalf("alias error = %v", err)
	}
}
