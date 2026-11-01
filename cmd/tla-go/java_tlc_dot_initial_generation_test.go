package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// ETest4, ETest10, and ETest11 use these unmodified models from TLC commit
// 5a47802b5c391f59ecdd44117981f4ff8c0656ba. Their initial-state expressions
// fail before emitting a state. Java TLC 1.7.4 closes the empty DOT graph.
func TestJavaTLCDotInitialGenerationErrors(t *testing.T) {
	const complete = "strict digraph DiskGraph {\nnodesep=0.35;\nsubgraph cluster_graph {\ncolor=\"white\";\n}\n}"
	for _, model := range []string{"etest4", "etest10", "etest11"} {
		t.Run(model, func(t *testing.T) {
			dir := t.TempDir()
			for _, ext := range []string{".tla", ".cfg"} {
				data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "dot-initial-errors", model+ext))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, model+ext), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got := runDumpFormat(t, dir, model, "dot", filepath.Join(dir, "go.dot"), 75, "-checkpoint", "0")
			if !bytes.Equal(got, []byte(complete)) {
				t.Fatalf("DOT differs from Java TLC 1.7.4:\n%s", got)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaDumpFormat(t, jar, dir, model, "dot", got, 75, "-coverage", "1", "-checkpoint", "0")
			}
		})
	}
}

// ContinueTest overrides the upstream harness to use three workers. This is a
// one-worker control: the complete Java graph is deterministic at this setting.
func TestJavaTLCDotContinueSerial(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"Continue.tla", "Continue.cfg", "Continue-java.dot"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "dot-initial-errors", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(filepath.Join(dir, "Continue-java.dot"))
	if err != nil {
		t.Fatal(err)
	}
	got := runDumpFormat(t, dir, "Continue", "dot", filepath.Join(dir, "go.dot"), 0, "-checkpoint", "0", "-continue")
	if !bytes.Equal(got, want) {
		t.Fatalf("DOT differs from Java TLC 1.7.4:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaDumpFormat(t, jar, dir, "Continue", "dot", got, 0, "-coverage", "1", "-checkpoint", "0", "-continue")
	}
}

// A later generation error must retain and finish already emitted initial
// states; an initial invariant violation instead leaves the header unfinished
// (covered by TestJavaTLCDumpInitialFailures).
func TestJavaTLCDotPartialInitialGeneration(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x \\in {0, 1} /\\ (x = 0 \\/ 1[1] = 1)\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
	got := runDumpFormat(t, dir, "M", "dot", filepath.Join(dir, "go.dot"), 75)
	want := "strict digraph DiskGraph {\nnodesep=0.35;\nsubgraph cluster_graph {\ncolor=\"white\";\n" +
		"-3406341414084290173 [label=\"x = 0\",style = filled]\n" +
		"{rank = same; -3406341414084290173;}\n}\n}"
	if string(got) != want {
		t.Fatalf("partial DOT differs from Java TLC 1.7.4:\n%s", got)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaDumpFormat(t, jar, dir, "M", "dot", got, 75)
	}
}

// These cases exercise TLC's distinct initial-generation exit paths in both
// writers. The expected bytes were checked against the pinned 1.7.4 jar.
func TestJavaTLCDumpInitialExitPaths(t *testing.T) {
	const header = "strict digraph DiskGraph {\nnodesep=0.35;\nsubgraph cluster_graph {\ncolor=\"white\";\n"
	for _, tc := range []struct {
		name, definitions, config string
		code                      int
		text, dot                 string
	}{
		{"incomplete first", "VARIABLES x, y\nInit == x = 0\nNext == UNCHANGED <<x, y>>", "INIT Init\nNEXT Next\n", 255, "", header},
		{"incomplete after state", "VARIABLES x, y\nInit == (x = 0 /\\ y = 0) \\/ x = 1\nNext == UNCHANGED <<x, y>>", "INIT Init\nNEXT Next\n", 255, "", header},
		{"initial violation", "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nInv == x # 0", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12, "", header},
		{"initial exception", "VARIABLE x\nInit == x = 1[1]\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", 75, "", header + "}\n}"},
		{"constant evaluation", "CONSTANT C\nVARIABLE x\nBad == 1 \\div 0\nInit == x = 0\nNext == x' = C", "INIT Init\nNEXT Next\nCONSTANT C <- Bad\n", 151, "", header},
	} {
		for _, format := range []string{"", "dot"} {
			t.Run(tc.name+"/"+format, func(t *testing.T) {
				dir := t.TempDir()
				writeDOTFixture(t, dir, tc.definitions, tc.config)
				output := filepath.Join(dir, "go.dump")
				if format == "dot" {
					output = filepath.Join(dir, "go.dot")
				}
				got := runDumpFormat(t, dir, "M", format, output, tc.code)
				want := tc.text
				if format == "dot" {
					want = tc.dot
				}
				if string(got) != want {
					t.Fatalf("dump = %q, want %q", got, want)
				}
				if jar := os.Getenv("TLC_JAR"); jar != "" {
					compareJavaDumpFormat(t, jar, dir, "M", format, got, tc.code)
				}
			})
		}
	}
}
