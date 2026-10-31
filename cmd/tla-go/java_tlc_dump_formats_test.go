package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func runDumpFormat(t *testing.T, dir, model, format, output string, code int, extra ...string) []byte {
	t.Helper()
	args := append(append([]string{"-deadlock", "-workers", "1", "-fp", "0"}, extra...), dumpArguments(format, output)...)
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

func compareJavaDumpFormat(t *testing.T, jar, dir, model, format string, got []byte, code int, extra ...string) {
	t.Helper()
	output := filepath.Join(dir, "java")
	args := append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-deadlock", "-workers", "1", "-fp", "0"}, extra...)
	args = append(args, dumpArguments(format, output)...)
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

// Java TLC writes an initial state before checking it, and exits without
// flushing its dump after an initial invariant violation.
func TestJavaTLCDumpInitialFailures(t *testing.T) {
	header := "strict digraph DiskGraph {\nnodesep=0.35;\nsubgraph cluster_graph {\ncolor=\"white\";\n"
	for _, tc := range []struct {
		name, invariant string
		want            map[string]string
		code            int
	}{
		{"invariant violation", "x # 1", map[string]string{"": "", "dot": header}, 12},
		{"evaluation error", "x < 1 \\/ 1[1] = 1", map[string]string{"": "State 1:\n/\\ x = 0\n/\\ y = 0\n\nState 2:\n/\\ x = 1\n/\\ y = 0\n\n"}, 75},
	} {
		for _, format := range []string{"", "dot"} {
			t.Run(tc.name+"/"+format, func(t *testing.T) {
				dir := t.TempDir()
				writeDOTFixture(t, dir, "VARIABLES x, y\nInit == x \\in 0..2 /\\ y = 0\nNext == UNCHANGED <<x, y>>\nInv == "+tc.invariant, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
				output := filepath.Join(dir, "go.dump")
				if format != "" {
					output = filepath.Join(dir, "go.dot")
				}
				got := runDumpFormat(t, dir, "M", format, output, tc.code)
				if want, exact := tc.want[format]; exact && string(got) != want {
					t.Fatalf("dump:\n%s\nwant:\n%s", got, want)
				}
				if jar := os.Getenv("TLC_JAR"); jar != "" {
					compareJavaDumpFormat(t, jar, dir, "M", format, got, tc.code)
				}
			})
		}
	}
}

// With VIEW, Java TLC fingerprints DOT nodes by the view value, and -view
// also prints dumped states through the view.
func TestJavaTLCDumpView(t *testing.T) {
	model := "VARIABLES x, y\nInit == x = 0 /\\ y = 0\nNext == x' = (x + 1) % 3 /\\ y' = (y + 1) % 2\nV == "
	for _, tc := range []struct {
		view, format, want string
		extra              []string
	}{
		{"<<x>>", "dot", `[label="/\\ x = 0\n/\\ y = 0",style = filled]`, nil},
		{"<<x>>", "dot", `[label="/\\ x = 0",style = filled]`, []string{"-view"}},
		{"x % 2", "", "State 1:\n0\nState 2:\n1\n", []string{"-view"}},
		{"<<x, y>>", "", "State 1:\n/\\ x = 0\n/\\ y = 0\n\n", []string{"-view"}},
	} {
		t.Run(tc.view+"/"+tc.format+strings.Join(tc.extra, ""), func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, model+tc.view, "INIT Init\nNEXT Next\nVIEW V\n")
			output := filepath.Join(dir, "go.dump")
			if tc.format != "" {
				output = filepath.Join(dir, "go.dot")
			}
			got := runDumpFormat(t, dir, "M", tc.format, output, 0, tc.extra...)
			if !strings.Contains(string(got), tc.want) {
				t.Fatalf("dump missing %q:\n%s", tc.want, got)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaDumpFormat(t, jar, dir, "M", tc.format, got, 0, tc.extra...)
			}
		})
	}
}
