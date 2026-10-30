package checker

import (
	"errors"
	"testing"
)

func TestNameResolutionSemanticErrors(t *testing.T) {
	for _, tc := range []struct {
		name, definitions string
		semantic          bool
	}{
		{"unknown name", "Bad == Missing", true},
		{"too many arguments", "F(a) == a\nBad == F(1, 2)", true},
		{"too few arguments", "F(a, b) == a\nBad == F(1)", true},
		{"valid argument count", "F(a) == a\nGood == F(1)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\n" + tc.definitions + "\n====\n"
			_, err := Check(source, "")
			_, semantic := errors.AsType[*SemanticError](err)
			if semantic != tc.semantic || (!tc.semantic && err != nil) {
				t.Fatalf("error %v, want semantic error %t", err, tc.semantic)
			}
		})
	}
}
