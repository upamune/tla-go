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

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCModelContinue(t *testing.T) {
	for _, tc := range []struct {
		name, body, cfg string
		code            int
	}{
		{"success", "Init == x=0\nNext == UNCHANGED x", "", 0},
		{"initial", "Init == x \\in {0,1}\nNext == UNCHANGED x\nInv == x=0", "INVARIANT Inv", 0},
		{"multiple initial", "Init == x=0\nNext == UNCHANGED x\nA == x=1\nB == x=2", "INVARIANT A B", 0},
		{"successor", "Init == x=0\nNext == x' \\in {0,1,2}\nInv == x=0", "INVARIANT Inv", 0},
		{"multiple invariants", "Init == x=0\nNext == x' \\in {0,1,2}\nA == x=0\nB == x#2", "INVARIANT A B", 0},
		{"state property", "Init == x=0\nNext == x' \\in {0,1,2}\nP == [](x=0)", "PROPERTY P", 0},
		{"initial state property", "Init == x=0\nNext == UNCHANGED x\nP == [](x=1)", "PROPERTY P", 0},
		{"action", "Init == x=0\nNext == x' \\in {0,1}\nP == [][x'=x]_x", "PROPERTY P", 0},
		{"invariant and action", "Init == x=0\nNext == x' \\in {0,1}\nInv == x=0\nP == [][x'=x]_x", "INVARIANT Inv\nPROPERTY P", 0},
		{"state constraint", "Init == x=0\nNext == x' \\in {0,1,2}\nInv == x=0\nBound == x#2", "INVARIANT Inv\nCONSTRAINT Bound", 0},
		{"action constraint", "Init == x=0\nNext == x' \\in {0,1,2}\nInv == x=0\nBound == x'#2", "INVARIANT Inv\nACTION_CONSTRAINT Bound", 0},
		{"view invariant", "Init == x=0\nNext == x' \\in {0,1,2,3}\nInv == x<2\nV == x % 2", "INVARIANT Inv\nVIEW V", 0},
		{"view action", "Init == x=0\nNext == x' \\in {0,1,2,3}\nP == [][x'=x]_x\nV == x % 2", "PROPERTY P\nVIEW V", 0},
		{"symmetry action", "CONSTANT S\nInit == x \\in S\nNext == x' \\in S\nP == [][x'=x]_x\nSym == Permutations(S)", "CONSTANT S = {a,b}\nPROPERTY P\nSYMMETRY Sym", 0},
		{"liveness after initial violation", "Init == x=0\nNext == UNCHANGED x\nInv == x=1\nP == <>(x=1)", "INVARIANT Inv\nPROPERTY P", 13},
		{"guarded temporal property", "Init == x=0\nNext == x'=1\nP == [](x=1 => [](x=2))", "PROPERTY P", 13},
		{"deadlock", "Init == x=0\nNext == /\\ x=0 /\\ x'=1", "", 11},
		{"deadlock after violation", "Init == x=0\nNext == /\\ x=0 /\\ x'=1\nInv == x=0", "INVARIANT Inv", 11},
		{"evaluation", "Init == x=0\nNext == x'=1\nInv == IF x=0 THEN TRUE ELSE 0=\"a\"", "INVARIANT Inv", 255},
		{"evaluation after violation", "Init == x=0\nNext == x' \\in {0,1,2}\nInv == IF x=0 THEN TRUE ELSE IF x=1 THEN FALSE ELSE 0=\"a\"", "INVARIANT Inv", 255},
		{"implied initial", "Init == x=0\nNext == UNCHANGED x\nP == x=1", "PROPERTY P", 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\n"+tc.body, "INIT Init\nNEXT Next\n"+tc.cfg+"\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-continue"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("Go exit %d, stderr %s, stdout %s", code, errout.String(), out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				want := javaModelContinueMessages(t, jar, dir, args, tc.code)
				got := modelContinueMessages(readToolMessages(out.String()))
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("Go messages %q\nJava messages %q", got, want)
				}
			}
		})
	}
}

