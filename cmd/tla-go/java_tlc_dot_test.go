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
)

func TestJavaTLCDotGraph(t *testing.T) {
	cases := []struct {
		name, body, cfg, want string
		code                  int
	}{
		{"branch", "VARIABLE x\nInit == x = 0\nUp == x < 2 /\\ x' = x + 1\nStay == UNCHANGED x\nDown == x > 0 /\\ x' = x - 1\nNext == Up \\/ Stay \\/ Down", "INIT Init\nNEXT Next\n", `-3406341414084290173 -> -686636423115914061 [label="Up",color="black",fontcolor="black"];`, 0},
		{"invariant", "VARIABLE x\nInit == x = 0\nNext == x' = x + 1\nInv == x < 2", "INIT Init\nNEXT Next\nINVARIANT Inv\n", `-7115858903467826205 [label="x = 2"];`, 12},
		{"initial invariant", "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nInv == x > 0", "INIT Init\nNEXT Next\nINVARIANT Inv\n", "color=\"white\";\n", 12},
		{"constraint", "VARIABLE x\nInit == x = 0\nNext == x' = x + 1\nBound == x < 2", "INIT Init\nNEXT Next\nCONSTRAINT Bound\n", `-686636423115914061 [label="x = 1"];`, 0},
		{"duplicate actions", "VARIABLE x\nInit == x \\in {0,1}\nA == /\\ x = 0 /\\ x' = 1\nB == /\\ x = 0 /\\ x' = 1\nStay == UNCHANGED x\nNext == A \\/ B \\/ Stay", "INIT Init\nNEXT Next\n", `label="B"`, 0},
		{"unicode", "VARIABLES x,y\nInit == /\\ x \\in {0,1} /\\ y = \"雪\\\"b\"\nOne == /\\ x = 0 /\\ x' = 1 /\\ UNCHANGED y\nHold == UNCHANGED <<x,y>>\nNext == One \\/ Hold", "INIT Init\nNEXT Next\n", `雪`, 0},
		{"view", "VARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 4\nV == x % 2", "INIT Init\nNEXT Next\nVIEW V\n", `-686636423115914061 -> -3406341414084290173`, 0},
		{"model values", "CONSTANT S\nVARIABLE x\nInit == x \\in S\nNext == UNCHANGED x", "INIT Init\nNEXT Next\nCONSTANT S = {a,b}\n", `label="x = a"`, 0},
		{"symmetry", "CONSTANT S\nVARIABLE x\nInit == x \\in S\nNext == x' \\in S\nSym == Permutations(S)", "INIT Init\nNEXT Next\nCONSTANT S = {a,b,c}\nSYMMETRY Sym\n", `label="x = a"`, 0},
		{"action constraint", "VARIABLE x\nInit == x = 0\nA == x' = x + 1\nB == UNCHANGED x\nNext == A \\/ B\nBound == x' < 3", "INIT Init\nNEXT Next\nACTION_CONSTRAINT Bound\n", `label="A"`, 0},
		{"deadlock", "VARIABLE x\nInit == x = 0\nNext == x = 1 /\\ UNCHANGED x", "INIT Init\nNEXT Next\n", `style = filled`, 11},
		{"rank resizing", "VARIABLE x\nInit == x \\in 0..30\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", `{rank = same;`, 0},
		{"many depths", "VARIABLE x\nInit == x = 0\nNext == IF x < 20 THEN x' = x + 1 ELSE UNCHANGED x", "INIT Init\nNEXT Next\n", `{rank = same;`, 0},
		{"liveness", "VARIABLE x\nInit == x = 0\nNext == IF x=0 THEN x' = 1 ELSE UNCHANGED x\nSpec == Init /\\ [][Next]_x\nProp == <> (x = 1)", "SPECIFICATION Spec\nPROPERTY Prop\n", `[style="dashed"]`, 13},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, tc.body, tc.cfg)
			path := filepath.Join(dir, "go.dot")
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-workers", "1", "-fp", "0", "-dump", "dot,actionlabels", path, filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code {
				t.Fatalf("exit %d, want %d; stdout: %s; stderr: %s", code, tc.code, out.String(), errout.String())
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(got), tc.want) {
				t.Fatalf("DOT missing %q:\n%s", tc.want, got)
			}
			if tc.name == "initial invariant" && strings.HasSuffix(string(got), "}") {
				t.Fatalf("initial failure should leave incomplete graph: %s", got)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaDOT(t, jar, dir, got, tc.code)
			}
		})
	}
}

func writeDOTFixture(t *testing.T, dir, body, cfg string) {
	t.Helper()
	for name, content := range map[string]string{"M.tla": "---- MODULE M ----\nEXTENDS Naturals, TLC, FiniteSets\n" + body + "\n====\n", "M.cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func compareJavaDOT(t *testing.T, jar, dir string, got []byte, wantCode int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", "-dump", "dot,actionlabels", "java.dot", "M.tla")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != wantCode {
		t.Fatalf("Java TLC: %v\n%s", err, output)
	}
	want, err := os.ReadFile(filepath.Join(dir, "java.dot"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("DOT differs from Java TLC\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDotOptionErrors(t *testing.T) {
	for _, args := range [][]string{
		{"-dump"},
		{"-dump", "dot,actionlabels"},
		{"-dump", "dot,actionlabels", "", "M.tla"},
		{"-dump", "dot", "out.dot", "M.tla"},
		{"-workers", "2", "M.tla"},
		{"-workers", "oops", "M.tla"},
		{"-fp", "-1", "M.tla"},
		{"-fp", "131", "M.tla"},
	} {
		if _, err := parseOptions(args); err == nil {
			t.Errorf("parseOptions(%q) unexpectedly succeeded", args)
		}
	}
}

func TestDotOutputErrors(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-dump", "dot,actionlabels", filepath.Join(dir, "missing", "out.dot"), filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 255 || !strings.Contains(errout.String(), "create DOT dump") {
		t.Fatalf("exit %d, stderr %q", code, errout.String())
	}
}

func TestDotOutputSuffix(t *testing.T) {
	for input, want := range map[string]string{"graph": "graph.dot", "graph.dot": "graph.dot", "graph.foo": "graph.foo.dot", "graph.DOT": "graph.DOT.dot"} {
		opts, err := parseOptions([]string{"-dump", "dot,actionlabels", input, "M.tla"})
		if err != nil || opts.dumpPath != want {
			t.Errorf("dump %q: got %q, error %v; want %q", input, opts.dumpPath, err, want)
		}
	}
}

func TestJavaTLCToolDotSuffix(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
	path := filepath.Join(dir, "graph")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-dump", "dot,actionlabels", path, filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 0 {
		t.Fatalf("exit %d; stdout %s; stderr %s", code, out.String(), errout.String())
	}
	graph, err := os.ReadFile(path + ".dot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unexpected file without suffix: %v", err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaDOT(t, jar, dir, graph, 0)
	}
}
