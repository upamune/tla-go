package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TLC 1.7.4 at 5a47802b5c391f59ecdd44117981f4ff8c0656ba. The state
// contains intervals of different lengths, equal lengths and empty intervals.
// Its graph pins the sorted state representation and fp 0 fingerprint even
// when the Java oracle is not installed.
func TestJavaTLCIntervalOrderingDOT(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar != "" {
		var err error
		jar, err = filepath.Abs(jar)
		if err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	const source = `---- MODULE M ----
EXTENDS Integers
VARIABLE x
Init == x = {1..10000, 0..0, 3..2, 5..4, (-2)..1, 3..6}
Next == UNCHANGED x
====
`
	for name, contents := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	goPath := filepath.Join(dir, "go.dot")
	var stdout, stderr bytes.Buffer
	goCode := run(context.Background(), []string{"-tool", "-fp", "0", "-workers", "1", "-dump", "dot", goPath, filepath.Join(dir, "M.tla")}, &stdout, &stderr)
	if goCode != 0 || stderr.Len() != 0 {
		t.Fatalf("Go exit %d, stderr %s, stdout %s", goCode, stderr.String(), stdout.String())
	}
	assertIntervalGraphStats(t, stdout.String())
	graph, err := os.ReadFile(goPath)
	if err != nil {
		t.Fatal(err)
	}
	const golden = `strict digraph DiskGraph {
nodesep=0.35;
subgraph cluster_graph {
color="white";
-6195011634579894739 [label="x = {{}, 0..0, -2..1, 3..6, 1..10000}",style = filled]
-6195011634579894739 -> -6195011634579894739 [label="",color="black",fontcolor="black"];
{rank = same; -6195011634579894739;}
}
}`
	if string(graph) != golden {
		t.Fatalf("Go DOT differs from the nonempty pinned Java fixture:\n%s", graph)
	}
	if jar == "" {
		return
	}
	javaOutput, javaCode := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-fp", "0", "-workers", "1", "-dump", "dot", "java.dot", "M.tla"})
	if javaCode != 0 {
		t.Fatalf("Java exit %d: %s", javaCode, javaOutput)
	}
	assertIntervalGraphStats(t, javaOutput)
	javaGraph, err := os.ReadFile(filepath.Join(dir, "java.dot"))
	if err != nil || !bytes.Equal(javaGraph, graph) {
		t.Fatalf("Java DOT differs: %v\n%s", err, javaGraph)
	}
}

func assertIntervalGraphStats(t *testing.T, output string) {
	t.Helper()
	const expected = "2 states generated, 1 distinct states found, 0 states left on queue."
	var stats []string
	for _, message := range readToolMessages(output) {
		if message.code == 2199 {
			stats = append(stats, message.body)
		}
	}
	if len(stats) != 1 || stats[0] != expected {
		t.Fatalf("expected completed one-state exploration, got statistics %q in %s", stats, output)
	}
}
