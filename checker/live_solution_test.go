package checker

import (
	"reflect"
	"testing"
)

func TestLiveSolutionsClassifyFairness(t *testing.T) {
	const source = "---- MODULE M ----\nVARIABLE x\nInit == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=0\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == []<>(x=1)\n===="
	model := prepareSimulationTest(t, source, "SPECIFICATION Spec PROPERTY P")
	solutions, err := model.liveSolutions()
	if err != nil || len(solutions) != 1 {
		t.Fatalf("got %d solutions, %v; want one", len(solutions), err)
	}
	s := solutions[0]
	if s.tableau != nil || len(s.checkState) != 0 || len(s.checkAction) != 2 {
		t.Fatalf("got tableau %v, %d state and %d action checks", s.tableau != nil, len(s.checkState), len(s.checkAction))
	}
	// WF_x(Next) becomes []<>(~ENABLED <<Next>>_x \/ <<Next>>_x); the negated
	// property becomes <>[]~(x=1).
	want := []livePEM{{aeAction: []int{0}, aeState: []int{}, eaAction: []int{1}}}
	if !reflect.DeepEqual(s.pems, want) {
		t.Fatalf("got possible error models %+v; want %+v", s.pems, want)
	}
}

func TestLiveSolutionsTableau(t *testing.T) {
	for _, tc := range []struct {
		property                 string
		nodes, initial, promises int
	}{
		// []~(x=1) has a single particle that is its own successor.
		{"<>(x=1)", 1, 1, 0},
		// <>((x=0) /\ []~(x=1)) postpones or fulfills its promise.
		{"(x=0) ~> (x=1)", 3, 2, 1},
	} {
		t.Run(tc.property, func(t *testing.T) {
			source := "---- MODULE M ----\nVARIABLE x\nInit == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=0\nP == " + tc.property + "\n===="
			model := prepareSimulationTest(t, source, "INIT Init NEXT Next PROPERTY P")
			solutions, err := model.liveSolutions()
			if err != nil || len(solutions) != 1 || solutions[0].tableau == nil {
				t.Fatalf("got %+v, %v; want one tableau solution", solutions, err)
			}
			s := solutions[0]
			if len(s.tableau.nodes) != tc.nodes || s.tableau.initial != tc.initial || len(s.promises) != tc.promises {
				t.Fatalf("got %d nodes, %d initial and %d promises", len(s.tableau.nodes), s.tableau.initial, len(s.promises))
			}
		})
	}
}

func TestLiveDistributeOrder(t *testing.T) {
	leaf := func(tag int) *liveNode { return &liveNode{kind: liveState, leaf: &liveLeaf{tag: tag}} }
	// (1 \/ 2) /\ 3 /\ (4 \/ 5) expands with the first disjunction varying slowest.
	conjunction := liveJunction(liveConj, liveJunction(liveDisj, leaf(1), leaf(2)), leaf(3), liveJunction(liveDisj, leaf(4), leaf(5)))
	dnf := conjunction.toDNF()
	var got [][]int
	for _, item := range dnf.items {
		var tags []int
		for _, part := range item.items {
			tags = append(tags, part.leaf.tag)
		}
		got = append(got, tags)
	}
	want := [][]int{{1, 3, 4}, {1, 3, 5}, {2, 3, 4}, {2, 3, 5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v; want %v", got, want)
	}
}

func TestLiveDisjunctionMergesRecurrence(t *testing.T) {
	leaf := func(tag int) *liveNode { return &liveNode{kind: liveState, leaf: &liveLeaf{tag: tag}} }
	often := func(tag int) *liveNode { return liveUnary(liveAll, liveUnary(liveEven, leaf(tag))) }
	dnf := liveJunction(liveDisj, often(1), leaf(2), often(3)).toDNF()
	if len(dnf.items) != 2 || dnf.items[0].leaf.tag != 2 {
		t.Fatalf("got %d disjuncts; want the state predicate and one recurrence", len(dnf.items))
	}
	body := dnf.items[1].aeBody()
	if body == nil || body.kind != liveDisj || len(body.items) != 2 || body.items[0].leaf.tag != 1 || body.items[1].leaf.tag != 3 {
		t.Fatalf("got recurrence %+v; want []<>(1 \\/ 3)", dnf.items[1])
	}
}
