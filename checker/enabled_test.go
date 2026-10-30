package checker

import (
	"context"
	"strings"
	"testing"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

func TestEnabledRequiresAssignedCurrentState(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nInit == /\\ ENABLED (x' = 1) /\\ x = 0\nNext == UNCHANGED x\n===="
	_, err := Check(source, "INIT Init NEXT Next")
	if err == nil || !strings.Contains(err.Error(), "complete state") {
		t.Fatalf("got %v", err)
	}
}

func TestEnabledRejectsTemporalActionsEvenWithoutStates(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nInit == x \\in {}\nNext == UNCHANGED x\nInv == ENABLED []TRUE\n===="
	_, err := Check(source, "INIT Init NEXT Next INVARIANT Inv")
	if err == nil || !strings.Contains(err.Error(), "temporal") {
		t.Fatalf("got %v", err)
	}
}

func TestEnabledCanBeCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := &model{ctx: ctx, names: []string{"x"}}
	_, err := m.enabled(&parser.Expression{Kind: "Identifier", Text: "TRUE"}, evaluator.State{"x": {}}, nil)
	if err != context.Canceled {
		t.Fatalf("got %v", err)
	}
}

func TestHigherActionCaptureIsNotSilentlyChanged(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nAct(Op(_)) == \\E z \\in 1..2 : x' = Op(z)\nInit == x = 0\nNext == \\E z \\in 1..2 : Act(LAMBDA n : n + z)\n===="
	_, err := Check(source, "INIT Init NEXT Next")
	if err == nil || !strings.Contains(err.Error(), "capture-avoiding") {
		t.Fatalf("got %v", err)
	}
}
