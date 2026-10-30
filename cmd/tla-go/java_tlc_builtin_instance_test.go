package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestJavaTLCStandardModuleInstance(t *testing.T) {
	cases := []struct {
		name, body, output string
		code               int
	}{
		{"print", "X == INSTANCE TLC\nASSUME X!PrintT(\"hello\")", `"hello"`, 0},
		{"no unqualified import", "X == INSTANCE TLC\nASSUME PrintT(\"hello\")", "undefined symbol PrintT", 150},
		{"shadowed name", "X == INSTANCE TLC\nPrintT(x) == FALSE\nASSUME X!PrintT(\"hello\") /\\ ~PrintT(\"hello\")", `"hello"`, 0},
		{"runtime", "X == INSTANCE TLC\nASSUME X!TLCSet(1, 12) /\\ X!TLCGet(1) = 12", "", 0},
		{"any", "X == INSTANCE TLC\nASSUME 1 \\in X!Any", "", 0},
		{"java time", "X == INSTANCE TLC\nASSUME X!JavaTime \\in 0..2147483647", "", 0},
		{"sorting", "X == INSTANCE TLC\nLess(a,b) == a < b\nASSUME X!SortSeq(<<3,1,2>>, Less) = <<1,2,3>>", "", 0},
		{"operator argument", "X == INSTANCE TLC\nApply(op(_), arg) == op(arg)\nASSUME Apply(X!ToString, 42) = \"42\"", "", 0},
		{"permutations", "X == INSTANCE TLC\nASSUME X!Permutations({1,2}) = {[i \\in {1,2} |-> i], [i \\in {1,2} |-> 3-i]}", "", 0},
		{"local let runtime", "ASSUME LET X == INSTANCE TLC IN X!TLCSet(1, 12) /\\ X!TLCGet(1) = 12", "", 0},
		{"unqualified instance", "INSTANCE TLC\nASSUME PrintT(\"hello\")", `"hello"`, 0},
		{"unused parameter", "X(p) == INSTANCE TLC", "", 0},
		{"parameterized call", "X(p) == INSTANCE TLC\nASSUME X(1)!PrintT(\"hello\")", "array is not of length 1", 10},
		{"parameter evaluation", "Y == INSTANCE TLC\nX(p) == INSTANCE TLC\nASSUME X(Y!PrintT(\"first\"))!PrintT(Y!PrintT(\"second\"))", "array is not of length 1", 10},
		{"parameterized any", "X(p) == INSTANCE TLC\nASSUME 1 \\in X(1)!Any", "cannot convert MethodHandle()Value", 10},
		{"parameterized time", "X(p) == INSTANCE TLC\nASSUME X(1)!JavaTime \\in 0..2147483647", "cannot convert MethodHandle()Value", 10},
		{"parameterized sort", "X(p) == INSTANCE TLC\nLess(a,b) == a < b\nASSUME X(1)!SortSeq(<<2,1>>,Less) = <<1,2>>", "array is not of length 2", 10},
		{"invalid substitution", "Foo(p) == TRUE\nX == INSTANCE TLC WITH PrintT <- Foo\nASSUME X!PrintT(\"hello\")", "invalid instance substitution", 150},
		{"use def", "X == INSTANCE TLC\nUSE DEF X!PrintT\nASSUME X!PrintT(\"hello\")", `"hello"`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeInstanceFixture(t, dir, tc.body)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code || tc.output != "" && !strings.Contains(out.String()+errout.String(), tc.output) {
				t.Fatalf("exit %d, want %d; stdout: %s; stderr: %s", code, tc.code, out.String(), errout.String())
			}
			if code == 0 && !strings.Contains(out.String(), "2 states generated, 1 distinct states found") {
				t.Fatalf("unexpected state counts: %s", out.String())
			}
			if tc.name == "parameter evaluation" {
				first, second := strings.Index(out.String(), `"first"`), strings.Index(out.String(), `"second"`)
				if first < 0 || second <= first {
					t.Fatalf("wrong parameter evaluation order: %s", out.String())
				}
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				javaOutput := compareJavaInstance(t, jar, dir, tc.code)
				if tc.name == "parameter evaluation" {
					first, second := strings.Index(javaOutput, `"first"`), strings.Index(javaOutput, `"second"`)
					if first < 0 || second <= first {
						t.Fatalf("Java parameter evaluation order: %s", javaOutput)
					}
				}
			}
		})
	}
}

