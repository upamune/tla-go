package checker

import (
	"context"
	"strings"
	"testing"
)

func TestInstanceValidation(t *testing.T) {
	module := "---- MODULE A ----\nCONSTANT C\nVARIABLE y\nLOCAL Hidden == y\nPublic == y\n===="
	loader := func(context.Context, string) (string, error) { return module, nil }
	for _, tt := range []struct{ name, body, contains string }{
		{"missing substitution", "I == INSTANCE A WITH C <- 1", "undefined symbol y"},
		{"unknown substitution", "I == INSTANCE A WITH C <- 1, y <- x, z <- 0", "invalid instance substitution"},
		{"duplicate substitution", "I == INSTANCE A WITH C <- 1, C <- 2, y <- x", "invalid instance substitution"},
		{"action variable", "I == INSTANCE A WITH C <- 1, y <- x'", "state-level"},
		{"state constant", "I == INSTANCE A WITH C <- x, y <- x", "constant-level"},
		{"private access", "I == INSTANCE A WITH C <- 1, y <- x\nF == I!Hidden", "undefined symbol"},
		{"duplicate name", "I == INSTANCE A WITH C <- 1, y <- x\nI == 1", "duplicate instance name"},
		{"early reference", "F == I!Public\nI == INSTANCE A WITH C <- 1, y <- x", "before its declaration"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := "---- MODULE M ----\nVARIABLE x\n" + tt.body + "\nInit == x = 0\nNext == x' = x\n===="
			_, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next", loader)
			if err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestInstanceConstantParameters(t *testing.T) {
	module := "---- MODULE A ----\nCONSTANT C\nVARIABLE y\nPublic == C = y\n===="
	loader := func(context.Context, string) (string, error) { return module, nil }
	for _, tt := range []struct {
		name, expression string
		allowed          bool
	}{
		{"literal", "P(0)!Public", true},
		{"constant quantified", "\\A i \\in {0,1} : P(i)!Public \\/ ~P(i)!Public", true},
		{"state argument", "P(x)!Public", false},
		{"state quantified", "\\A i \\in {x} : P(i)!Public", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := "---- MODULE M ----\nVARIABLE x\nP(p) == INSTANCE A WITH C <- p, y <- x\nInit == x = 0\nNext == x' = x\nInv == " + tt.expression + "\n===="
			_, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv", loader)
			if tt.allowed && err != nil {
				t.Fatal(err)
			}
			if !tt.allowed && (err == nil || !strings.Contains(err.Error(), "constant argument")) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestInstanceConstantParameterCheckedInFunctionDomain(t *testing.T) {
	module := "---- MODULE A ----\nCONSTANT C\nVARIABLE y\nPublic == C\n===="
	loader := func(context.Context, string) (string, error) { return module, nil }
	source := "---- MODULE M ----\nVARIABLE x\nP(p) == INSTANCE A WITH C <- p, y <- x\nf[i \\in {P(x)!Public}] == i\nInit == x = 0\nNext == x' = x\n===="
	_, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next", loader)
	if err == nil || !strings.Contains(err.Error(), "constant argument") {
		t.Fatalf("got %v", err)
	}
}

func TestCyclicInstanceModulesAreRejected(t *testing.T) {
	modules := map[string]string{
		"A": "---- MODULE A ----\nI == INSTANCE B\nValue == I!Value\n====",
		"B": "---- MODULE B ----\nI == INSTANCE A\nValue == I!Value\n====",
	}
	loader := func(_ context.Context, name string) (string, error) { return modules[name], nil }
	source := "---- MODULE M ----\nVARIABLE x\nI == INSTANCE A\nInit == x = 0\nNext == x' = x\n===="
	_, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next", loader)
	if err == nil || !strings.Contains(err.Error(), "cyclic instance") {
		t.Fatalf("got %v", err)
	}
}

func TestNestedModuleCannotSeeLaterParentDefinition(t *testing.T) {
	source := "---- MODULE M ----\n---- MODULE Inner ----\nValue == Later\n====\nLater == 1\nVARIABLE x\nI == INSTANCE Inner\nInit == x = 0\nNext == x' = x\n===="
	_, err := Check(source, "INIT Init NEXT Next")
	if err == nil || !strings.Contains(err.Error(), "undefined symbol Later") {
		t.Fatalf("got %v", err)
	}
}

func TestNestedLocalInstanceIsNotExported(t *testing.T) {
	module := "---- MODULE A ----\n---- MODULE Inner ----\nValue == 1\n====\nLOCAL J == INSTANCE Inner\nVisible == J!Value\n===="
	loader := func(context.Context, string) (string, error) { return module, nil }
	source := "---- MODULE M ----\nVARIABLE x\nI == INSTANCE A\nInit == x = I!J!Value\nNext == x' = x\n===="
	_, err := CheckWithLoader(context.Background(), source, "INIT Init NEXT Next", loader)
	if err == nil || !strings.Contains(err.Error(), "undefined symbol") {
		t.Fatalf("got %v", err)
	}
}

func TestNestedModuleCannotReuseAnEarlierExtendedName(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Sequences\n---- MODULE Sequences ----\nMine == 1\n====\nVARIABLE x\nInit == x = 0\nNext == x' = x\n===="
	_, err := Check(source, "INIT Init NEXT Next")
	if err == nil || !strings.Contains(err.Error(), "duplicate nested module Sequences") {
		t.Fatalf("got %v", err)
	}
}
