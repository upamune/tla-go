package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The expected files are the masters of the upstream DumpAsDotTest and
// Github407Test, which Java TLC 1.7.4 reproduces byte for byte.
func TestJavaTLCUpstreamDumpFormats(t *testing.T) {
	for _, tc := range []struct {
		name, model, format, expected string
		code                          int
	}{
		{"colorized DOT", "MCa", "dot,colorize,actionlabels", "DumpAsDotTest.dot", 13},
		{"state dump", "Github407", "", "Github407.dump", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := copyDumpFixtures(t)
			want, err := os.ReadFile(filepath.Join(dir, tc.expected))
			if err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(dir, "go"+filepath.Ext(tc.expected))
			got := runDumpFormat(t, dir, tc.model, tc.format, output, tc.code)
			if !bytes.Equal(got, want) {
				t.Fatalf("dump differs from the upstream master\ngot:\n%s\nwant:\n%s", got, want)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaDumpFormat(t, jar, dir, tc.model, tc.format, got, tc.code)
			}
		})
	}
}

// Java TLC also writes a plain-dump state again for each stuttering step of a
// liveness check, and numbers colors by the first appearance of each action.
func TestJavaTLCDumpFormatVariants(t *testing.T) {
	model := "VARIABLES x, y\nInit == x \\in {0, 1} /\\ y = \"a\"\nA == x < 3 /\\ x' = x + 1 /\\ UNCHANGED y\n" +
		"B == y' = \"b\" /\\ UNCHANGED x\nzeta == x = 2 /\\ x' = 0 /\\ UNCHANGED y\nNext == A \\/ B \\/ zeta\nCon == x < 3\nProp == <>(x = 2)"
	for _, format := range []string{"", "dot", "dot,colorize", "dot,ActionLabels,Colorize"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, model, "INIT Init\nNEXT Next\nCONSTRAINT Con\nPROPERTY Prop\n")
			output := filepath.Join(dir, "go.dump")
			if format != "" {
				output = filepath.Join(dir, "go.dot")
			}
			got := runDumpFormat(t, dir, "M", format, output, 13)
			if len(got) == 0 {
				t.Fatal("empty dump")
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaDumpFormat(t, jar, dir, "M", format, got, 13)
			}
		})
	}
}

func copyDumpFixtures(t *testing.T) string {
	t.Helper()
	source := filepath.Join("testdata", "java-tlc", "dump")
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func dumpArguments(format, output string) []string {
	if format == "" {
		return []string{"-dump", output}
	}
	return []string{"-dump", format, output}
}

func runDumpFormat(t *testing.T, dir, model, format, output string, code int) []byte {
	t.Helper()
	args := append([]string{"-deadlock", "-workers", "1", "-fp", "0"}, dumpArguments(format, output)...)
	var out, errout bytes.Buffer
	if got := run(context.Background(), append(args, filepath.Join(dir, model+".tla")), &out, &errout); got != code || errout.Len() != 0 {
		t.Fatalf("exit %d, want %d; stdout:\n%s\nstderr:\n%s", got, code, out.String(), errout.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func compareJavaDumpFormat(t *testing.T, jar, dir, model, format string, got []byte, code int) {
	t.Helper()
	output := filepath.Join(dir, "java")
	args := append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-deadlock", "-workers", "1", "-fp", "0"}, dumpArguments(format, output)...)
	cmd := exec.Command("java", append(args, model+".tla")...)
	cmd.Dir = dir
	if out, _ := cmd.CombinedOutput(); cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
		t.Fatalf("Java exit %v, want %d:\n%s", cmd.ProcessState, code, out)
	}
	suffix := ".dump"
	if format != "" {
		suffix = ".dot"
	}
	want, err := os.ReadFile(output + suffix)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("dump differs from Java TLC\ngot:\n%s\nwant:\n%s", got, want)
	}
}