func TestJavaTLCInstanceStatefulRuntime(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"M.tla": "---- MODULE M ----\nEXTENDS Naturals\nX == INSTANCE TLC\nVARIABLE x\nInit == x = 0 /\\ X!TLCSet(1, 0)\nNext == /\\ x < 2 /\\ x' = x + 1 /\\ X!TLCSet(1, x')\nInv == X!TLCGet(1) >= 0\n====\n",
		"M.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 11 || !strings.Contains(out.String(), "3 states generated, 3 distinct states found") {
		t.Fatalf("exit %d; stdout: %s; stderr: %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaInstance(t, jar, dir, 11)
	}
}

func TestJavaTLCInstanceRandomEvaluation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       []string
	}{
		{"two draws", "ASSUME X!PrintT(X!RandomElement(1..100))\nASSUME X!PrintT(X!RandomElement(1..100))", []string{"73", "69"}},
		{"constant", "R == X!TLCEval(X!RandomElement(1..100))\nASSUME X!PrintT(R) /\\ X!PrintT(R)", []string{"73", "73"}},
		{"parameterized", "F(i) == X!TLCEval(X!RandomElement(1..100))\nASSUME X!PrintT(F(1)) /\\ X!PrintT(F(2))", []string{"73", "69"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeInstanceFixture(t, dir, "X == INSTANCE TLC\n"+tc.body)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-seed", "42", filepath.Join(dir, "M.tla")}, &out, &errout)
			if got := randomOutput(out.String()); code != 0 || !slices.Equal(got, tc.want) {
				t.Fatalf("exit %d, values %v, stderr %s", code, got, errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				checkJavaInstanceRandom(t, jar, dir, tc.want)
			}
		})
	}
}

func randomOutput(output string) []string {
	return regexp.MustCompile(`(?m)^\d+$`).FindAllString(output, -1)
}

func checkJavaInstanceRandom(t *testing.T, jar, dir string, want []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", "-seed", "42", "M.tla")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil || !slices.Equal(randomOutput(string(output)), want) {
		t.Fatalf("Java TLC: %v\n%s", err, output)
	}
}

func writeInstanceFixture(t *testing.T, dir, body string) {
	t.Helper()
	files := map[string]string{
		"M.tla": "---- MODULE M ----\nEXTENDS Naturals\n" + body + "\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n====\n",
		"M.cfg": "INIT Init\nNEXT Next\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func compareJavaInstance(t *testing.T, jar, dir string, code int) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", "M.tla")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
		t.Fatalf("Java TLC: %v\n%s", err, output)
	}
	return string(output)
}

func TestJavaTLCInstanceLocalModule(t *testing.T) {
	dir := t.TempDir()
	writeInstanceFixture(t, dir, "X == INSTANCE TLC\nASSUME X!Foo = 42")
	if err := os.WriteFile(filepath.Join(dir, "TLC.tla"), []byte("---- MODULE TLC ----\nFoo == 42\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout); code != 0 {
		t.Fatalf("exit %d; stdout: %s; stderr: %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaInstance(t, jar, dir, 0)
	}
}

func TestJavaTLCInstanceDOT(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"M.tla": "---- MODULE M ----\nEXTENDS Naturals\nX == INSTANCE TLC\nVARIABLES x,y,z\nInit == /\\ x=0 /\\ y=1 /\\ z=2\nNext == /\\ x' = (x+1)%2 /\\ UNCHANGED <<y,z>>\n====\n",
		"M.cfg": "INIT Init\nNEXT Next\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "go.dot")
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-workers", "1", "-fp", "0", "-dump", "dot,actionlabels", path, filepath.Join(dir, "M.tla")}, &out, &errout); code != 0 {
		t.Fatalf("exit %d; stdout: %s; stderr: %s", code, out.String(), errout.String())
	}
	graph, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaDOT(t, jar, dir, graph, 0)
	}
}
