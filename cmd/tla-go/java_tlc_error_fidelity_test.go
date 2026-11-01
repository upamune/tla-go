package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
	"github.com/upamune/tla-go/internal/upstreamjunit"
)

// These original models are from TLA+ Tools 5a47802b5c391f59ecdd44117981f4ff8c0656ba.
func TestJavaTLCErrorFidelityUpstream(t *testing.T) {
	for _, tc := range []struct {
		name string
		exit int
		ids  []string
	}{
		{"etest7", 151, []string{"2281", "2280"}},
		{"Github179a", 10, []string{"2105", "2154"}},
		{"Github179b", 75, []string{"1000", "2154", "2103"}},
		{"Github179c", 75, []string{"1000", "2154", "2103", "2121", "2217"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, ext := range []string{".tla", ".cfg"} {
				data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", tc.name+ext))
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(dir, tc.name+ext), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			compareErrorFidelity(t, dir, tc.name, tc.exit, tc.ids)
		})
	}
}

// Generated variants probe other source positions and values, plus eager
// argument failures and successful printing where no override error is raised.
func TestJavaTLCErrorFidelityVariants(t *testing.T) {
	for _, tc := range []struct {
		name, definitions, config string
		exit                      int
		ids                       []string
	}{
		{"unbound-second-variable", "CONSTANT C\nVARIABLES x, y\nFoo == y\nInit == /\\ x = 0 /\\ y = 0\nNext == UNCHANGED <<x,y>>", "INIT Init\nNEXT Next\nCONSTANT C <- Foo\n", 151, []string{"2281", "2280"}},
		{"choose-failure", "CONSTANT C\nVARIABLE x\nFoo == CHOOSE n \\in {} : TRUE\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\nCONSTANT C <- Foo\n", 151, []string{"2281", "!2280"}},
		{"eager-failure", "VARIABLE x\nInit == x = 0\nNext == PrintT(1 = {1}) /\\ UNCHANGED x", "INIT Init\nNEXT Next\n", 75, []string{"1000", "2103", "2121", "2217", "!2154"}},
		{"print-success", "VARIABLE x\nInit == x = 0\nNext == PrintT({1,2}) /\\ UNCHANGED x", "INIT Init\nNEXT Next\n", 0, []string{"!2154", "!2281"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\n" + tc.definitions + "\n====\n"
			for name, data := range map[string]string{"M.tla": source, "M.cfg": tc.config} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			compareErrorFidelity(t, dir, "M", tc.exit, tc.ids)
		})
	}
}

func TestJavaTLCDeferredPrintValueTypes(t *testing.T) {
	original, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "Github179c.tla"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, expression string }{{"boolean", "TRUE"}, {"string", `"a"`}, {"function", "[n \\in {1} |-> n]"}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := strings.Replace(string(original), "x = Max(s)", "x = "+tc.expression, 1)
			for name, data := range map[string]string{"Github179c.tla": source, "Github179c.cfg": "INIT Init\nNEXT Next\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			compareErrorFidelity(t, dir, "Github179c", 75, []string{"1000", "2154", "2103", "2121", "2217"})
		})
	}
}

func compareErrorFidelity(t *testing.T, dir, name string, wantExit int, ids []string) {
	t.Helper()
	args := []string{"-tool", "-deadlock", "-fp", "0", "-workers", "1", "-checkpoint", "0"}
	var out, stderr bytes.Buffer
	code := run(context.Background(), append(append([]string{}, args...), filepath.Join(dir, name+".tla")), &out, &stderr)
	if code != wantExit || stderr.Len() != 0 {
		t.Fatalf("Go exit %d stderr %s output %s", code, stderr.String(), out.String())
	}
	got, err := upstreamjunit.Messages(out.String())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if after, ok := strings.CutPrefix(id, "!"); ok {
			if len(got[after]) != 0 {
				t.Fatalf("unexpected %s in %s", id, out.String())
			}
			continue
		}
		if len(got[id]) == 0 {
			t.Fatalf("missing %s in %s", id, out.String())
		}
	}
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		return
	}
	data, err := os.ReadFile(jar)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != upstreamjunit.JarSHA256 {
		t.Fatalf("TLC_JAR must be pinned 1.7.4: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	javaArgs := append([]string{"-Xmx256m", "-XX:+UseParallelGC", "-jar", jar}, args...)
	javaArgs = append(javaArgs, name+".tla")
	cmd := exec.CommandContext(ctx, "java", append([]string{"-Djava.io.tmpdir=" + t.TempDir()}, javaArgs...)...)
	cmd.Dir = dir
	output, err := javatmp.Apply(cmd).CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != wantExit {
		t.Fatalf("Java %v: %s", err, output)
	}
	want, err := upstreamjunit.Messages(string(output))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		id = strings.TrimPrefix(id, "!")
		if !reflect.DeepEqual(got[id], want[id]) {
			t.Errorf("message %s\nGo: %q\nJava: %q", id, got[id], want[id])
		}
	}
}
