package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCTheoremNewDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		code       int
	}{
		{"constant", "THEOREM T == ASSUME NEW a, a = a PROVE a = a", 0},
		{"unnamed", "THEOREM ASSUME NEW a PROVE a = a", 0},
		{"variable", "THEOREM T == ASSUME NEW VARIABLE a PROVE a = a", 0},
		{"state", "THEOREM T == ASSUME NEW STATE a PROVE a = a", 0},
		{"action", "THEOREM T == ASSUME NEW ACTION A PROVE A = A", 0},
		{"operator", "THEOREM T == ASSUME NEW F(_), F(1) = F(1) PROVE TRUE", 0},
		{"infix operator", "THEOREM T == ASSUME NEW _ + _ PROVE 1 + 2 = 1 + 2", 0},
		{"bounded", "THEOREM T == ASSUME NEW a \\in {1}, a = 1 PROVE a = 1", 0},
		{"dependent bound", "THEOREM T == ASSUME NEW a, NEW b \\in {a} PROVE b = a", 0},
		{"later declaration", "THEOREM T == ASSUME NEW a PROVE a = a\na == 1", 0},
		{"duplicate", "THEOREM T == ASSUME NEW a, NEW a PROVE a = a", 0},
		{"unbound assumption", "THEOREM T == ASSUME a = a, NEW a PROVE TRUE", 150},
		{"unbound outside", "THEOREM T == ASSUME NEW a PROVE a = a\nB == a", 150},
		{"conflict", "a == 1\nTHEOREM T == ASSUME NEW a PROVE a = a", 150},
		{"standard name", "EXTENDS Naturals\nTHEOREM T == ASSUME NEW Nat PROVE TRUE", 150},
		{"builtin operator", "THEOREM T == ASSUME NEW _ = _ PROVE TRUE", 150},
		{"arity", "THEOREM T == ASSUME NEW F(_) PROVE F(1, 2) = 1", 150},
		{"self reference", "THEOREM T == ASSUME NEW a \\in {a} PROVE TRUE\na == 1", 150},
		{"bounded variable", "THEOREM T == ASSUME NEW VARIABLE a \\in {1} PROVE TRUE", 150},
		{"bounded operator", "THEOREM T == ASSUME NEW F(_) \\in {1} PROVE TRUE", 150},
		{"proof local fact", "THEOREM T == ASSUME NEW a PROVE a = a BY a = a", 0},
		{"proof unknown fact", "THEOREM T == ASSUME NEW a PROVE a = a BY b = b", 150},
		{"proof local is not definition", "THEOREM T == ASSUME NEW a PROVE a = a BY DEF a", 150},
		{"proof forward definition", "THEOREM T == ASSUME NEW a PROVE a = a BY DEF F\nF == TRUE", 150},
		{"proof previous definition", "F == TRUE\nTHEOREM T == ASSUME NEW a PROVE a = a BY DEF F", 0},
		{"unnamed proof", "THEOREM ASSUME NEW a PROVE a = a BY a = a", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "M.tla")
			if err := os.WriteFile(path, []byte("---- MODULE M ----\n"+tc.body+"\n====\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", path}, &out, &errout)
			if code != tc.code {
				t.Fatalf("Go exit %d want %d\n%s\n%s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", path)
				cmd.Dir = dir
				javaOutput, err := javatmp.Apply(cmd).CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.code {
					t.Fatalf("Java: %v\n%s", err, javaOutput)
				}
			}
		})
	}
}

func TestJavaSANYTheoremNewLevels(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to compare SANY levels")
	}
	for _, tc := range []struct {
		name, statement string
		level           int
	}{
		{"constant", "NEW a PROVE a = a", 0},
		{"variable", "NEW VARIABLE a PROVE a = a", 1},
		{"state", "NEW STATE a PROVE a = a", 1},
		{"action", "NEW ACTION A PROVE A = A", 2},
		{"action operator", "NEW ACTION A(_) PROVE A(v)", 2},
		{"state-bound", "NEW a \\in {v} PROVE a = a", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "M.tla")
			source := "---- MODULE M ----\nVARIABLE v\nTHEOREM T == ASSUME " + tc.statement + "\n====\n"
			if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{"-sany-metadata", path}, &out, &errout); code != 0 {
				t.Fatalf("Go exit %d: %s", code, errout.String())
			}
			got := summarizeSANY(t, out.Bytes()).definitions["T"]
			cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-cp", jar, "tla2sany.xml.XMLExporter", "-o", path)
			java, err := javatmp.Apply(cmd).Output()
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				Entries []struct {
					Theorem *struct {
						Name  string `xml:"uniquename"`
						Level int    `xml:"level"`
					} `xml:"TheoremDefNode"`
				} `xml:"context>entry"`
			}
			if err := xml.Unmarshal(java, &doc); err != nil {
				t.Fatal(err)
			}
			for _, entry := range doc.Entries {
				if entry.Theorem != nil && entry.Theorem.Name == "T" {
					if got != tc.level || got != entry.Theorem.Level {
						t.Fatalf("Go level %d, Java level %d, expected %d", got, entry.Theorem.Level, tc.level)
					}
					return
				}
			}
			t.Fatal("Java XML does not contain theorem T")
		})
	}
}

func TestJavaTLCTheoremNewImports(t *testing.T) {
	for _, theorem := range []string{
		"THEOREM T == ASSUME NEW a PROVE a = a BY DEF Imported, Local",
		"THEOREM ASSUME NEW a PROVE a = a BY DEF Imported, Local",
	} {
		t.Run(theorem, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"A.tla": "---- MODULE A ----\n\n\n\n\nImported == TRUE\n====\n",
				"M.tla": "---- MODULE M ----\nEXTENDS A\nLocal == TRUE\n" + theorem + "\n====\n",
				"M.cfg": "",
			}
			for name, content := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout); code != 0 {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
				cmd.Dir = dir
				if output, err := javatmp.Apply(cmd).CombinedOutput(); err != nil {
					t.Fatalf("Java: %v\n%s", err, output)
				}
			}
		})
	}
}
