package checker

import (
	"context"
	"runtime/debug"
	"testing"
)

// A function cache key identifies a LAMBDA argument by its scope's address,
// so the bound frame of each set-map element must not be reused while the
// cache lives. Java TLC 1.7.4 also reports success with one distinct state.
func TestLambdaFunctionCacheKeepsBoundFrames(t *testing.T) {
	module := "---- MODULE A ----\nEXTENDS Naturals\nCONSTANT Op(_)\nf[x \\in 1..2] == Op(x)\n===="
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE v\nI(Op(_)) == INSTANCE A\nInit == v = 0\nNext == v' = v\n" +
		"Fs == {I(LAMBDA y : y)!f} \\cup {I(LAMBDA y : y + k)!f : k \\in 1..3}\nInv == (CHOOSE g \\in Fs : g[1] = 4)[2] = 5\n===="
	loader := func(context.Context, string) (string, error) { return module, nil }
	result, err := CheckWithLoader(context.Background(), source, "INIT Init\nNEXT Next\nINVARIANT Inv", loader)
	if err != nil || result.Status != Success || result.GeneratedStates != 2 || result.DistinctStates != 1 {
		t.Fatalf("got %+v, %v", result, err)
	}
}

// Comparing h with itself creates the function cache before the quantifier,
// so every k shares it. The cache must keep each LAMBDA scope reachable:
// otherwise a later scope can take a collected one's address and hit its
// entry. Frequent collections make such reuse likely. Java TLC 1.7.4 also
// reports success with one distinct state.
func TestLambdaFunctionCacheKeepsCollectedScopes(t *testing.T) {
	defer debug.SetGCPercent(debug.SetGCPercent(1))
	module := "---- MODULE A ----\nEXTENDS Naturals\nCONSTANT Op(_)\nf[x \\in 1..2] == Op(x)\n===="
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE v\nI(Op(_)) == INSTANCE A\nInit == v = 0\nNext == v' = v\n" +
		"Inv == LET h[n \\in 1..2] == n + v IN h = h /\\ \\A k \\in 1..2000 : I(LAMBDA y : y + k)!f = [x \\in 1..2 |-> x + k]\n===="
	loader := func(context.Context, string) (string, error) { return module, nil }
	result, err := CheckWithLoader(context.Background(), source, "INIT Init\nNEXT Next\nINVARIANT Inv", loader)
	if err != nil || result.Status != Success || result.GeneratedStates != 2 || result.DistinctStates != 1 {
		t.Fatalf("got %+v, %v", result, err)
	}
}