func TestJavaTLCModelContinueUpstream(t *testing.T) {
	const sourceDir = "testdata/java-tlc/continue"
	for _, tc := range []struct {
		name, counts                string
		initial, traceStates, depth int
	}{
		{"Continue", "32 states generated, 29 distinct states found", 0, 32, 29},
		{"DoInitFunctorInvariantContinue", "21 states generated, 11 distinct states found", 10, -1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, ext := range []string{".tla", ".cfg"} {
				name := tc.name + ext
				content, err := os.ReadFile(filepath.Join(sourceDir, name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-continue"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, tc.name+".tla")), &out, &errout)
			if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), tc.counts) {
				t.Fatalf("Go exit %d, stderr %s, stdout %s", code, errout.String(), out.String())
			}
			got := modelContinueMessages(readToolMessages(out.String()))
			if count := countModelMessage(got, "2107:"); count != tc.initial {
				t.Fatalf("initial violations %d, want %d; messages %q", count, tc.initial, got)
			}
			if tc.traceStates >= 0 && countModelMessage(got, "2217:") != tc.traceStates {
				t.Fatalf("expected %d trace states, messages %q", tc.traceStates, got)
			}
			if tc.depth > 0 && !strings.Contains(out.String(), fmt.Sprintf("The depth of the complete state graph search is %d.", tc.depth)) {
				t.Fatalf("expected depth %d, stdout %s", tc.depth, out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, tc.name+".tla")...)...)
				cmd.Dir = dir
				output, err := javatmp.Apply(cmd).CombinedOutput()
				if err != nil || !strings.Contains(string(output), tc.counts) {
					t.Fatalf("Java: %v\n%s", err, output)
				}
				want := modelContinueMessages(readToolMessages(string(output)))
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("Go messages %q\nJava messages %q", got, want)
				}
			}
		})
	}
}

func countModelMessage(messages []string, prefix string) int {
	count := 0
	for _, message := range messages {
		if strings.HasPrefix(message, prefix) {
			count++
		}
	}
	return count
}

func javaModelContinueMessages(t *testing.T, jar, dir string, args []string, code int) []string {
	t.Helper()
	javaArgs := append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)
	cmd := exec.Command("java", javaArgs...)
	cmd.Dir = dir
	output, _ := javatmp.Apply(cmd).CombinedOutput()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
		t.Fatalf("Java exit %v, output %s", cmd.ProcessState, output)
	}
	return modelContinueMessages(readToolMessages(string(output)))
}

func TestJavaTLCModernModelContinueAlias(t *testing.T) {
	for _, tc := range []struct{ name, init string }{
		{"initial", "x \\in {0,1}"},
		{"successor", "x=0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == " + tc.init + "\nNext == x' \\in {0,1}\nInv == x=0\nA == [v |-> x]"
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\nALIAS A\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-continue"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			if code != 0 || errout.Len() != 0 {
				t.Fatalf("Go exit %d, stderr %s, stdout %s", code, errout.String(), out.String())
			}
			if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
				got := modelContinueMessages(readToolMessages(out.String()))
				want := javaModelContinueMessages(t, jar, dir, args, code)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("Go messages %q\nJava messages %q", got, want)
				}
			}
		})
	}
}

func modelContinueMessages(messages []toolMessage) []string {
	var result []string
	for _, msg := range messages {
		switch msg.code {
		case 2107, 2108, 2110, 2111, 2112, 2113, 2114, 2116, 2121, 2122, 2217, 2218:
			result = append(result, fmt.Sprintf("%d:%s", msg.code, msg.body))
		case 2193:
			result = append(result, "2193")
		}
	}
	return result
}

func TestJavaTLCModelContinueDOT(t *testing.T) {
	for _, tc := range []struct{ name, body, cfg string }{
		{"initial", "Init == x \\in {0,1}\nNext == UNCHANGED x\nInv == x=0", "INVARIANT Inv"},
		{"successor", "Init == x=0\nNext == x' \\in {0,1,2}\nInv == x=0", "INVARIANT Inv"},
		{"action", "Init == x=0\nNext == x' \\in {0,1}\nP == [][x'=x]_x", "PROPERTY P"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\n"+tc.body, "INIT Init\nNEXT Next\n"+tc.cfg+"\n")
			path := filepath.Join(dir, "go.dot")
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-workers", "1", "-fp", "0", "-continue", "-dump", "dot,actionlabels", path, filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 0 || errout.Len() != 0 {
				t.Fatalf("Go exit %d, stdout %s, stderr %s", code, out.String(), errout.String())
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-jar", jar, "-workers", "1", "-fp", "0", "-continue", "-dump", "dot,actionlabels", "java.dot", "M.tla")
				cmd.Dir = dir
				output, err := javatmp.Apply(cmd).CombinedOutput()
				if err != nil {
					t.Fatalf("Java TLC: %v\n%s", err, output)
				}
				want, err := os.ReadFile(filepath.Join(dir, "java.dot"))
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("DOT differs: %v\nGo %s\nJava %s", err, got, want)
				}
			}
		})
	}
}
