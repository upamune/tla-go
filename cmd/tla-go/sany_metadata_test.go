package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSANYMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "M.tla")
	source := "---- MODULE M ----\nVARIABLES x,y\n" +
		"F(a) == TRUE\nG == F(x)\nH(a) == a\nI == H(x)\n" +
		"J == ENABLED (x' = x)\nK == LET z == x' IN TRUE\nL == [] (x = x)\n====\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-sany-metadata", path}, &out, &errout); code != 0 {
		t.Fatalf("exit %d: %s", code, errout.String())
	}
	got := summarizeSANY(t, out.Bytes())
	want := sanySummary{variables: []string{"x", "y"}, definitions: map[string]int{"F": 0, "G": 0, "H": 0, "I": 1, "J": 1, "K": 0, "L": 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("metadata: got %+v, want %+v", got, want)
	}
}

func TestSANYMetadataRejectsUnsupported(t *testing.T) {
	for _, tc := range []struct{ name, body, message string }{
		{"unknown symbol", "A == missing", "missing"},
		{"temporal enabled", "A == ENABLED ([]TRUE)", "ENABLED"},
		{"recursive definition", "RECURSIVE F(_)\nF(n) == IF n = 0 THEN 0 ELSE F(n-1)", "recursive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "M.tla")
			if err := os.WriteFile(path, []byte("---- MODULE M ----\nEXTENDS Naturals\n"+tc.body+"\n====\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-sany-metadata", path}, &out, &errout)
			if code == 0 || !strings.Contains(errout.String(), tc.message) || out.Len() != 0 {
				t.Fatalf("exit %d, stdout %s, stderr %s", code, out.String(), errout.String())
			}
		})
	}
}
