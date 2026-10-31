package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestJavaTLCSimulationLiveness(t *testing.T) {
	cases := []struct {
		name, body, cfg string
		code            int
	}{
		{"violated at initial", "Init == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=0\nP == <>(x=1)", "INIT Init\nNEXT Next\nPROPERTY P\n", 13},
		{"satisfied at initial", "Init == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=0\nP == <>(x=0)", "INIT Init\nNEXT Next\nPROPERTY P\n", 0},
		{"never satisfied", "Init == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=0\nP == <>(x=2)", "INIT Init\nNEXT Next\nPROPERTY P\n", 13},
		{"constrained trace", "Init == x=0\nNext == x'=1\nBound == x=0\nP == <>(x=1)", "INIT Init\nNEXT Next\nCONSTRAINT Bound\nPROPERTY P\n", 13},
		{"deadlock without checking", "Init == x=0\nNext == /\\ x=0 /\\ x'=1\nP == <>(x=2)", "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\nPROPERTY P\n", 13},
		{"recurrence toggle zero", "Init == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=0\nP == []<>(x=0)", "INIT Init\nNEXT Next\nPROPERTY P\n", 13},
		{"recurrence toggle one", "Init == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=0\nP == []<>(x=1)", "INIT Init\nNEXT Next\nPROPERTY P\n", 13},
		{"recurrence stays zero", "Init == x=0\nNext == x'=0\nP == []<>(x=0)", "INIT Init\nNEXT Next\nPROPERTY P\n", 0},
		{"recurrence misses one", "Init == x=0\nNext == x'=0\nP == []<>(x=1)", "INIT Init\nNEXT Next\nPROPERTY P\n", 13},
		{"recurrence moves to one", "Init == x=0\nNext == x'=1\nP == []<>(x=0)", "INIT Init\nNEXT Next\nPROPERTY P\n", 13},
		{"recurrence starts at zero", "Init == x=0\nNext == x'=1\nP == []<>(x=1)", "INIT Init\nNEXT Next\nPROPERTY P\n", 13},
		{"stability toggle zero", "Init == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=0\nP == <>[](x=0)", "INIT Init\nNEXT Next\nPROPERTY P\n", 0},
		{"stability toggle one", "Init == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=0\nP == <>[](x=1)", "INIT Init\nNEXT Next\nPROPERTY P\n", 13},
		{"stability stays zero", "Init == x=0\nNext == x'=0\nP == <>[](x=0)", "INIT Init\nNEXT Next\nPROPERTY P\n", 0},
		{"stability misses one", "Init == x=0\nNext == x'=0\nP == <>[](x=1)", "INIT Init\nNEXT Next\nPROPERTY P\n", 13},
		{"stability moves to one", "Init == x=0\nNext == x'=1\nP == <>[](x=0)", "INIT Init\nNEXT Next\nPROPERTY P\n", 0},
		{"stability starts at zero", "Init == x=0\nNext == x'=1\nP == <>[](x=1)", "INIT Init\nNEXT Next\nPROPERTY P\n", 13},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\n"+tc.body, tc.cfg)
			var out, errout bytes.Buffer
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "1", "-depth", "5", filepath.Join(dir, "M.tla")}
			code := run(context.Background(), args, &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("Go exit %d; stdout %s; stderr %s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				got := livenessMessages(readToolMessages(out.String()))
				want := livenessMessages(javaSimulationTool(t, jar, dir, code))
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("Go messages %q\nJava messages %q", got, want)
				}
			}
		})
	}
}

func TestJavaTLCSimulationRecurrenceGraph(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, mode := range []string{"[]<>", "<>[]"} {
		for _, next := range []string{
			"IF x=0 THEN x'=1 ELSE IF x=1 THEN x'=2 ELSE x'=0",
			"IF x=0 THEN x'=1 ELSE x'=2",
			"IF x=0 THEN x'=1 ELSE IF x=1 THEN x'=2 ELSE IF x=2 THEN x'=3 ELSE x'=4",
			"x' \\in {0, 1, 2}",
		} {
			for n := range 3 {
				for _, seed := range []string{"1", "2", "3"} {
					t.Run(fmt.Sprintf("%s/%s/%d/%s", mode, next, n, seed), func(t *testing.T) {
						body := fmt.Sprintf("VARIABLE x\nInit == x=0\nNext == %s\nP == %s(x=%d)", next, mode, n)
						compareSimulationGraph(t, jar, body, seed)
					})
				}
			}
		}
	}
}

