package checker

import "testing"

func TestConfigSubstitutions(t *testing.T) {
	for _, tc := range []struct {
		name, source, config string
		status               Status
		states               int
	}{
		{
			name:   "constant",
			source: "---- MODULE M ----\nCONSTANT C\nVARIABLE x\nFoo == 1\nInit == x = C\nNext == UNCHANGED x\nInv == x = 1\n====",
			config: "CONSTANT C <- Foo\nINIT Init\nNEXT Next\nINVARIANT Inv",
			status: Success, states: 1,
		},
		{
			name:   "definition in specification",
			source: "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nMCInit == x = 1\nSpec == Init /\\ [][x' = x]_x\nInv == x = 1\n====",
			config: "CONSTANT Init <- MCInit\nSPECIFICATION Spec\nINVARIANT Inv",
			status: Success, states: 1,
		},
		{
			name:   "operator constants",
			source: "---- MODULE M ----\nEXTENDS Naturals\nCONSTANT F(_), G(_, _), C\nVARIABLE x\nInit == x = 0\nNext == x' = F(x)\nInv == x /= 2\nInc(n) == n + 1\nAdd(a,b) == a + b\n====",
			config: "INIT Init\nNEXT Next\nINVARIANT Inv\nCONSTANT F <- Inc G <- Add C = 1",
			status: InvariantViolation, states: 3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Check(tc.source, tc.config)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != tc.status || result.DistinctStates != tc.states {
				t.Fatalf("got %+v", result)
			}
		})
	}
}

func TestInvalidConfigSubstitutions(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nCONSTANT F(_), C\nVARIABLE x\nInc(n) == n + 1\nAdd(a,b) == a + b\nState == x\nInit == x = 0\nNext == UNCHANGED x\n===="
	for _, constants := range []string{
		"F <- Add C = 1", "F <- Missing C = 1", "F <- Inc C <- Inc",
		"F <- Inc C <- State", "F <- Inc C = 1 Missing <- Inc",
		"F <- Inc C = 1 Inc <- Add", "F <- [M] Inc C = 1", "F <- Inc F <- Inc C = 1",
	} {
		t.Run(constants, func(t *testing.T) {
			if result, err := Check(source, "INIT Init NEXT Next CONSTANT "+constants); err == nil {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestConfigSubstitutionCannotCreateDefinitionCycle(t *testing.T) {
	source := "---- MODULE M ----\nCONSTANT C\nVARIABLE x\nFoo == C\nInit == x = C\nNext == UNCHANGED x\n===="
	if result, err := Check(source, "INIT Init NEXT Next CONSTANT C <- Foo"); err == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestConfigSubstitutionDoesNotHideInvalidDefinition(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nBroken == Undefined\nReplacement == TRUE\nInit == x = 0\nNext == UNCHANGED x\n===="
	if result, err := Check(source, "INIT Init NEXT Next CONSTANT Broken <- Replacement"); err == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
}
