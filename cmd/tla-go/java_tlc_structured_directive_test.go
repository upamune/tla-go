package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestJavaTLCStructuredProofDirectives(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		code       int
	}{
		{"use definition", "F == TRUE\nTHEOREM T == TRUE\n<1> USE DEF F\n<1> QED", 0},
		{"hide definition", "F == TRUE\nTHEOREM T == TRUE\n<1> HIDE DEF F\n<1> QED", 0},
		{"local definition", "THEOREM T == TRUE\n<1> DEFINE F == TRUE\n<1> USE DEF F\n<1> HIDE DEF F\n<1> QED", 0},
		{"use formula", "THEOREM T == TRUE\n<1> USE TRUE\n<1> QED", 0},
		{"hide formula", "THEOREM T == TRUE\n<1> HIDE TRUE\n<1> QED", 150},
		{"prior step", "THEOREM T == TRUE\n<1>1 TRUE\n<1> USE <1>1\n<1> HIDE <1>1\n<1> QED", 0},
		{"hidden step explicit use", "THEOREM T == TRUE\n<1>1 TRUE\n<1> HIDE <1>1\n<1>2 TRUE BY <1>1\n<1> QED", 0},
		{"hide theorem", "THEOREM U == TRUE\nTHEOREM T == TRUE\n<1> HIDE U\n<1> QED", 0},
		{"hide assumption", "ASSUME A == TRUE\nTHEOREM T == TRUE\n<1> HIDE A\n<1> QED", 0},
		{"forward step", "THEOREM T == TRUE\n<1> USE <1>1\n<1>1 TRUE\n<1> QED", 150},
		{"forward definition", "THEOREM T == TRUE\n<1> USE DEF F\n<1> QED\nF == TRUE", 150},
		{"use only", "F == TRUE\nTHEOREM T == TRUE\n<1> USE ONLY DEF F\n<1> QED", 0},
		{"hide only", "F == TRUE\nTHEOREM T == TRUE\n<1> HIDE ONLY DEF F\n<1> QED", 150},
		{"named directive", "F == TRUE\nTHEOREM T == TRUE\n<1>1 USE DEF F\n<1> QED", 0},
		{"directive not a fact", "F == TRUE\nTHEOREM T == TRUE\n<1>1 USE DEF F\n<1>2 TRUE BY <1>1\n<1> QED", 150},
		{"directive has proof", "F == TRUE\nTHEOREM T == TRUE\n<1> USE DEF F BY TRUE\n<1> QED", 150},
		{"empty directive", "THEOREM T == TRUE\n<1> USE\n<1> QED", 150},
		{"hide definition as fact", "F == TRUE\nTHEOREM T == TRUE\n<1> HIDE F\n<1> QED", 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, contents := range map[string]string{"M.tla": "---- MODULE M ----\n" + tc.body + "\n====\n", "M.cfg": ""} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
				cmd.Dir = dir
				output, err := cmd.CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.code {
					t.Fatalf("Java: %v\n%s", err, output)
				}
			}
		})
	}
}

func TestJavaTLCProofDirectiveModules(t *testing.T) {
	for _, tc := range []struct {
		name, header, directive, post string
		code                          int
	}{
		{"self", "", "USE MODULE M", "", 0},
		{"missing", "", "USE MODULE Missing", "", 150},
		{"unimported standard", "", "HIDE MODULE Naturals", "", 150},
		{"direct standard", "EXTENDS Naturals", "HIDE MODULE Naturals", "", 0},
		{"standard dependency", "EXTENDS TLC", "USE MODULE Sequences", "", 0},
		{"extension", "EXTENDS A", "HIDE MODULE A", "", 0},
		{"transitive extension", "EXTENDS A", "USE MODULE B", "", 0},
		{"instance", "I == INSTANCE A", "HIDE MODULE A", "", 0},
		{"transitive instance", "I == INSTANCE A", "USE MODULE B", "", 0},
		{"later instance", "", "USE MODULE A", "I == INSTANCE A\n", 0},
		{"let instance", "X == LET I == INSTANCE A IN TRUE", "USE MODULE B", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"A.tla": "---- MODULE A ----\nEXTENDS B\nC == TRUE\n====\n",
				"B.tla": "---- MODULE B ----\nD == TRUE\n====\n",
				"M.tla": "---- MODULE M ----\n" + tc.header + "\nTHEOREM T == TRUE\n<1> " + tc.directive + "\n<1> QED\n" + tc.post + "====\n",
				"M.cfg": "",
			}
			for name, contents := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
				cmd.Dir = dir
				output, err := cmd.CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.code {
					t.Fatalf("Java: %v\n%s", err, output)
				}
			}
		})
	}
}

