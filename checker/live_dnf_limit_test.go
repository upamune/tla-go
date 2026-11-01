package checker

import "testing"

func TestLiveDNFWithinLimit(t *testing.T) {
	leaf := &liveNode{kind: liveState}
	choice := &liveNode{kind: liveDisj, items: []*liveNode{leaf, leaf}}
	formula := &liveNode{kind: liveConj}
	for range 20 {
		formula.items = append(formula.items, choice)
	}
	if liveDNFWithinLimit(formula, 1024) {
		t.Fatal("accepted an exponential expansion")
	}
	if !liveDNFWithinLimit(formula, 1<<20) {
		t.Fatal("rejected a formula at the limit")
	}
	negated := &liveNode{kind: liveNeg, body: formula}
	if !liveDNFWithinLimit(negated, 40) {
		t.Fatal("did not account for negation before distribution")
	}
}
