package checker

import (
	"context"
	"reflect"
	"testing"
)

func TestLivenessWarnings(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS TLC\nCONSTANTS P, a, b\nVARIABLE x\nInit == x \\in P\nNext == x' \\in P\n" +
		"Sym == Permutations(P)\nCon == x \\in P\nProp == <>(x = a)\nASSUME FALSE\n===="
	cfg := "CONSTANTS P = {a, b} a = a b = b\nINIT Init\nNEXT Next\nCONSTRAINT Con\nPROPERTY Prop\n"
	for _, tc := range []struct {
		name, cfg  string
		simulation *SimulationOptions
		want       []int
	}{
		{"model checking", cfg + "SYMMETRY Sym\n", nil, []int{2279, 2284}},
		{"simulation", cfg, &SimulationOptions{Traces: 1, Depth: 1}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []int
			options := Options{Simulation: tc.simulation, OnWarning: func(warning Warning) { got = append(got, warning.Code) }}
			result, err := CheckWithOptions(context.Background(), source, tc.cfg, options)
			// Java TLC warns before it reports the false assumption.
			if err != nil || result.Status != AssumptionViolation || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("warnings %v, result %+v, error %v; want %v", got, result, err, tc.want)
			}
		})
	}
}
