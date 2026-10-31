package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/upamune/tla-go/checker"
)

type dotDump struct {
	file   *os.File
	writer *bufio.Writer
	levels map[int]*javaLongSet
	order  javaIntSet
}

func openGraphDump(path string) (*dotDump, error) {
	if path == "" {
		return nil, nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create DOT dump: %w", err)
	}
	graph := &dotDump{file: file, writer: bufio.NewWriter(file), levels: map[int]*javaLongSet{}}
	if _, err := fmt.Fprint(graph.writer, "strict digraph DiskGraph {\nnodesep=0.35;\nsubgraph cluster_graph {\ncolor=\"white\";\n"); err != nil {
		file.Close()
		return nil, err
	}
	return graph, nil
}

func (d *dotDump) event(event checker.GraphEvent) error {
	if event.Stuttering {
		_, err := fmt.Fprintf(d.writer, "%d -> %d [style=\"dashed\"];\n", int64(event.Parent), int64(event.Fingerprint))
		d.addRank(event.Depth-1, event.Parent)
		return err
	}
	if !event.Initial {
		if _, err := fmt.Fprintf(d.writer, "%d -> %d [label=\"%s\",color=\"black\",fontcolor=\"black\"];\n", int64(event.Parent), int64(event.Fingerprint), dotEscape(event.Action)); err != nil {
			return err
		}
		d.addRank(event.Depth-1, event.Parent)
	} else {
		d.addRank(event.Depth, event.Fingerprint)
	}
	if !event.New {
		return nil
	}
	style, end := "", ";\n"
	if event.Initial {
		style, end = ",style = filled", "\n"
	}
	_, err := fmt.Fprintf(d.writer, "%d [label=\"%s\"%s]%s", int64(event.Fingerprint), dotEscape(dotState(event)), style, end)
	return err
}

func dotState(event checker.GraphEvent) string {
	var result strings.Builder
	prefix := ""
	if len(event.Order) > 1 {
		prefix = "/\\ "
	}
	for i, name := range event.Order {
		if i > 0 {
			result.WriteByte('\n')
		}
		fmt.Fprintf(&result, "%s%s = %s", prefix, name, stateValueText(event.State[name]))
	}
	return result.String()
}

func dotEscape(text string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`).Replace(text)
}

func (d *dotDump) addRank(level int, fingerprint uint64) {
	if d.levels[level] == nil {
		d.levels[level] = &javaLongSet{}
		d.order.add(level)
	}
	d.levels[level].add(fingerprint)
}

func (d *dotDump) close(finish bool) error {
	if finish {
		d.writeFooter()
	}
	err := d.writer.Flush()
	if closeErr := d.file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func (d *dotDump) writeFooter() {
	for _, number := range d.order.values() {
		level := d.levels[number]
		fmt.Fprint(d.writer, "{rank = same; ")
		for _, fingerprint := range level.values() {
			fmt.Fprintf(d.writer, "%d;", int64(fingerprint))
		}
		fmt.Fprintln(d.writer, "}")
	}
	fmt.Fprint(d.writer, "}\n}")
}
