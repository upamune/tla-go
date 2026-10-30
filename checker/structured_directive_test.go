package checker

import (
	"context"
	"fmt"
	"testing"
)

func TestStructuredProofDirectives(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"use definition", "F == TRUE\nTHEOREM T == TRUE\n<1> USE DEF F\n<1> QED", true},
		{"hide definition", "F == TRUE\nTHEOREM T == TRUE\n<1> HIDE DEF F\n<1> QED", true},
		{"local definition", "THEOREM T == TRUE\n<1> DEFINE F == TRUE\n<1> USE DEF F\n<1> HIDE DEF F\n<1> QED", true},
		{"use formula", "THEOREM T == TRUE\n<1> USE TRUE\n<1> QED", true},
		{"hide formula", "THEOREM T == TRUE\n<1> HIDE TRUE\n<1> QED", false},
		{"prior step", "THEOREM T == TRUE\n<1>1 TRUE\n<1> USE <1>1\n<1> HIDE <1>1\n<1> QED", true},
		{"hidden step explicit use", "THEOREM T == TRUE\n<1>1 TRUE\n<1> HIDE <1>1\n<1>2 TRUE BY <1>1\n<1> QED", true},
		{"hide theorem", "THEOREM U == TRUE\nTHEOREM T == TRUE\n<1> HIDE U\n<1> QED", true},
		{"hide assumption", "ASSUME A == TRUE\nTHEOREM T == TRUE\n<1> HIDE A\n<1> QED", true},
		{"forward step", "THEOREM T == TRUE\n<1> USE <1>1\n<1>1 TRUE\n<1> QED", false},
		{"forward definition", "THEOREM T == TRUE\n<1> USE DEF F\n<1> QED\nF == TRUE", false},
		{"use only", "F == TRUE\nTHEOREM T == TRUE\n<1> USE ONLY DEF F\n<1> QED", true},
		{"hide only", "F == TRUE\nTHEOREM T == TRUE\n<1> HIDE ONLY DEF F\n<1> QED", false},
		{"named directive", "F == TRUE\nTHEOREM T == TRUE\n<1>1 USE DEF F\n<1> QED", true},
		{"directive not a fact", "F == TRUE\nTHEOREM T == TRUE\n<1>1 USE DEF F\n<1>2 TRUE BY <1>1\n<1> QED", false},
		{"directive has proof", "F == TRUE\nTHEOREM T == TRUE\n<1> USE DEF F BY TRUE\n<1> QED", false},
		{"empty directive", "THEOREM T == TRUE\n<1> USE\n<1> QED", false},
		{"hide definition as fact", "F == TRUE\nTHEOREM T == TRUE\n<1> HIDE F\n<1> QED", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CheckContext(context.Background(), "---- MODULE M ----\n"+tc.body+"\n====\n", "")
			if (err == nil) != tc.valid {
				t.Fatalf("valid %t: %v", tc.valid, err)
			}
		})
	}
}

func TestProofDirectiveModuleVisibility(t *testing.T) {
	for _, tc := range []struct {
		name, header, directive string
		valid                   bool
	}{
		{"self", "", "USE MODULE M", true},
		{"missing", "", "USE MODULE Missing", false},
		{"unimported standard", "", "HIDE MODULE Naturals", false},
		{"direct standard", "EXTENDS Naturals", "HIDE MODULE Naturals", true},
		{"standard dependency", "EXTENDS TLC", "USE MODULE Sequences", true},
		{"extension", "EXTENDS A", "HIDE MODULE A", true},
		{"transitive extension", "EXTENDS A", "USE MODULE B", true},
		{"instance", "I == INSTANCE A", "HIDE MODULE A", true},
		{"transitive instance", "I == INSTANCE A", "USE MODULE B", true},
		{"later instance", "", "USE MODULE A", true},
		{"let instance", "X == LET I == INSTANCE A IN TRUE", "USE MODULE B", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loader := func(_ context.Context, name string) (string, error) {
				switch name {
				case "A":
					return "---- MODULE A ----\nEXTENDS B\nC == TRUE\n====\n", nil
				case "B":
					return "---- MODULE B ----\nD == TRUE\n====\n", nil
				default:
					return "", fmt.Errorf("missing module %s", name)
				}
			}
			post := ""
			if tc.name == "later instance" {
				post = "I == INSTANCE A\n"
			}
			source := "---- MODULE M ----\n" + tc.header + "\nTHEOREM T == TRUE\n<1> " + tc.directive + "\n<1> QED\n" + post + "====\n"
			_, err := CheckWithLoader(context.Background(), source, "", loader)
			if (err == nil) != tc.valid {
				t.Fatalf("valid %t: %v", tc.valid, err)
			}
		})
	}
}

func TestModuleProofDirectiveModuleFacts(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"use self", "USE MODULE M", true},
		{"hide self", "HIDE MODULE M", true},
		{"use missing", "USE MODULE Missing", false},
		{"hide missing", "HIDE MODULE Missing", false},
		{"unimported standard", "USE MODULE Naturals", false},
		{"use imported", "EXTENDS Naturals\nUSE MODULE Naturals", true},
		{"hide imported", "EXTENDS Naturals\nHIDE MODULE Naturals", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CheckContext(context.Background(), "---- MODULE M ----\n"+tc.body+"\n====\n", "")
			if (err == nil) != tc.valid {
				t.Fatalf("valid %t: %v", tc.valid, err)
			}
		})
	}
}
