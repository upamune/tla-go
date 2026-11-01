package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The inputs are unchanged test-model files from TLC commit 5a47802b5c391f59ecdd44117981f4ff8c0656ba.
// Github798's embedded CONFIG is not selected by the upstream -config argument:
// TLC looks for a nonexistent .tla.cfg file. The expected bytes were captured
// with the pinned 1.7.4 jar and the sweep's one-worker arguments.
func TestJavaTLCDotStartupErrors(t *testing.T) {
	const header = "strict digraph DiskGraph {\nnodesep=0.35;\nsubgraph cluster_graph {\ncolor=\"white\";\n"
	for _, tc := range []struct {
		model  string
		code   int
		config bool
	}{
		{"Github179a", 10, true},
		{"Github798I", 255, false},
		{"Github798N", 255, false},
		{"etest12", 10, true},
		{"etest13", 10, true},
		{"etest14", 10, true},
		{"etest15", 10, true},
	} {
		t.Run(tc.model, func(t *testing.T) {
			dir := t.TempDir()
			extensions := []string{".tla"}
			if tc.config {
				extensions = append(extensions, ".cfg")
			}
			for _, ext := range extensions {
				name := tc.model + ext
				data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			extra := []string{"-tool", "-checkpoint", "0"}
			if !tc.config {
				extra = append(extra, "-config", tc.model+".tla")
			}
			got := runDumpFormat(t, dir, tc.model, "dot", filepath.Join(dir, "go.dot"), tc.code, extra...)
			if string(got) != header {
				t.Fatalf("DOT = %q, want %q", got, header)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaDumpFormat(t, jar, dir, tc.model, "dot", got, tc.code, append(extra, "-coverage", "1")...)
			}
		})
	}
}

// ETest6 fails while generating successors, after one initial state was
// written. Unlike a startup failure, TLC finishes this graph.
func TestJavaTLCDotSuccessorErrorFixture(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"etest6.tla", "etest6.cfg", "etest6-java.dot"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(filepath.Join(dir, "etest6-java.dot"))
	if err != nil {
		t.Fatal(err)
	}
	extra := []string{"-tool", "-checkpoint", "0"}
	got := runDumpFormat(t, dir, "etest6", "dot", filepath.Join(dir, "go.dot"), 75, extra...)
	if !bytes.Equal(got, want) {
		t.Fatalf("DOT = %q, want %q", got, want)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaDumpFormat(t, jar, dir, "etest6", "dot", got, 75, append(extra, "-coverage", "1")...)
	}
}

func TestJavaTLCDotAssumptionExitOptions(t *testing.T) {
	for _, assumption := range []struct {
		expression string
		code       int
	}{{"FALSE", 10}, {"TRUE", 0}} {
		for _, format := range []string{"dot", "dot,actionlabels", "dot,colorize,actionlabels"} {
			t.Run(assumption.expression+"/"+format, func(t *testing.T) {
				dir := t.TempDir()
				writeDOTFixture(t, dir, "VARIABLE x\nASSUME "+assumption.expression+"\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
				got := runDumpFormat(t, dir, "M", format, filepath.Join(dir, "go.dot"), assumption.code, "-tool")
				if jar := os.Getenv("TLC_JAR"); jar != "" {
					compareJavaDumpFormat(t, jar, dir, "M", format, got, assumption.code, "-tool")
				}
			})
		}
	}
}
