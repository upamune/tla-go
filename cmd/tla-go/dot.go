package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/value"
)

// stateDump writes the states and transitions reported during exploration.
type stateDump interface {
	event(checker.GraphEvent) error
	// close writes any trailer when finish is true and closes the file.
	close(finish bool) error
	empty() bool
}

// dotFormat holds the options of Java TLC's DotStateWriter.
type dotFormat struct {
	colorize, actionLabels bool
}

const dotColorScheme = "paired12"

type dotDump struct {
	file   *os.File
	writer *bufio.Writer
	levels map[int]*javaLongSet
	order  javaIntSet
	format dotFormat
	// colors and actions follow DotStateWriter's action-to-color HashMap.
	colors  map[string]int
	actions javaStringSet
}

func openStateDump(path string, format *dotFormat) (stateDump, error) {
	if path == "" {
		return nil, nil
	}
	if format == nil {
		dump, err := openTextDump(path)
		if err != nil {
			return nil, err
		}
		return dump, nil
	}
	dump, err := openDOTDump(path, *format)
	if err != nil {
		return nil, err
	}
	return dump, nil
}

func openDOTDump(path string, format dotFormat) (*dotDump, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create DOT dump: %w", err)
	}
	graph := &dotDump{file: file, writer: bufio.NewWriter(file), levels: map[int]*javaLongSet{}, format: format, colors: map[string]int{}}
	header := "strict digraph DiskGraph {\n"
	if format.colorize {
		header += fmt.Sprintf("edge [colorscheme=\"%s\"]\n", dotColorScheme)
	}
	// Like Java TLC's DotStateWriter, write the header immediately.
	if _, err := fmt.Fprint(graph.writer, header+"nodesep=0.35;\nsubgraph cluster_graph {\ncolor=\"white\";\n"); err != nil {
		file.Close()
		return nil, err
	}
	if err := graph.writer.Flush(); err != nil {
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
		if _, err := fmt.Fprintf(d.writer, "%d -> %d%s;\n", int64(event.Parent), int64(event.Fingerprint), d.transitionLabel(event.Action)); err != nil {
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

func (d *dotDump) transitionLabel(action string) string {
	color := "black"
	if d.format.colorize {
		color = strconv.Itoa(d.actionColor(action))
	}
	label := ""
	if d.format.actionLabels {
		label = dotEscape(action)
	}
	return fmt.Sprintf(" [label=\"%s\",color=\"%s\",fontcolor=\"%s\"]", label, color, color)
}

// actionColor numbers actions from 2 in the order they are first seen.
func (d *dotDump) actionColor(action string) int {
	if color, ok := d.colors[action]; ok {
		return color
	}
	d.colors[action] = len(d.colors) + 2
	d.actions.add(action)
	return d.colors[action]
}

func dotState(event checker.GraphEvent) string {
	if event.View != "" {
		return strings.TrimSpace(event.View)
	}
	var result strings.Builder
	prefix := ""
	if len(event.Order) > 1 {
		prefix = "/\\ "
	}
	for i, name := range event.Order {
		if i > 0 {
			result.WriteByte('\n')
		}
		v := event.State[name]
		if v.Kind() == value.Invalid {
			fmt.Fprintf(&result, "%s%s = null", prefix, name)
		} else {
			fmt.Fprintf(&result, "%s%s = %s", prefix, name, v)
		}
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

func (d *dotDump) empty() bool { return len(d.levels) == 0 }

func (d *dotDump) close(finish bool) error {
	if finish {
		d.writeFooter()
	}
	return closeDump(d.writer, d.file, finish)
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
	fmt.Fprint(d.writer, "}\n")
	if d.format.colorize && len(d.colors) > 1 {
		d.writeLegend()
	}
	fmt.Fprint(d.writer, "}")
}

func (d *dotDump) writeLegend() {
	fmt.Fprintf(d.writer, "subgraph cluster_legend {graph[style=bold];label = \"Next State Actions\" style=\"solid\"\nnode [ labeljust=\"l\",colorscheme=\"%s\",style=filled,shape=record ]\n", dotColorScheme)
	for _, action := range d.actions.values() {
		fmt.Fprintf(d.writer, "%s [label=\"%s\",fillcolor=%d]\n", strings.ReplaceAll(action, "!", ":"), action, d.colors[action])
	}
	fmt.Fprint(d.writer, "}")
}
