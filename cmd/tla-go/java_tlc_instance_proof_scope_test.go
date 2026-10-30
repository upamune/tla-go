package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestJavaTLCProofLocalDefinitionsInInstance(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"Base.tla": "---- MODULE Base ----\nBaseValue == TRUE\n====\n",
		"A.tla":    "---- MODULE A ----\nEXTENDS Base\nVARIABLE x\nTHEOREM T == TRUE\n<1>1 (LET F == x IN F) = x\n<1> QED BY <1>1\n====\n",
		"M.tla":    "---- MODULE M ----\nVARIABLE x\nI == INSTANCE A WITH x <- x\nInit == x = 0\nNext == UNCHANGED x\n====\n",
		"M.cfg":    "INIT Init\nNEXT Next\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 0 || errout.Len() != 0 {
		t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaToolMessages(t, jar, dir, 0, readToolMessages(out.String()))
	}
}
