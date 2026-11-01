package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func parseGraph(text string) ([]string, error) {
	g := graph{labels: map[string]string{}, ids: map[string]string{}, initial: map[string]bool{}}
	for line := range strings.SplitSeq(text, "\n") {
		if err := g.consume(strings.TrimSpace(line)); err != nil {
			return nil, err
		}
	}
	lines, err := g.lines()
	sort.Strings(lines)
	return lines, err
}

func TestCapturedGraphsCanonicalize(t *testing.T) {
	dir := filepath.Join("..", "testdata", "continue-51c3a77")
	want, err := os.ReadFile(filepath.Join(dir, "graph.canonical"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"java-01.dot", "java-02.dot", "go-51.dot"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			got, err := parseGraph(string(data))
			if err != nil || strings.Join(got, "\n")+"\n" != string(want) {
				t.Fatalf("canonical output differs: %v", err)
			}
		})
	}
}

func TestRejectMalformedOrTruncatedGraphs(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "continue-51c3a77", "go-51.dot"))
	if err != nil {
		t.Fatal(err)
	}
	original := string(data)
	trimmed := strings.TrimRight(original, " \t\r\n")
	oneClosure := strings.TrimSuffix(trimmed, "}")
	twoClosures := strings.TrimSuffix(strings.TrimRight(oneClosure, " \t\r\n"), "}")
	for _, tc := range []struct{ name, text string }{
		{"one missing closure", oneClosure},
		{"both closures missing", twoClosures},
		{"missing graph header", strings.Replace(original, "strict digraph DiskGraph {\n", "", 1)},
		{"repeated graph header", strings.Replace(original, "strict digraph DiskGraph {\n", "strict digraph DiskGraph {\nstrict digraph DiskGraph {\n", 1)},
		{"missing cluster", strings.Replace(original, "subgraph cluster_graph {\n", "", 1)},
		{"repeated cluster", strings.Replace(original, "subgraph cluster_graph {\n", "subgraph cluster_graph {\nsubgraph cluster_graph {\n", 1)},
		{"premature closure", strings.Replace(original, `color="white";`, "}", 1)},
		{"edge outside cluster", strings.Replace(original, "nodesep=0.35;\n", "nodesep=0.35;\n1 -> 2 [label=\"\",color=\"black\",fontcolor=\"black\"];\n", 1)},
		{"unknown rank vertex", strings.Replace(original, "{rank = same; 5053833254051665420;}", "{rank = same; 0;}", 1)},
		{"extra closing brace", original + "\n}\n"},
		{"trailing statement", original + "\nunexpected\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.text == original {
				t.Fatal("test mutation did not change the graph")
			}
			if _, err := parseGraph(tc.text); err == nil {
				t.Fatal("accepted malformed DOT")
			}
		})
	}
}
