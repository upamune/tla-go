package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLILivenessDebugIsOptIn(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(model, []byte(`---- MODULE M ----
EXTENDS Integers
VARIABLE x
Init == x = 0
Next == x' = 1 - x
Spec == Init /\ [][Next]_x /\ WF_x(Next)
P == (x = 0) ~> (x = 2)
====`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE"), 0o600); err != nil {
		t.Fatal(err)
	}
	var plainOut, plainErr, debugOut, debugErr bytes.Buffer
	plainCode := run(context.Background(), []string{model}, &plainOut, &plainErr)
	debugCode := run(context.Background(), []string{"-debug-liveness", model}, &debugOut, &debugErr)
	if plainCode != debugCode || plainOut.String() != debugOut.String() || plainErr.Len() != 0 {
		t.Fatalf("debugging changed CLI result: %d %q %q vs %d %q", plainCode, plainOut.String(), plainErr.String(), debugCode, debugOut.String())
	}
	if !strings.Contains(debugErr.String(), `"event":"fair-component"`) {
		t.Fatalf("missing debug events: %s", debugErr.String())
	}
}

func TestCLIReducedTableauDebugIsOptIn(t *testing.T) {
	dir := copySymmetryRecurrenceFixtures(t)
	model := filepath.Join(dir, "ChooseTableauSymmetryMC.tla")
	var plainOut, plainErr, debugOut, debugErr bytes.Buffer
	plainCode := run(context.Background(), []string{model}, &plainOut, &plainErr)
	debugCode := run(context.Background(), []string{"-debug-liveness", model}, &debugOut, &debugErr)
	if plainCode != 13 || debugCode != plainCode || plainOut.String() != debugOut.String() || plainErr.Len() != 0 {
		t.Fatalf("debugging changed CLI result: %d %q %q vs %d %q", plainCode, plainOut.String(), plainErr.String(), debugCode, debugOut.String())
	}
	if !strings.Contains(debugErr.String(), `"event":"tableau-node"`) || !strings.Contains(debugErr.String(), `"event":"tableau-regenerated"`) {
		t.Fatalf("missing regenerated tableau debug events: %s", debugErr.String())
	}
}
