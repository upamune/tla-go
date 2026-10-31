package main

import (
	"fmt"
	"strconv"
)

// javaOptionError is a TLC command-line error that exits with status 1.
type javaOptionError struct{ message string }

func (e *javaOptionError) Error() string { return e.message }

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

func configureDepthFirst(opts *options) error {
	if !opts.dfid || opts.simulate {
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
