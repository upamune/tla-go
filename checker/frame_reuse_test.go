package checker

import (
	"context"
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
