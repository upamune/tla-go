package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestJavaTLCSimulationTool(t *testing.T) {
	cases := []struct {
		name, body, cfg string
		code            int
		want            []int
	}{
		{"success", "Init == x = 0\nNext == x' \\in {0, 1, 2}", "INIT Init\nNEXT Next\n", 0, []int{2269, 2209}},
		{"empty initial", "Init == x \\in {}\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", 255, []int{2269, 2118}},
		{"implied initial property", "Init == x = 0\nNext == UNCHANGED x\nP == x = 1", "INIT Init\nNEXT Next\nPROPERTY P\n", 0, []int{2269, 2209}},
		{"initial invariant", "Init == x \\in {0, 1}\nNext == UNCHANGED x\nInv == x = 0", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12, []int{2269, 2107}},
		{"successor invariant", "Init == x = 0\nNext == x' \\in {0, 1, 2}\nInv == x # 2", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12, []int{2269, 2110, 2121, 2217, 2209, 2210}},
		{"invariant after stuttering", "Init == x=0\nNext == UNCHANGED x\nInv == TLCGet(\"level\")=0", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12, []int{2269, 2110, 2121, 2217, 2209, 2210}},
		{"deadlock", "Init == x = 0\nNext == /\\ x = 0 /\\ x' = 1", "INIT Init\nNEXT Next\n", 11, []int{2269, 2114, 2121, 2217, 2209, 2210}},
		{"action evaluation after stutter", "Init == x=0\nNext == x'=1\nP == [][IF x=0 THEN TRUE ELSE 0=\"a\"]_x", "INIT Init\nNEXT Next\nPROPERTY P\n", 13, []int{2269, 2113, 2121, 2217, 2209, 2210}},
		{"constraint", "Init == x = 0\nNext == x' \\in {0, 1, 2}\nBound == x # 2", "INIT Init\nNEXT Next\nCONSTRAINT Bound\n", 0, []int{2269, 2209}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\n"+tc.body, tc.cfg)
			var out, errout bytes.Buffer
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "1", "-depth", "5", filepath.Join(dir, "M.tla")}
			code := run(context.Background(), args, &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("Go exit %d, stderr %s; output:\n%s", code, errout.String(), out.String())
			}
			got := readToolMessages(out.String())
			for _, want := range tc.want {
				if !hasToolMessage(got, want) {
					t.Errorf("missing message %d:\n%s", want, out.String())
				}
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				want := javaSimulationTool(t, jar, dir, tc.code)
				if a, b := simulationMessages(got), simulationMessages(want); !reflect.DeepEqual(a, b) {
					t.Fatalf("Go messages %q; Java messages %q", a, b)
				}
			}
		})
	}
}

func javaSimulationTool(t *testing.T, jar, dir string, code int) []toolMessage {
	return javaSimulationToolSeed(t, jar, dir, code, "1")
}

func javaSimulationToolSeed(t *testing.T, jar, dir string, code int, seed string) []toolMessage {
	t.Helper()
	args := []string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", seed, "-depth", "5", "M.tla"}
	cmd := exec.Command("java", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
		t.Fatalf("Java TLC: %v\n%s", err, output)
	}
	return readToolMessages(string(output))
}

func simulationMessages(messages []toolMessage) []string {
	var result []string
	for _, msg := range messages {
		switch msg.code {
		case 2102, 2106, 2107, 2109, 2110, 2111, 2112, 2113, 2114, 2116, 2118, 2122, 2218, 2269:
			result = append(result, fmt.Sprintf("%d:%s", msg.code, msg.body))
		case 2217:
			result = append(result, fmt.Sprintf("2217:%s", msg.body))
		case 2209:
			if match := regexp.MustCompile(`([0-9]+) states generated`).FindStringSubmatch(msg.body); len(match) > 0 {
				result = append(result, "2209:"+match[1])
			}
		}
	}
	return result
}

