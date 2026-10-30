package checker

import (
	"context"
	"errors"
	"testing"
)

func TestModuleProofDirectiveWithExtensions(t *testing.T) {
	cases := []struct {
		name, dependency, directive string
		valid                       bool
	}{
		{"imported definition", "Foo == TRUE\nUSE DEF Foo", "USE DEF Foo", true},
		{"private definition", "LOCAL Foo == TRUE\nUSE DEF Foo", "", true},
		{"imported theorem", "THEOREM T == TRUE", "HIDE T", true},
		{"extension forward definition", "USE DEF Foo\nFoo == TRUE", "", false},
		{"root forward definition", "Foo == TRUE", "USE DEF Later\nLater == TRUE", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS A\n" + tc.directive + "\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n===="
			loader := func(_ context.Context, name string) (string, error) {
				if name != "A" {
					return "", errors.New("unknown module")
				}
				return "---- MODULE A ----\n" + tc.dependency + "\n====", nil
			}
			result, err := CheckWithLoader(context.Background(), source, "INIT Init\nNEXT Next\n", loader)
			if tc.valid && (err != nil || result.Status != Success || result.DistinctStates != 1) {
				t.Fatalf("result %+v, error %v", result, err)
			}
			if !tc.valid && err == nil {
				t.Fatalf("unexpected success: %+v", result)
			}
		})
	}
}
