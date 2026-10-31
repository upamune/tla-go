package main

import (
	"fmt"
	"strconv"
)

// parseDepthFirstOption reads TLC's -dfid argument with its error messages.
func parseDepthFirstOption(opts *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, commandLineError("Error: expect a nonnegative integer for -dfid option.")
	}
	depth, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil {
		return 0, commandLineError("Errorexpect a nonnegative integer for -dfid option. But encountered " + args[1])
	}
	if depth < 0 {
		return 0, commandLineError("Error: expect a nonnegative integer for -dfid option.")
	}
	opts.dfid, opts.dfidDepth = true, int(depth)
	return 1, nil
}

// configureDepthFirst rejects options that the depth-first mode lacks. TLC's
// depth-first checker rejects several workers itself when checking starts.
func configureDepthFirst(opts *options) error {
	if !depthFirstMode(*opts) {
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
