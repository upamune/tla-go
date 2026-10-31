package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/upamune/tla-go/checker"
)

// simulationTraceFile follows SimulationWorker, which writes each state with
// TLCStateMut.toString: through VIEW under -view and tersely under -terse.
func simulationTraceFile(prefix string, style traceStyle) func(int, []checker.TraceStep) error {
	return func(index int, trace []checker.TraceStep) error {
		name := fmt.Sprintf("%s_0_%d", prefix, index)
		var contents bytes.Buffer
		fmt.Fprintf(&contents, "---------------- MODULE %s -----------------\n", name)
		for i, step := range trace {
			fmt.Fprintf(&contents, "STATE_%d == \n", i+1)
			step.State = style.state(step.State)
			printTraceState(&contents, step)
			contents.WriteByte('\n')
		}
		contents.WriteString("=================================================\n")
		return os.WriteFile(name, contents.Bytes(), 0o666)
	}
}
