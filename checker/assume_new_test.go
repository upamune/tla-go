package checker

import (
	"context"
	"fmt"
	"testing"
)

func TestTheoremNewDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name, module string
		valid        bool
	}{
		{"constant", "THEOREM T == ASSUME NEW a, a = a PROVE a = a", true},
		{"unnamed theorem", "THEOREM ASSUME NEW a PROVE a = a", true},
		{"state", "THEOREM T == ASSUME NEW VARIABLE a PROVE a = a", true},
		{"operator", "THEOREM T == ASSUME NEW F(_), F(1) = F(1) PROVE TRUE", true},
		{"infix operator", "THEOREM T == ASSUME NEW _ + _ PROVE 1 + 2 = 1 + 2", true},
		{"bounded", "THEOREM T == ASSUME NEW a \\in {1}, a = 1 PROVE a = 1", true},
		{"dependent bound", "THEOREM T == ASSUME NEW a, NEW b \\in {a} PROVE b = a", true},
		{"later module symbol", "THEOREM T == ASSUME NEW a PROVE a = a\na == 1", true},
		{"duplicate proof declaration", "THEOREM T == ASSUME NEW a, NEW a PROVE a = a", true},
		{"undefined before declaration", "THEOREM T == ASSUME a = a, NEW a PROVE TRUE", false},
		{"undefined outside theorem", "THEOREM T == ASSUME NEW a PROVE a = a\nB == a", false},
		{"earlier module symbol", "a == 1\nTHEOREM T == ASSUME NEW a PROVE a = a", false},
		{"standard name", "EXTENDS Naturals\nTHEOREM T == ASSUME NEW Nat PROVE TRUE", false},
		{"builtin operator", "THEOREM T == ASSUME NEW _ = _ PROVE TRUE", false},
		{"bad operator arity", "THEOREM T == ASSUME NEW F(_) PROVE F(1, 2) = 1", false},
		{"self reference in bound", "THEOREM T == ASSUME NEW a \\in {a} PROVE TRUE\na == 1", false},
		{"bounded variable", "THEOREM T == ASSUME NEW VARIABLE a \\in {1} PROVE TRUE", false},
		{"bounded operator", "THEOREM T == ASSUME NEW F(_) \\in {1} PROVE TRUE", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\n" + tc.module + "\n====\n"
			_, err := CheckContext(context.Background(), source, "")
			if (err == nil) != tc.valid {
				t.Fatalf("valid %t: %v", tc.valid, err)
			}
		})
	}
}

func TestTheoremNewScopeInSimpleProof(t *testing.T) {
	for _, tc := range []struct {
		name, module string
		valid        bool
	}{
		{"local fact", "THEOREM T == ASSUME NEW a PROVE a = a BY a = a", true},
		{"unknown fact", "THEOREM T == ASSUME NEW a PROVE a = a BY b = b", false},
		{"local is not definition", "THEOREM T == ASSUME NEW a PROVE a = a BY DEF a", false},
		{"forward definition", "THEOREM T == ASSUME NEW a PROVE a = a BY DEF F\nF == TRUE", false},
		{"previous definition", "F == TRUE\nTHEOREM T == ASSUME NEW a PROVE a = a BY DEF F", true},
		{"unnamed local fact", "THEOREM ASSUME NEW a PROVE a = a BY a = a", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CheckContext(context.Background(), "---- MODULE M ----\n"+tc.module+"\n====\n", "")
			if (err == nil) != tc.valid {
				t.Fatalf("valid %t: %v", tc.valid, err)
			}
		})
	}
}

func TestTheoremProofWithImportedAndLocalDefinitions(t *testing.T) {
	loader := func(_ context.Context, name string) (string, error) {
		if name != "A" {
			return "", fmt.Errorf("unknown module %s", name)
		}
		return "---- MODULE A ----\n\n\n\n\nImported == TRUE\n====\n", nil
	}
	for _, theorem := range []string{
		"THEOREM T == ASSUME NEW a PROVE a = a BY DEF Imported, Local",
		"THEOREM ASSUME NEW a PROVE a = a BY DEF Imported, Local",
	} {
		source := "---- MODULE M ----\nEXTENDS A\nLocal == TRUE\n" + theorem + "\n====\n"
		if _, err := CheckWithLoader(context.Background(), source, "", loader); err != nil {
			t.Fatalf("%s: %v", theorem, err)
		}
	}
}