func TestJavaTLCModuleProofDirectiveModuleFacts(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		code       int
	}{
		{"use self", "USE MODULE M", 0},
		{"hide self", "HIDE MODULE M", 0},
		{"use missing", "USE MODULE Missing", 150},
		{"hide missing", "HIDE MODULE Missing", 150},
		{"unimported standard", "USE MODULE Naturals", 150},
		{"use imported", "EXTENDS Naturals\nUSE MODULE Naturals", 0},
		{"hide imported", "EXTENDS Naturals\nHIDE MODULE Naturals", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, contents := range map[string]string{"M.tla": "---- MODULE M ----\n" + tc.body + "\n====\n", "M.cfg": ""} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
				cmd.Dir = dir
				output, err := cmd.CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.code {
					t.Fatalf("Java: %v\n%s", err, output)
				}
			}
		})
	}
}

func TestJavaTLCProofDirectiveNestedModule(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		code       int
	}{
		{"parent extension", "EXTENDS A\n---- MODULE N ----\nTHEOREM T == TRUE\n<1> USE MODULE A\n<1> QED\n====\nI == INSTANCE N", 0},
		{"parent module is not visible", "---- MODULE N ----\nTHEOREM T == TRUE\n<1> USE MODULE M\n<1> QED\n====\nI == INSTANCE N", 150},
		{"later parent instance", "---- MODULE N ----\nTHEOREM T == TRUE\n<1> USE MODULE A\n<1> QED\n====\nJ == INSTANCE A\nI == INSTANCE N", 0},
		{"transitive later parent instance", "---- MODULE N ----\nTHEOREM T == TRUE\n<1> USE MODULE B\n<1> QED\n====\nJ == INSTANCE A\nI == INSTANCE N", 0},
		{"transitive parent let instance", "---- MODULE N ----\nTHEOREM T == TRUE\n<1> USE MODULE B\n<1> QED\n====\nX == LET I == INSTANCE A IN TRUE\nJ == INSTANCE N", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{"A.tla": "---- MODULE A ----\nEXTENDS B\nF == TRUE\n====\n", "B.tla": "---- MODULE B ----\nG == TRUE\n====\n", "M.tla": "---- MODULE M ----\n" + tc.body + "\n====\n", "M.cfg": ""}
			for name, contents := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
				cmd.Dir = dir
				output, err := cmd.CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.code {
					t.Fatalf("Java: %v\n%s", err, output)
				}
			}
		})
	}
}

func TestJavaTLCProofDirectiveInstancedFacts(t *testing.T) {
	for _, tc := range []struct {
		name, header, directive string
		code                    int
	}{
		{"named theorem hide", "I == INSTANCE A", "HIDE I!U", 0},
		{"named theorem use", "I == INSTANCE A", "USE I!U", 0},
		{"transitive theorem hide", "I == INSTANCE A", "HIDE I!V", 0},
		{"unnamed theorem hide", "INSTANCE A", "HIDE U", 0},
		{"local instance in same module", "LOCAL I == INSTANCE A", "HIDE I!U", 0},
		{"local instance hidden from extension", "EXTENDS C", "HIDE I!U", 150},
		{"operator is not fact", "I == INSTANCE A", "HIDE I!F", 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"A.tla": "---- MODULE A ----\nEXTENDS B\nTHEOREM U == TRUE\nF == TRUE\n====\n",
				"B.tla": "---- MODULE B ----\nTHEOREM V == TRUE\n====\n",
				"C.tla": "---- MODULE C ----\nLOCAL I == INSTANCE A\n====\n",
				"M.tla": "---- MODULE M ----\n" + tc.header + "\nTHEOREM T == TRUE\n<1> " + tc.directive + "\n<1> QED\n====\n",
				"M.cfg": "",
			}
			for name, contents := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
				cmd.Dir = dir
				output, err := cmd.CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.code {
					t.Fatalf("Java: %v\n%s", err, output)
				}
			}
		})
	}
}
