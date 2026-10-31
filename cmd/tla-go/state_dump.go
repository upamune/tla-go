package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/upamune/tla-go/checker"
)

// checkDumpPath creates the dump file while options are processed, as Java
// TLC does, so that an unwritable path is a command-line error. A path in the
// metadata directory is created once that directory exists.
func checkDumpPath(path string, out io.Writer) error {
	if path == "" || strings.HasPrefix(path, "${metadir}") {
		return nil
	}
	file, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(out, "Error: Unable to write to file %s\n", path)
		return commandLineError(fmt.Sprintf("Error: Given file name %s for dumping states invalid.", path))
	}
	return file.Close()
}

// textDump follows Java TLC's StateWriter, which numbers each new state. Like
// Java TLC, it also writes a state again for each stuttering step reported
// while liveness is checked.
type textDump struct {
	file   *os.File
	writer *bufio.Writer
	states int
}

func openTextDump(path string) (*textDump, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create state dump: %w", err)
	}
	return &textDump{file: file, writer: bufio.NewWriter(file)}, nil
}

func (d *textDump) event(event checker.GraphEvent) error {
	if !event.New && !event.Stuttering {
		return nil
	}
	d.states++
	if _, err := fmt.Fprintf(d.writer, "State %d:\n", d.states); err != nil {
		return err
	}
	printStateOrdered(d.writer, event.State, event.Order)
	return nil
}

func (d *textDump) empty() bool { return d.states == 0 }

// close flushes the dump when TLC finishes it. Java TLC exits without closing
// its writer after some failures, such as an initial invariant violation, so
// the states it buffered are not written.
func (d *textDump) close(finish bool) error {
	return closeDump(d.writer, d.file, finish)
}

func closeDump(writer *bufio.Writer, file *os.File, finish bool) error {
	var err error
	if finish {
		err = writer.Flush()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}
