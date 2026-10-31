package checker

import (
	"context"
	"reflect"
	"testing"
)

// A symmetry set is named by the constant its definition overrides, as in
// Java TLC's Tool.getSymmetryPerms, and an EXCEPT outside the domain warns
// at the location of the function.
func TestModelWarnings(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nCONSTANTS A, B, a, b, c\nVARIABLE f\nSmall == {a}\n" +
		"Sym == Permutations(Small) \\cup Permutations(B)\nInit == f = <<0>>\nNext == f[1] < 2 /\\ f' = [f EXCEPT ![2] = 0, ![1] = @ + 1]\n===="
	cfg := "CONSTANTS B = {b, c} a = a b = b c = c\nCONSTANT A <- Small\nINIT Init\nNEXT Next\nSYMMETRY Sym\nCHECK_DEADLOCK FALSE\n"
	var got []Warning
	options := Options{OnWarning: func(warning Warning) { got = append(got, warning) }}
	result, err := CheckWithOptions(context.Background(), source, cfg, options)
	want := []Warning{
		{Code: 2300, Message: "The set A has been defined to be a symmetry set but contains less than two elements."},
		{Code: 2144, Message: "The EXCEPT was applied to non-existing fields of the value at\nline 8, col 27 to line 8, col 27 of module M"},
	}
	if err != nil || result.Status != Success || len(got) < 2 || !reflect.DeepEqual(got[:2], want) {
		t.Fatalf("warnings %+v, result %+v, error %v", got, result, err)
	}
}
