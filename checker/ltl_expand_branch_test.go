package checker

import (
	"context"
	"reflect"
	"testing"
)

func TestLTLBranchIDSetClone(t *testing.T) {
	var original ltlIDSet
	for _, id := range []int{-1, 0, 1, 63, 64, 2048} {
		original.add(id)
	}
	clone := original.clone()
	clone.add(2)
	clone.add(65)
	for _, id := range []int{-1, 0, 1, 63, 64, 2048} {
		if !original.has(id) || !clone.has(id) {
			t.Fatalf("missing id %d after cloning", id)
		}
	}
	if original.has(2) || original.has(65) {
		t.Fatal("mutating the clone changed the original")
	}
}

func TestLTLBranchAlternativesWithLargeIDs(t *testing.T) {
	a := &ltlFormula{id: 65, kind: ltlAtom}
	b := &ltlFormula{id: 66, kind: ltlAtom}
	c := &ltlFormula{id: 127, kind: ltlAtom}
	always := &ltlFormula{id: 2, kind: ltlAlways, left: a}
	eventually := &ltlFormula{id: 70, kind: ltlEventually, left: b}
	choice := &ltlFormula{id: 4, kind: ltlOr, left: eventually, right: c}
	root := &ltlFormula{id: 5, kind: ltlAnd, left: always, right: choice}
	expander := ltlExpander{model: &model{ctx: context.Background()}, tableau: true, eventuality: []*ltlFormula{eventually}}
	got, err := expander.alternatives([]*ltlFormula{root})
	if err != nil {
		t.Fatal(err)
	}
	want := []ltlAlternative{
		{next: []*ltlFormula{always}, accepting: []bool{true}},
		{next: []*ltlFormula{always, eventually}, accepting: []bool{false}},
		{next: []*ltlFormula{always}, accepting: []bool{true}},
	}
	conditions := [][]*ltlFormula{{a, b}, {a}, {a, c}}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(expander.tableauConditions, conditions) {
		t.Fatalf("alternatives = %#v, conditions = %#v", got, expander.tableauConditions)
	}
}
