package checker

import (
	"strings"
	"testing"
)

const operatorConfigSource = "---- MODULE Op ----\nEXTENDS Naturals\nCONSTANT F(_), G(_, _), C\nVARIABLE x\nInit == x = 0\nNext == x' = F(x)\nInv == x /= 2\nInc(n) == n + 1\nAdd(a,b) == a + b\n===="

func TestConfigOperatorAssignments(t *testing.T) {
	for _, tc := range []struct {
		name, constants string
		status          Status
		states          int
	}{
		{"plain value", "F = 1 G <- Add C = 1", Success, 2},
		{"operator table", "F(0) = 1 F(1) = 2 F(2) = 2 G <- Add C = 1", InvariantViolation, 3},
		{"substitution target value", "F <- Inc G <- Add C = 1 Inc = 2", InvariantViolation, 2},
		{"substitution beats table", "F(0) = 7 F <- Inc G <- Add C = 1", InvariantViolation, 3},
		{"table beats plain value", "F = 9 F(0) = 1 F(1) = 2 G <- Add C = 1", InvariantViolation, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Check(operatorConfigSource, "INIT Init NEXT Next INVARIANT Inv CONSTANT "+tc.constants)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != tc.status || result.DistinctStates != tc.states {
				t.Fatalf("got %+v", result)
			}
		})
	}
}

func TestConfigModelValueAndLazyOperator(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nCONSTANT F(_)\nVARIABLE x\nInit == x = F(1 \\div 0)\nNext == UNCHANGED x\nInv == x = F(0)\n===="
	result, err := Check(source, "INIT Init NEXT Next INVARIANT Inv CONSTANT F = F")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != Success || result.DistinctStates != 1 {
		t.Fatalf("got %+v", result)
	}
}

func TestConfigTableSupportsMultipleArgumentsAndModelValues(t *testing.T) {
	source := "---- MODULE M ----\nCONSTANT F(_, _), a\nVARIABLE x\nInit == x = F(a, {1, 2})\nNext == UNCHANGED x\nInv == x = 3\n===="
	result, err := Check(source, "INIT Init NEXT Next INVARIANT Inv CONSTANT a = a F(a, {2, 1}) = 3")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != Success || result.DistinctStates != 1 {
		t.Fatalf("got %+v", result)
	}
}

func TestConfigTableRejectsInconsistentArity(t *testing.T) {
	for _, constants := range []string{
		"F(0) = 1 F(1, 2) = 2 G <- Add C = 1",
		"F(0) = 1 F(1, 2) = 2 F <- Inc G <- Add C = 1",
		"Inc(0) = [Op] 1 Inc(1, 2) = [Op] 2 F <- Inc G <- Add C = 1",
	} {
		config := "INIT Init NEXT Next INVARIANT Inv CONSTANT " + constants
		if result, err := Check(operatorConfigSource, config); err == nil {
			t.Fatalf("unexpected result: %+v", result)
		}
	}
}

func TestConfigTableReportsUndefinedArguments(t *testing.T) {
	config := "INIT Init NEXT Next INVARIANT Inv CONSTANT F(0) = 1 G <- Add C = 1"
	result, err := Check(operatorConfigSource, config)
	want := "Attempted to apply operator:\n{ <0, 1>}\nto arguments (1), which is undefined."
	if err != nil || result.Status != EvaluationFailure || result.Failure == nil || !strings.Contains(result.Failure.Message, want) {
		t.Fatalf("got %+v, error %v, want %q", result, err, want)
	}
}
