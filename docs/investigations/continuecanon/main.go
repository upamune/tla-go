// Command canonical converts the restricted DOT emitted for Continue into
// sorted vertices and directed edge records. It rejects unrecognized syntax;
// this is not a general DOT parser.
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

var (
	vertex = regexp.MustCompile(`^(-?[0-9]+) \[label="((?:\\.|[^"\\])*)"(,style = filled)?\];?$`)
	edge   = regexp.MustCompile(`^(-?[0-9]+) -> (-?[0-9]+) \[label="",color="black",fontcolor="black"\];$`)
	rank   = regexp.MustCompile(`^\{rank = same; (-?[0-9]+);\}$`)
	header = [...]string{"strict digraph DiskGraph {", "nodesep=0.35;", "subgraph cluster_graph {", `color="white";`}
)

const (
	graphBody = len(header) + iota
	graphOuter
	graphComplete
)

type (
	arc   struct{ from, to string }
	graph struct {
		labels  map[string]string
		ids     map[string]string
		initial map[string]bool
		edges   []arc
		ranks   []string
		stage   int
	}
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./docs/investigations/continuecanon graph.dot")
		os.Exit(2)
	}
	if err := convert(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func convert(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	g := graph{labels: map[string]string{}, ids: map[string]string{}, initial: map[string]bool{}}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if err := g.consume(strings.TrimSpace(scanner.Text())); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	lines, err := g.lines()
	if err != nil {
		return err
	}
	sort.Strings(lines)
	for _, line := range lines {
		fmt.Println(line)
	}
	return nil
}

func (g *graph) consume(line string) error {
	if line == "" {
		return nil
	}
	if g.stage < graphBody {
		return g.consumeHeader(line)
	}
	if g.stage == graphBody {
		return g.consumeBody(line)
	}
	return g.consumeEnd(line)
}

func (g *graph) consumeHeader(line string) error {
	if line != header[g.stage] {
		return fmt.Errorf("expected %q, got %q", header[g.stage], line)
	}
	g.stage++
	return nil
}

func (g *graph) consumeBody(line string) error {
	if line == "}" {
		g.stage = graphOuter
		return nil
	}
	if m := vertex.FindStringSubmatch(line); m != nil {
		return g.addVertex(m)
	}
	if m := edge.FindStringSubmatch(line); m != nil {
		g.edges = append(g.edges, arc{m[1], m[2]})
		return nil
	}
	if m := rank.FindStringSubmatch(line); m != nil {
		g.ranks = append(g.ranks, m[1])
		return nil
	}
	return fmt.Errorf("unrecognized DOT line: %s", line)
}

func (g *graph) consumeEnd(line string) error {
	if g.stage == graphOuter && line == "}" {
		g.stage = graphComplete
		return nil
	}
	return fmt.Errorf("unexpected DOT line after cluster: %s", line)
}

func (g *graph) addVertex(m []string) error {
	id, label := m[1], m[2]
	if old, ok := g.labels[id]; ok && old != label {
		return fmt.Errorf("conflicting labels for vertex %s", id)
	}
	if old, ok := g.ids[label]; ok && old != id {
		return fmt.Errorf("duplicate state label for vertices %s and %s", old, id)
	}
	g.labels[id], g.ids[label] = label, id
	g.initial[id] = g.initial[id] || m[3] != ""
	return nil
}

func (g *graph) lines() ([]string, error) {
	if g.stage != graphComplete {
		return nil, fmt.Errorf("incomplete DOT graph")
	}
	for _, id := range g.ranks {
		if _, ok := g.labels[id]; !ok {
			return nil, fmt.Errorf("rank references unknown vertex: %s", id)
		}
	}
	lines := make([]string, 0, len(g.labels)+len(g.edges))
	for id, label := range g.labels {
		lines = append(lines, fmt.Sprintf("V initial=%t label=%s", g.initial[id], label))
	}
	for _, e := range g.edges {
		from, fromOK := g.labels[e.from]
		to, toOK := g.labels[e.to]
		if !fromOK || !toOK {
			return nil, fmt.Errorf("edge references unknown vertex: %s -> %s", e.from, e.to)
		}
		lines = append(lines, fmt.Sprintf("E %s -> %s", from, to))
	}
	return lines, nil
}