func TestSimulationRejectsUnsupportedLiveness(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == UNCHANGED x\nProp == []<>(x=0) \\/ <>[](TLCGet(\"level\") > 1)", "INIT Init\nNEXT Next\nPROPERTY Prop\n")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-simulate", "num=1", "-seed", "1", filepath.Join(dir, "M.tla")}, &out, &errout)
	if code == 0 || !strings.Contains(out.String(), "using TLCGet is not yet supported") {
		t.Fatalf("exit %d, stdout %s, stderr %s", code, out.String(), errout.String())
	}
}

func TestJavaTLCSimulationRandomSubset(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC, Randomization
VARIABLES x, step
Init == /\ x = {} /\ step = 0
Next == /\ step < 2 /\ x' = RandomSubset(3, 1..20) /\ step' = step + 1
Inv == step < 2
====
`
	const cfg = "INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n"
	for _, seed := range []string{"1", "0", "-42"} {
		t.Run(seed, func(t *testing.T) {
			dir := t.TempDir()
			for name, text := range map[string]string{"M.tla": source, "M.cfg": cfg} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", seed, "-depth", "5", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 12 || errout.Len() != 0 {
				t.Fatalf("Go exit %d, stderr %s, stdout %s", code, errout.String(), out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				want := javaSimulationToolSeed(t, jar, dir, code, seed)
				if a, b := simulationMessages(readToolMessages(out.String())), simulationMessages(want); !reflect.DeepEqual(a, b) {
					t.Fatalf("Go messages %q; Java messages %q", a, b)
				}
			}
		})
	}
}

func TestJavaTLCSimulationUpstreamTool(t *testing.T) {
	const sourceDir = "testdata/java-tlc/multiple-config/multi-trace"
	source, err := os.ReadFile(filepath.Join(sourceDir, "BasicMultiTrace.tla"))
	if err != nil {
		t.Fatal(err)
	}
	model := strings.Replace(string(source), "MODULE BasicMultiTrace", "MODULE M", 1)
	cases := []struct {
		name string
		code int
	}{
		{"MC", 0},
		{"MCInv", 12},
		{"MCInvInitState", 12},
		{"MCActionProp", 13},
		{"MCWithConstraint", 0},
		{"MCWithActionConstraint", 0},
		{"MCBadInvInitState", 255},
		{"MCBadInvNonInitState", 76},
		{"MCActionPropBadEval", 13},
		{"MCUnderspecInit", 255},
		{"MCUnderspecNext", 255},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := os.ReadFile(filepath.Join(sourceDir, tc.name+".cfg"))
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			for name, text := range map[string]string{"M.tla": model, "M.cfg": string(cfg) + "\nCHECK_DEADLOCK FALSE\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "1", "-depth", "5", filepath.Join(dir, "M.tla")}
			code := run(context.Background(), args, &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("Go exit %d, stderr %s, stdout %s", code, errout.String(), out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				got := simulationMessages(readToolMessages(out.String()))
				want := simulationMessages(javaSimulationTool(t, jar, dir, code))
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("Go messages %q\nJava messages %q", got, want)
				}
			}
		})
	}
}

func TestJavaTLCSimulationMultipleTraces(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x' \\in {0, 1, 2}", "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n")
	var out, errout bytes.Buffer
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=3", "-seed", "1", "-depth", "4", filepath.Join(dir, "M.tla")}
	code := run(context.Background(), args, &out, &errout)
	if code != 0 || !strings.Contains(out.String(), "37 states generated") {
		t.Fatalf("Go exit %d, stderr %s, stdout %s", code, errout.String(), out.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "-simulate", "num=3", "-seed", "1", "-depth", "4", "M.tla")
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(output), "37 states generated") {
			t.Fatalf("Java: %v\n%s", err, output)
		}
	}
}

func TestJavaTLCSimulationPauseDoesNotWait(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == /\\ TLCSet(\"pause\", TRUE) /\\ UNCHANGED x", "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n")
	var out, errout bytes.Buffer
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "1", "-depth", "5", filepath.Join(dir, "M.tla")}
	code := runWithInput(context.Background(), args, strings.NewReader(""), &out, &errout)
	if code != 0 || strings.Contains(out.String(), "Press enter") {
		t.Fatalf("Go exit %d; stdout %s; stderr %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		want := javaSimulationTool(t, jar, dir, code)
		if a, b := simulationMessages(readToolMessages(out.String())), simulationMessages(want); !reflect.DeepEqual(a, b) {
			t.Fatalf("Go messages %q; Java messages %q", a, b)
		}
	}
}

func TestSimulationStopsOnExit(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == /\\ TLCSet(\"exit\", TRUE) /\\ UNCHANGED x", "INIT Init\nNEXT Next\n")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-simulate", "num=1", "-seed", "1", filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), "2 states generated") {
		t.Fatalf("exit %d; stdout %s; stderr %s", code, out.String(), errout.String())
	}
}
