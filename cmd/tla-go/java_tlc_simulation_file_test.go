package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestJavaTLCSimulationTraceFiles(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals
VARIABLES x, y
Init == /\ x \in 0..1 /\ y = 0
Next == /\ x' = (x + 1) % 2 /\ y' = y + 1
C == y < 2
Inv == y < 1
====
`
	for _, tc := range []struct {
		name, config               string
		depth, traces, exit, files int
	}{
		{"normal", "", 4, 2, 0, 2},
		{"empty", "", 0, 1, 0, 1},
		{"constraint", "CONSTRAINT C\n", 6, 1, 0, 1},
		{"violation", "INVARIANT Inv\n", 4, 1, 12, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n" + tc.config} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			goPrefix := filepath.Join(dir, "go")
			common := []string{"-tool", "-workers", "1", "-seed", "1", "-depth", fmt.Sprint(tc.depth)}
			args := append(append([]string{}, common...), "-simulate", fmt.Sprintf("file=%s,num=%d", goPrefix, tc.traces), filepath.Join(dir, "M.tla"))
			var out, errout bytes.Buffer
			if code := run(context.Background(), args, &out, &errout); code != tc.exit {
				t.Fatalf("exit %d, want %d: %s\n%s", code, tc.exit, out.String(), errout.String())
			}
			goFiles, err := filepath.Glob(goPrefix + "_0_*")
			if err != nil || len(goFiles) != tc.files {
				t.Fatalf("files %v, %v; want %d", goFiles, err, tc.files)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareSimulationFiles(t, jar, dir, common, tc.traces, tc.exit, goPrefix, goFiles, false)
			}
			if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" && tc.name == "normal" {
				compareSimulationFiles(t, jar, dir, common, tc.traces, tc.exit, goPrefix, goFiles, true)
			}
		})
	}
}

func compareSimulationFiles(t *testing.T, jar, dir string, common []string, traces, exit int, goPrefix string, goFiles []string, modern bool) {
	t.Helper()
	javaDir := t.TempDir()
	for _, name := range []string{"M.tla", "M.cfg"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(javaDir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	javaPrefix := filepath.Join(javaDir, "java")
	args := []string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}
	args = append(args, common...)
	args = append(args, "-simulate", fmt.Sprintf("num=%d,file=%s", traces, javaPrefix), "-metadir", filepath.Join(javaDir, "meta"), "M.tla")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", args...)
	cmd.Dir = javaDir
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != exit {
		t.Fatalf("Java exit %v, error %v: %s", cmd.ProcessState, err, output)
	}
	files, err := filepath.Glob(javaPrefix + "_0_*")
	if err != nil || len(files) != len(goFiles) {
		t.Fatalf("Java files %v, %v; Go files %v", files, err, goFiles)
	}
	for i := range files {
		if strings.TrimPrefix(files[i], javaPrefix) != strings.TrimPrefix(goFiles[i], goPrefix) {
			t.Fatalf("file names differ: Go %s, Java %s", goFiles[i], files[i])
		}
		got, err := os.ReadFile(goFiles[i])
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(files[i])
		if err != nil {
			t.Fatal(err)
		}
		if modern {
			if !strings.Contains(string(want), "\\* <Init line") || !strings.Contains(string(want), "STATE_5 ==") || strings.Contains(string(got), "STATE_5 ==") {
				t.Fatalf("unexpected modern trace:\nGo:\n%s\nJava:\n%s", got, want)
			}
			continue
		}
		if !reflect.DeepEqual(strings.ReplaceAll(string(got), goPrefix, "PREFIX"), strings.ReplaceAll(string(want), javaPrefix, "PREFIX")) {
			t.Fatalf("file %d differs:\nGo:\n%s\nJava:\n%s", i, got, want)
		}
	}
}

func TestJavaTLCSimulationTraceFileContinue(t *testing.T) {
	const source = "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nGood == x' = 0\nBad == x' = 1\nNext == Good \\/ Bad\nInv == x = 0\n===="
	dir := t.TempDir()
	for name, data := range map[string]string{"M.tla": source, "M.cfg": "INIT Init NEXT Next INVARIANT Inv"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	prefix := filepath.Join(dir, "go")
	common := []string{"-tool", "-workers", "1", "-seed", "1", "-continue", "-depth", "1"}
	args := append(append([]string{}, common...), "-simulate", "file="+prefix+",num=10", filepath.Join(dir, "M.tla"))
	var out, errout bytes.Buffer
	if code := run(context.Background(), args, &out, &errout); code != 255 {
		t.Fatalf("exit %d: %s\n%s", code, out.String(), errout.String())
	}
	files, err := filepath.Glob(prefix + "_0_*")
	if err != nil || len(files) != 5 {
		t.Fatalf("files %v, %v; want 5", files, err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareSimulationFiles(t, jar, dir, common, 10, 255, prefix, files, false)
	}
}

func TestJavaTLCSimulationTraceFileValues(t *testing.T) {
	const source = `---- MODULE M ----
VARIABLES x, y
Init == /\ x = [a |-> {1, 2}, b |-> <<1, 2>>] /\ y = {"one", "two"}
Next == UNCHANGED <<x, y>>
====
`
	dir := t.TempDir()
	for name, data := range map[string]string{"M.tla": source, "M.cfg": "INIT Init NEXT Next"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	prefix := filepath.Join(dir, "go")
	common := []string{"-tool", "-workers", "1", "-seed", "1", "-depth", "2"}
	args := append(append([]string{}, common...), "-simulate", "file="+prefix+",num=1", filepath.Join(dir, "M.tla"))
	var out, errout bytes.Buffer
	if code := run(context.Background(), args, &out, &errout); code != 0 {
		t.Fatalf("exit %d: %s\n%s", code, out.String(), errout.String())
	}
	files, err := filepath.Glob(prefix + "_0_*")
	if err != nil || len(files) != 1 {
		t.Fatalf("files %v, %v; want 1", files, err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareSimulationFiles(t, jar, dir, common, 1, 0, prefix, files, false)
	}
}

func TestSimulationTraceFileWriteFailure(t *testing.T) {
	dir := t.TempDir()
	source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n===="
	for name, data := range map[string]string{"M.tla": source, "M.cfg": "INIT Init NEXT Next"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	prefix := filepath.Join(dir, "missing", "trace")
	args := []string{"-tool", "-seed", "1", "-depth", "0", "-simulate", "file=" + prefix + ",num=1", filepath.Join(dir, "M.tla")}
	var out, errout bytes.Buffer
	if code := run(context.Background(), args, &out, &errout); code == 0 {
		t.Fatalf("file write failure reported success: %s\n%s", out.String(), errout.String())
	}
}
