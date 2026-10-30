package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIPrintsUpstreamBagSuite(t *testing.T) {
	path := filepath.Join("..", "..", "checker", "testdata", "bags", "test50.tla")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{path}, &out, &errout)
	if code != 0 || errout.Len() != 0 {
		t.Fatalf("code %d, stderr %q", code, errout.String())
	}
	text := out.String()
	for _, line := range []string{`"Test 1 OK"  TRUE`, `"Test 2 OK"  TRUE`, `"Test 3 OK"  TRUE`, `"Test 4 OK"  TRUE`, `"Test 5 OK"  TRUE`, `"Test 6 OK"  TRUE`, `"Test 7 OK"  TRUE`} {
		if strings.Count(text, line+"\n") != 1 {
			t.Fatalf("expected one output line %q, got %q", line, text)
		}
	}
}

func TestCLIAssumptionEvaluationError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "M.tla")
	source := "---- MODULE M ----\nEXTENDS Naturals\nASSUME 1 \\div 0 = 0\n===="
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{path}, &out, &errout)
	want := "Error: Evaluating assumption line 3, col 8 to line 3, col 19 of module M failed.\nThe second argument of \\div is 0.\n"
	if code != 10 || !strings.Contains(out.String(), want) || errout.Len() != 0 {
		t.Fatalf("code %d; stdout %q; stderr %q", code, out.String(), errout.String())
	}
}

func TestCLIPauseReadsInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Pause.tla")
	source := "---- MODULE Pause ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nNext == /\\ x' = x + 1 /\\ TLCSet(\"pause\", TRUE)\nLimit == x < 1\n===="
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Pause.cfg"), []byte("INIT Init NEXT Next CONSTRAINT Limit CHECK_DEADLOCK FALSE"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	input := strings.NewReader("\n")
	code := runWithInput(context.Background(), []string{path}, input, &out, &errout)
	if code != 0 || input.Len() != 0 || strings.Count(out.String(), "Press enter to resume model checking.") != 1 || errout.Len() != 0 {
		t.Fatalf("code %d, remaining input %d, stdout %q, stderr %q", code, input.Len(), out.String(), errout.String())
	}
}
