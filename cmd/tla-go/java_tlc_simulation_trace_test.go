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

func TestJavaTLCSimulationTraceSemantics(t *testing.T) {
	cases := []struct {
		name, next, extra, cfg string
		generated              int
	}{
		{"unchanged constraint", "x' = x", "Bound == Len(Trace) = 1", "CONSTRAINT Bound", 11},
		{"changing constraint", "x' = x + 1", "Bound == Len(Trace) = 1", "CONSTRAINT Bound", 3},
		{"unchanged action", "/\\ Len(Trace) <= 2 /\\ x' = x", "", "", 11},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals, Sequences, TLCExt\nVARIABLE x\nInit == x = 0\nNext == " + tc.next + "\n" + tc.extra + "\n====\n"
			cfg := "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n" + tc.cfg + "\n"
			dir := writeSimulationFixture(t, source, cfg)
			var out, errout bytes.Buffer
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=2", "-seed", "1", "-depth", "5", filepath.Join(dir, "M.tla")}
			code := run(context.Background(), args, &out, &errout)
			want := fmt.Sprintf("%d states generated", tc.generated)
			if code != 0 || !strings.Contains(out.String(), want) {
				t.Fatalf("Go exit %d; stdout %s; stderr %s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
				output := javaModernSimulation(t, jar, dir, "2", "5")
				if !strings.Contains(output, want) {
					t.Fatalf("Java output lacks %q:\n%s", want, output)
				}
			}
		})
	}
}

func TestJavaTLCSimulationTraceStuttering(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, Sequences, TLC, TLCExt
VARIABLE x
Init == x = 0
Next == /\ PrintT(<<x, Len(Trace)>>) /\ x' \in {0, 1}
====
`
	dir := writeSimulationFixture(t, source, "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n")
	var out, errout bytes.Buffer
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "1", "-depth", "8", filepath.Join(dir, "M.tla")}
	code := run(context.Background(), args, &out, &errout)
	if code != 0 {
		t.Fatalf("Go exit %d; stdout %s; stderr %s", code, out.String(), errout.String())
	}
	pattern := regexp.MustCompile(`(?m)^<<[^\r\n]+>>$`)
	got := pattern.FindAllString(out.String(), -1)
	want := []string{"<<0, 1>>", "<<1, 2>>", "<<1, 2>>", "<<0, 3>>", "<<1, 4>>", "<<1, 4>>", "<<1, 4>>", "<<0, 5>>"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Go output %q; want %q", got, want)
	}
	if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
		java := pattern.FindAllString(javaModernSimulation(t, jar, dir, "1", "8"), -1)
		if !reflect.DeepEqual(got, java) {
			t.Fatalf("Go output %q; Java output %q", got, java)
		}
	}
}

func writeSimulationFixture(t *testing.T, source, cfg string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range map[string]string{"M.tla": source, "M.cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func javaModernSimulation(t *testing.T, jar, dir, num, depth string) string {
	t.Helper()
	cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "-simulate", "num="+num, "-seed", "1", "-depth", depth, "M.tla")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Java TLC: %v\n%s", err, output)
	}
	return string(output)
}
