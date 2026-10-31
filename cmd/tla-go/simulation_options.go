package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

func parseOtherOption(opts *options, args []string) (int, error) {
	switch args[0] {
	case "-simulate":
		return parseSimulationOption(opts, args)
	case "-depth":
		return parseSimulationDepth(opts, args)
	default:
		return 0, parsePath(opts, args[0])
	}
}

func parseSimulationOption(opts *options, args []string) (int, error) {
	opts.simulate = true
	if len(args) < 2 || !(strings.Contains(args[1], "num=") || strings.Contains(args[1], "file=")) {
		return 0, nil
	}
	for part := range strings.SplitSeq(args[1], ",") {
		key, text, ok := strings.Cut(part, "=")
		if !ok || key != "num" {
			return 0, fmt.Errorf("unsupported simulation option %s", part)
		}
		count, err := strconv.ParseInt(text, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid simulation trace count %s", text)
		}
		opts.simulationTraces = max(1, int(count))
	}
	return 1, nil
}

func parseSimulationDepth(opts *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("-depth requires an integer")
	}
	depth, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil || depth < -1 {
		return 0, fmt.Errorf("invalid simulation depth %s", args[1])
	}
	if depth == -1 {
		opts.simulationDepth = int(^uint(0) >> 1)
	} else {
		opts.simulationDepth = int(depth)
	}
	return 1, nil
}

func configureSimulation(opts *options) error {
	if !opts.simulate {
		return nil
	}
	if opts.dumpPath != "" {
		return fmt.Errorf("-dump is not supported in simulation mode")
	}
	if opts.debugLiveness {
		return fmt.Errorf("-debug-liveness is not supported in simulation mode")
	}
	if opts.seedSet {
		return nil
	}
	var seed [8]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return fmt.Errorf("generate simulation seed: %w", err)
	}
	opts.seed = int64(binary.BigEndian.Uint64(seed[:]))
	return nil
}
