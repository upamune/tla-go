package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

func parseSimulationOption(opts *options, args []string) (int, error) {
	opts.simulate = true
	if len(args) < 2 || !(strings.Contains(args[1], "num=") || strings.Contains(args[1], "file=")) {
		return 0, nil
	}
	for part := range strings.SplitSeq(args[1], ",") {
		if err := parseSimulationArgument(opts, part); err != nil {
			return 0, err
		}
	}
	return 1, nil
}

func parseSimulationArgument(opts *options, part string) error {
	key, text, ok := strings.Cut(part, "=")
	if ok && key == "file" {
		opts.simulationFile, opts.simulationFileSet = text, true
		return nil
	}
	if !ok || key != "num" {
		return fmt.Errorf("unsupported simulation option %s", part)
	}
	count, err := strconv.ParseInt(text, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid simulation trace count %s", text)
	}
	opts.simulationTraces, opts.simulationUnlimited = int(count), false
	return nil
}

func parseSimulationDepth(opts *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("-depth requires an integer")
	}
	depth, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil || depth < -1 {
		return 0, fmt.Errorf("invalid simulation depth %s", args[1])
	}
	opts.simulationDepth = int(depth)
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
	if opts.workers > 1 {
		return fmt.Errorf("-workers greater than 1 is not supported in simulation mode")
	}
	if opts.seedSet {
		return nil
	}
	var seed [8]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return fmt.Errorf("generate simulation seed: %w", err)
	}
	opts.seed = int64(binary.BigEndian.Uint64(seed[:]))
	// Java TLC ignores -aril when it chooses the simulation seed itself.
	opts.aril = 0
	return nil
}
