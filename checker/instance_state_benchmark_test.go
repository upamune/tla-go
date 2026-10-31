package checker

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkParameterizedInstanceState(b *testing.B) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nI(z) == INSTANCE A WITH y <- z\nInit == x = 0\nNext == x' = (x + 1) % 128\nInv == I(x)!Nonnegative\n====\n"
	const dependency = "---- MODULE A ----\nEXTENDS Naturals\nVARIABLE y\nNonnegative == y >= 0\n====\n"
	loader := func(_ context.Context, name string) (string, error) {
		if name != "A" {
			return "", fmt.Errorf("unexpected module %s", name)
		}
		return dependency, nil
	}
	b.ReportAllocs()
	for b.Loop() {
		result, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv", loader)
		if err != nil || result.Status != Success || result.DistinctStates != 128 {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}

func BenchmarkParameterizedInstanceConstant(b *testing.B) {
	const source = "---- MODULE M ----\nI(p) == INSTANCE A WITH C <- p\nASSUME I(1)!Valid\n====\n"
	const dependency = "---- MODULE A ----\nCONSTANT C\nValid == C = 1\n====\n"
	loader := func(_ context.Context, name string) (string, error) {
		if name != "A" {
			return "", fmt.Errorf("unexpected module %s", name)
		}
		return dependency, nil
	}
	b.ReportAllocs()
	for b.Loop() {
		result, err := CheckWithLoader(context.Background(), source, "", loader)
		if err != nil || result.Status != Success {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}

// A LAMBDA bound by a quantifier or by an action's \E gives each call of a
// parameterized instance's function definition a new operator scope.
func BenchmarkParameterizedInstanceOperatorFunction(b *testing.B) {
	const dependency = "---- MODULE A ----\nEXTENDS Naturals\nCONSTANT Op(_)\nf[x \\in 1..2] == Op(x)\n====\n"
	loader := func(_ context.Context, name string) (string, error) {
		if name != "A" {
			return "", fmt.Errorf("unexpected module %s", name)
		}
		return dependency, nil
	}
	const header = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES v, w\nI(Op(_)) == INSTANCE A\nInit == v = 0 /\\ w = 0\n"
	cases := []struct {
		name, definitions, config string
		distinct                  int
	}{
		{"quantifier", "Next == v' = (v + 1) % 1000 /\\ UNCHANGED w\nInv == \\A k \\in 1..3 : I(LAMBDA y : y + k + v)!f[1] = 1 + k + v\n", "INIT Init NEXT Next INVARIANT Inv", 1000},
		{"action", "Next == \\E k \\in 1..2 : v' = (v + k) % 1000 /\\ w' = I(LAMBDA y : y + k)!f[1]\n", "INIT Init NEXT Next", 2001},
	}
	for _, c := range cases {
		source := header + c.definitions + "====\n"
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				result, err := CheckWithLoader(context.Background(), source, c.config, loader)
				if err != nil || result.Status != Success || result.DistinctStates != c.distinct {
					b.Fatalf("got %+v, %v", result, err)
				}
			}
		})
	}
}
