package checker

import (
	"context"
	"testing"
)

func BenchmarkParameterizedOperatorFunctionAction(b *testing.B) {
	const module = "---- MODULE A ----\nEXTENDS Naturals\nCONSTANT F(_)\nVARIABLE v\nStep[i \\in {1}] == v' = IF v < 4999 THEN F(v) ELSE v\n====\n"
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInc(n) == n + 1\nP(Op(_)) == INSTANCE A WITH F <- Op, v <- x\nInit == x = 0\nNext == P(Inc)!Step[1]\n====\n"
	const config = "INIT Init\nNEXT Next\n"
	loader := func(context.Context, string) (string, error) { return module, nil }
	for b.Loop() {
		result, err := CheckWithLoader(context.Background(), source, config, loader)
		if err != nil || result.Status != Success || result.GeneratedStates != 5001 || result.DistinctStates != 5000 {
			b.Fatalf("unexpected result: %+v, %v", result, err)
		}
	}
}
