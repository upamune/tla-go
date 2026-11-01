package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/syntax/parser"
)

// coordinatedEventOutput serializes the listed callback transactions within one
// check. Eligible periodic reports use the same boundary and stop before final
// or error output. Checkpoint, debug, graph and user-file writes
// remain separate. Custom writers, including SpecTE's MultiWriter, retain the
// original path because their Write methods may call back recursively. Positive
// coverage currently rejects -continue, -dfid and -simulate.
type coordinatedEventOutput struct {
	mu  sync.Mutex
	out io.Writer
}

func (c *coordinatedEventOutput) commit(data []byte) {
	if len(data) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// Existing CLI output ignores write errors and does not retry short writes.
	_, _ = c.out.Write(data)
}

func coordinateEventOutput(interval time.Duration, out io.Writer) bool {
	if interval <= 0 {
		return false
	}
	file, ok := out.(*os.File)
	return ok && file != nil
}

// render invokes the formatter without a coordinator lock: formatting may
// trigger nested callbacks. Only the finished bytes are committed atomically.
func (c *coordinatedEventOutput) render(format func(io.Writer)) {
	var buf bytes.Buffer
	format(&buf)
	c.commit(buf.Bytes())
}

func coordinateCallbacks(checkOpts *checker.Options, opts options, module *parser.Module, out io.Writer) *coordinatedEventOutput {
	c := &coordinatedEventOutput{out: out}
	checkOpts.OnSearch = func(event checker.SearchEvent) {
		c.render(func(w io.Writer) { configuredSearchOutput(opts, w)(event) })
	}
	if !opts.noWarning {
		var mu sync.Mutex
		var buf bytes.Buffer
		warning := configuredWarningOutput(opts, &buf)
		checkOpts.OnWarning = func(event checker.Warning) {
			mu.Lock()
			defer mu.Unlock()
			buf.Reset()
			warning(event)
			c.commit(buf.Bytes())
		}
	}
	if opts.compat.userOutput == nil {
		checkOpts.OnPrint = func(line string) { c.render(func(w io.Writer) { fmt.Fprintln(w, line) }) }
	}
	// Rebuild the formatter with a per-callback buffer. Keep SpecTE recording
	// separate from output; it does not evaluate user expressions.
	if checkOpts.OnViolation != nil {
		checkOpts.OnViolation = func(result checker.Result) {
			if opts.compat.specTE != nil {
				opts.compat.specTE.record(result)
			}
			c.render(func(w io.Writer) { modelViolationOutput(opts, module, w)(result) })
		}
	}
	checkOpts.OnTemporalViolation = func(result checker.Result) {
		c.render(func(w io.Writer) { temporalViolationOutput(opts, module, w)(result) })
	}
	checkOpts.OnTemporalCheck = func(result checker.Result) {
		c.render(func(w io.Writer) { temporalCheckOutput(opts, w)(result) })
	}
	return c
}

func temporalViolationOutput(opts options, module *parser.Module, out io.Writer) func(checker.Result) {
	return func(result checker.Result) {
		result = newTraceStyle(opts).apply(result)
		if opts.tool {
			printToolViolation(toolOutput{out: out}, result, module)
		} else {
			printViolation(out, result, module)
		}
	}
}

func temporalCheckOutput(opts options, out io.Writer) func(checker.Result) {
	return func(result checker.Result) {
		if result.Status == checker.LivenessViolation && result.Violation != nil && result.Violation.Unwitnessed != nil {
			if opts.tool {
				failure := result.Violation.Unwitnessed
				if failure.Counts != "" {
					fmt.Fprintln(out, failure.Counts)
				}
				(toolOutput{out: out}).message(1000, 1, failure.Message)
			} else {
				printUnwitnessed(out, result.Violation.Unwitnessed)
			}
		}
		printTemporalCheckEnd(out, opts.tool, *result.TemporalCheckDuration)
	}
}
