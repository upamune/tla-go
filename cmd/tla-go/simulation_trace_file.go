package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/upamune/tla-go/checker"
)

func simulationTraceFile(prefix string) func(int, []checker.TraceStep) error {
	return func(index int, trace []checker.TraceStep) error {
		name := fmt.Sprintf("%s_0_%d", prefix, index)
		var contents bytes.Buffer
		fmt.Fprintf(&contents, "---------------- MODULE %s -----------------\n", name)
		for i, step := range trace {
			fmt.Fprintf(&contents, "STATE_%d == \n", i+1)
			printStateOrdered(&contents, step.State, step.StateOrder)
			contents.WriteByte('\n')
		}
		contents.WriteString("=================================================\n")
		return os.WriteFile(name, contents.Bytes(), 0o666)
	}
}