func compareSimulationGraph(t *testing.T, jar, body, seed string) {
	t.Helper()
	compareSimulationGraphConfig(t, jar, body, "INIT Init\nNEXT Next\nPROPERTY P\n", seed)
}

func TestJavaTLCSimulationMultipleTemporalProperties(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, pair := range [][2]string{
		{"<>[](x=1)", "[]<>(x=1)"},
		{"[]<>(x=1)", "<>[](x=1)"},
		{"<>(x=1)", "<>[](x=1)"},
		{"<>[](x=1)", "<>(x=1)"},
		{"[]<>(x=1)", "<>(x=1)"},
		{"<>[](x=0)", "[]<>(x=1)"},
	} {
		t.Run(pair[0]+"/"+pair[1], func(t *testing.T) {
			body := fmt.Sprintf("VARIABLE x\nInit == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=2\nP == %s\nQ == %s", pair[0], pair[1])
			compareSimulationGraphConfig(t, jar, body, "INIT Init\nNEXT Next\nPROPERTY P Q\n", "1")
		})
	}
	for _, first := range []string{"<>", "[]<>", "<>[]"} {
		for _, second := range []string{"<>", "[]<>", "<>[]"} {
			for _, next := range []string{"IF x=0 THEN x'=1 ELSE x'=2", "IF x=0 THEN x'=1 ELSE IF x=1 THEN x'=2 ELSE x'=0"} {
				t.Run(first+"/"+second+"/"+next, func(t *testing.T) {
					body := fmt.Sprintf("VARIABLE x\nInit == x=0\nNext == %s\nP == %s(x=1)\nQ == %s(x=2)", next, first, second)
					compareSimulationGraphConfig(t, jar, body, "INIT Init\nNEXT Next\nPROPERTY P Q\n", "1")
				})
			}
			for _, pair := range [][2]int{{0, 1}, {1, 0}, {2, 0}} {
				t.Run(fmt.Sprintf("%s/%s/%d/%d", first, second, pair[0], pair[1]), func(t *testing.T) {
					body := fmt.Sprintf("VARIABLE x\nInit == x=0\nNext == IF x=0 THEN x'=1 ELSE x'=2\nP == %s(x=%d)\nQ == %s(x=%d)", first, pair[0], second, pair[1])
					compareSimulationGraphConfig(t, jar, body, "INIT Init\nNEXT Next\nPROPERTY P Q\n", "1")
				})
			}
		}
	}
}

func TestJavaTLCModernSimulationStabilityVersionDifference(t *testing.T) {
	jar := os.Getenv("TLC_MODERN_JAR")
	if jar == "" {
		t.Skip("TLC_MODERN_JAR is not set")
	}
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == x' \\in {0, 1, 2}\nP == <>[](x=0)", "INIT Init\nNEXT Next\nPROPERTY P\n")
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "2", "-depth", "5"}
	cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
	cmd.Dir = dir
	output, _ := cmd.CombinedOutput()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 13 {
		t.Fatalf("Java TLC 1.8.0: %s", output)
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	if code != 0 || errout.Len() != 0 {
		t.Fatalf("tla-go exit %d; stdout %s; stderr %s", code, out.String(), errout.String())
	}
}

func compareSimulationGraphConfig(t *testing.T, jar, body, cfg, seed string) {
	compareSimulationGraphDepth(t, jar, body, cfg, seed, "5")
}

func compareSimulationGraphDepth(t *testing.T, jar, body, cfg, seed, depth string) {
	compareSimulationGraphParameters(t, jar, body, cfg, seed, depth, "1")
}

func compareSimulationGraphParameters(t *testing.T, jar, body, cfg, seed, depth, traces string) {
	t.Helper()
	dir := t.TempDir()
	writeDOTFixture(t, dir, body, cfg)
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=" + traces, "-seed", seed, "-depth", depth}
	cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
	cmd.Dir = dir
	javaOut, _ := cmd.CombinedOutput()
	if cmd.ProcessState == nil || !slices.Contains([]int{0, 13, 255}, cmd.ProcessState.ExitCode()) {
		t.Fatalf("Java TLC failed: %s", javaOut)
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	if code != cmd.ProcessState.ExitCode() || errout.Len() != 0 {
		t.Fatalf("Go exit %d, Java exit %d; stdout %s; stderr %s", code, cmd.ProcessState.ExitCode(), out.String(), errout.String())
	}
	got, want := comparableProgress(livenessMessages(readToolMessages(out.String())), livenessMessages(readToolMessages(string(javaOut))))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Go messages %q\nJava messages %q", got, want)
	}
}
