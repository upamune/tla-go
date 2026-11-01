package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCStructuredProofFixture(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"test207.tla", "test207.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "test207.tla")}, &out, &errout)
	stats := "0 states generated, 0 distinct states found, 0 states left on queue."
	if code != 0 || !strings.Contains(out.String(), stats) {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	var metadata, metaErr bytes.Buffer
	if code := run(context.Background(), []string{"-sany-metadata", filepath.Join(dir, "test207.tla")}, &metadata, &metaErr); code != 0 {
		t.Fatalf("metadata exit %d: %s", code, metaErr.String())
	}
	if level, ok := summarizeSANY(t, metadata.Bytes()).definitions["Thm"]; !ok || level != 0 {
		t.Fatalf("theorem level %d (present %t), want 0", level, ok)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "test207", 0)
		if !strings.Contains(java, stats) {
			t.Fatalf("Java output: %s", java)
		}
	}
}

func TestJavaTLCStructuredProofScopes(t *testing.T) {
	for _, tc := range []struct {
		name, proof string
		code        int
	}{
		{"prior step", "<1>1. TRUE\n<1>2. TRUE BY <1>1\n<1>3. QED", 0},
		{"forward step", "<1>1. TRUE BY <1>2\n<1>2. TRUE\n<1>3. QED", 150},
		{"self reference", "<1>1. TRUE BY <1>1\n<1>2. QED", 0},
		{"declaration in subproof", "<1>1. ASSUME NEW K PROVE TRUE\n <2>1. K=K\n <2>2. QED\n<1>2. QED", 0},
		{"declaration absent after", "<1>1. ASSUME NEW K PROVE TRUE\n<1>2. K=K\n<1>3. QED", 150},
		{"suffices visible after", "<1>1. SUFFICES ASSUME NEW K PROVE TRUE\n<1>2. K=K\n<1>3. QED", 0},
		{"suffices absent during", "<1>1. SUFFICES ASSUME NEW K PROVE TRUE\n <2>1. K=K\n <2>2. QED\n<1>2. QED", 150},
		{"define local", "<1>1. DEFINE F == TRUE\n<1>2. F BY DEF F\n<1>3. QED", 0},
		{"multiple definitions", "<1> DEFINE F == TRUE G == F\n<1>2. G BY DEF F, G\n<1> QED", 0},
		{"forward local definition", "<1> DEFINE F == G G == TRUE\n<1> QED", 150},
		{"define cannot have proof", "<1>1. DEFINE F == TRUE BY TRUE\n<1>2. QED", 150},
		{"define is not a fact", "<1>1. DEFINE F == TRUE\n<1>2. TRUE BY <1>1\n<1>3. QED", 150},
		{"define does not escape", "<1>1. TRUE\n <2>1. DEFINE F == TRUE\n <2>2. QED\n<1>2. F\n<1>3. QED", 150},
		{"new is not later definition", "<1>1. ASSUME NEW F PROVE F=F\n <2>1. TRUE BY DEF F\n <2>2. QED\n<1>2. QED\nF == TRUE", 150},
		{"forward selector", "<1>1. F!lab\n<1>2. QED\nF == lab :: TRUE", 150},
		{"shadowed selector", "<1>1. ASSUME NEW F PROVE TRUE\n <2>1. F!lab\n <2>2. QED\n<1>2. QED\nF == lab :: TRUE", 150},
		{"duplicate define", "<1> DEFINE F == TRUE\n<1> DEFINE F == FALSE\n<1> QED", 150},
		{"skip depth", "<1>1. TRUE\n <3>1. TRUE\n <3>2. QED\n<1> QED", 0},
		{"missing qed", "<1>1. TRUE", 150},
		{"unscoped step ref", "BY <1>1", 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, contents := range map[string]string{"M.tla": "---- MODULE M ----\nTHEOREM T == TRUE\n" + tc.proof + "\n====\n", "M.cfg": ""} {
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
				cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
				cmd.Dir = dir
				output, err := javatmp.Apply(cmd).CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.code {
					t.Fatalf("Java: %v\n%s", err, output)
				}
			}
		})
	}
}

func TestJavaTLCAnonymousTheoremSelections(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"test216.tla", "test216.cfg", "test216a.tla", "test216b.tla", "test216c.tla"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "test216.tla")}, &out, &errout)
	stats := "28 states generated, 7 distinct states found, 0 states left on queue."
	if code != 0 || !strings.Contains(out.String(), stats) {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "test216", 0)
		if !strings.Contains(java, stats) {
			t.Fatalf("Java output: %s", java)
		}
	}
}
