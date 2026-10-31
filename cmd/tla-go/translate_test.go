package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTranslateAndCheck translates an upstream PlusCal model, as the
// CallReturn1Test of the Java tools does, and checks the result.
func TestTranslateAndCheck(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join("..", "..", "pcal", "testdata", "upstream", "tlatools", "org.lamport.tlatools", "test-model", "pcal", "CallReturn1.tla")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	spec := filepath.Join(dir, "CallReturn1.tla")
	if err := os.WriteFile(spec, data, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-translate", "-unixEOL", "-wf", "-termination", spec}, &out, &errout)
	if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), "Translation completed.") {
		t.Fatalf("translate: exit %d, stdout %q, stderr %q", code, out.String(), errout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "CallReturn1.old")); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	code = run(context.Background(), []string{spec}, &out, &errout)
	if code != 0 || !strings.Contains(out.String(), "8 distinct states found") {
		t.Fatalf("check: exit %d, stdout %q, stderr %q", code, out.String(), errout.String())
	}
}

// The -spec options make Java run TLC on PlusCal's TLA+ specification of
// the translation, which tla-go does not provide.
func TestTranslateRejectsSpecOptions(t *testing.T) {
	spec := filepath.Join(t.TempDir(), "M.tla")
	if err := os.WriteFile(spec, []byte("---- MODULE M ----\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, option := range []string{"-spec", "-myspec", "-spec2", "-myspec2"} {
		var out, errout bytes.Buffer
		code := run(context.Background(), []string{"-translate", option, "PlusCal", spec}, &out, &errout)
		if code != 255 || !strings.Contains(out.String(), "not supported by tla-go") {
			t.Fatalf("%s: exit %d, stdout %q", option, code, out.String())
		}
	}
}

// TestUsageMentionsTranslate checks that the usage text shows how to start
// the translator, including after an option that only the translator takes.
func TestUsageMentionsTranslate(t *testing.T) {
	for _, args := range [][]string{{"-help"}, {"-unixEOL", "-translate", "M.tla"}} {
		var out, errout bytes.Buffer
		run(context.Background(), args, &out, &errout)
		if text := out.String() + errout.String(); !strings.Contains(text, "tla-go -translate [pcal.trans options] file.tla") {
			t.Errorf("%v: usage does not mention -translate:\n%s", args, text)
		}
	}
}
