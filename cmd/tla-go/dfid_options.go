package main

import (
	"fmt"
	"io"
	"strconv"
)

// javaOptionError is a TLC command-line error that exits with status 1.
type javaOptionError struct{ message string }

func (e *javaOptionError) Error() string { return e.message }

// printJavaOptionError follows TLC.printErrorMsg, which reports
// WRONG_COMMANDLINE_PARAMS_TLC with TLC's usage line on standard output, as
// message 1102 when -tool precedes the failing option.
func printJavaOptionError(out io.Writer, tool bool, err error) {
	body := err.Error() + "\nUsage: java tlc2.TLC [-option] inputfile"
	if !tool {
		fmt.Fprintf(out, "Error: %s\n", body)
		return
	}
	t := toolOutput{out: out}
	t.message(2262, 0, "tla-go "+version)
	t.message(1102, 1, body)
}

// parseDepthFirstOption reads TLC's -dfid argument with its error messages.
func parseDepthFirstOption(opts *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, &javaOptionError{"Error: expect a nonnegative integer for -dfid option."}
	}
	depth, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil {
		return 0, &javaOptionError{"Errorexpect a nonnegative integer for -dfid option. But encountered " + args[1]}
	}
	if depth < 0 {
		return 0, &javaOptionError{"Error: expect a nonnegative integer for -dfid option."}
	}
	opts.dfid, opts.dfidDepth = true, int(depth)
	return 1, nil
}

// configureDepthFirst rejects options that the depth-first mode lacks. TLC's
// depth-first checker rejects several workers itself, so only that mode
// accepts them here.
func configureDepthFirst(opts *options) error {
	if !depthFirstMode(*opts) {
		if opts.workers != 1 {
			return fmt.Errorf("unsupported -workers value %d", opts.workers)
		}
		return nil
	}
	if opts.dumpPath != "" {
		return fmt.Errorf("-dump is not supported in depth-first mode")
	}
	if opts.debugLiveness {
		return fmt.Errorf("-debug-liveness is not supported in depth-first mode")
	}
	return nil
}
