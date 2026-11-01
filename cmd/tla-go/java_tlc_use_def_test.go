package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCModuleProofDirectives(t *testing.T) {
	cases := []struct {
		name, directive string
		code            int
	}{
		{"use definition", "USE DEF Foo", 0},
		{"multiple definitions", "USE DEFS Foo, Bar", 0},
		{"fact and definition", "USE Foo DEF Bar", 0},
		{"constant fact", "USE C", 0},
		{"variable fact", "USE x", 0},
		{"standard definition", "USE DEF PrintT, +", 0},
		{"hide definition", "HIDE DEF Foo", 0},
		{"hide theorem", "HIDE T", 0},
		{"missing definition", "USE DEF Missing", 150},
		{"forward definition", "USE DEF Later\nLater == TRUE", 150},
		{"forward fact", "USE Later\nLater == TRUE", 150},
		{"forward nested fact", "USE (Foo /\\ Later)\nLater == TRUE", 150},
		{"bound fact", "USE \\A y \\in {1}: y = 1", 0},
		{"empty definition", "USE DEF", 150},
		{"constant is not definition", "USE DEF C", 150},
		{"variable is not definition", "USE DEF x", 150},
		{"expression is not definition", "USE DEF Foo /\\ TRUE", 150},
		{"empty use", "USE", 150},
		{"hide expression", "HIDE Foo", 150},
		{"only unsupported", "USE ONLY DEF Foo", 150},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "CONSTANT C\nVARIABLE x\nFoo == TRUE\nBar == TRUE\nTHEOREM T == TRUE\n" + tc.directive + "\nInit == x = 0\nNext == UNCHANGED x"
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nCONSTANT C = 1\n")
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code {
				t.Fatalf("exit %d, want %d; stdout: %s; stderr: %s", code, tc.code, out.String(), errout.String())
			}
			if code == 0 && !strings.Contains(out.String(), "2 states generated, 1 distinct states found") {
				t.Fatalf("unexpected exploration output: %s", out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaProofDirective(t, jar, dir, tc.code)
			}
		})
	}
}

func compareJavaProofDirective(t *testing.T, jar, dir string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", "M.tla")
	cmd.Dir = dir
	output, err := javatmp.Apply(cmd).CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
		t.Fatalf("Java TLC: %v\n%s", err, output)
	}
}

func TestJavaTLCModuleProofDirectiveImports(t *testing.T) {
	for _, tc := range []struct {
		name, dependency, directive string
		code                        int
	}{
		{"definition", "Foo == TRUE\nUSE DEF Foo", "USE DEF Foo", 0},
		{"private definition", "LOCAL Foo == TRUE\nUSE DEF Foo", "", 0},
		{"theorem", "THEOREM T == TRUE", "HIDE T", 0},
		{"extension forward reference", "USE DEF Foo\nFoo == TRUE", "", 150},
		{"root forward reference", "Foo == TRUE", "USE DEF Later\nLater == TRUE", 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"M.tla": "---- MODULE M ----\nEXTENDS A\n" + tc.directive + "\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n====\n",
				"A.tla": "---- MODULE A ----\n" + tc.dependency + "\n====\n",
				"M.cfg": "INIT Init\nNEXT Next\n",
			}
			for name, content := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout); code != tc.code {
				t.Fatalf("exit %d, want %d; stdout: %s; stderr: %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaProofDirective(t, jar, dir, tc.code)
			}
		})
	}
}

func TestJavaTLCProofDirectiveInstance(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"M.tla": "---- MODULE M ----\nI == INSTANCE A\nUSE DEF I!Foo\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n====\n",
		"A.tla": "---- MODULE A ----\nFoo == TRUE\n====\n",
		"M.cfg": "INIT Init\nNEXT Next\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout); code != 0 {
		t.Fatalf("exit %d; stdout: %s; stderr: %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaProofDirective(t, jar, dir, 0)
	}
}
