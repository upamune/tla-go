package evaluator

import (
	"errors"
	"strings"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestDeferredCardinalityHugeLimit(t *testing.T) {
	p := &Program{}
	p.SetBound(int(^uint(0) >> 1))
	deferred, err := value.SetOperation("\\cup", value.Interval(1, 2), value.Interval(3, 4))
	if err != nil {
		t.Fatal(err)
	}
	if err := p.checkDeferredCardinality(deferred, 4); err != nil {
		t.Fatalf("small deferred set at largest bound: %v", err)
	}
}

func TestSetBoundOperations(t *testing.T) {
	m, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Naturals, Sequences, FiniteSets, Bags, TLC\n" +
		"Cup == (1..6) \\cup {7, 8, 9, 10, 11}\nCupAtLimit == (1..6) \\cup {6, 7, 8, 9, 10}\nCupDeferred == [{1} -> 1..6] \\cup [{1} -> 7..11]\n" +
		"Cap == (1..20) \\cap Nat\nDiff == (1..11) \\ {0}\nUnion == UNION {{1, 2, 3, 4, 5, 6}, {7, 8, 9, 10, 11}}\nUnionIntervals == UNION {1..6, 7..11}\n" +
		"Select == SelectSeq([i \\in 1..11 |-> i], LAMBDA e : TRUE)\nCombine == [i \\in 1..6 |-> i] @@ [i \\in 7..11 |-> i]\n" +
		"Bag == BagUnion({[i \\in 1..6 |-> 1], [i \\in 7..11 |-> 1]})\nMapped == BagOfAll(LAMBDA e : e, [i \\in 1..11 |-> 1])\n" +
		"Size == Cardinality(UNION {1..6, 7..11})\nPowerset == Cardinality(SUBSET (1..4))\n====")
	if err != nil {
		t.Fatal(err)
	}
	p := New(m)
	p.SetBound(10)
	method := map[string]bool{"Select": true, "Combine": true, "Bag": true, "Mapped": true, "Size": true}
	pass := map[string]bool{"CupAtLimit": true, "CupDeferred": true, "UnionIntervals": true, "Powerset": true}
	for _, definition := range m.Definitions {
		_, err := p.Evaluate(definition.Body, Context{})
		failure, _ := errors.AsType[*RuntimeError](err)
		switch {
		case pass[definition.Name]:
			if err != nil {
				t.Errorf("%s: %v", definition.Name, err)
			}
		case failure == nil || !strings.HasSuffix(failure.Message, "Attempted to construct a set with too many elements (>10)."):
			t.Errorf("%s: error %v", definition.Name, err)
		case method[definition.Name] != (failure.ToolCode == 2154) || method[definition.Name] == failure.WrongInvocation:
			t.Errorf("%s: failure %+v", definition.Name, failure)
		}
	}
}
