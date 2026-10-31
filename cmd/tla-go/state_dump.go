package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/upamune/tla-go/checker"
)

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

func (d *textDump) close(bool) error {
	err := d.writer.Flush()
	if closeErr := d.file.Close(); err == nil {
		err = closeErr
	}
	return err
}
