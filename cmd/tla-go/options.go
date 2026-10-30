package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var errMissingLivenessStrategy = errors.New("Error: expect a strategy such as final for -lncheck option.")

func parseOptions(args []string) (options, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		consumed, err := parseOption(&opts, args[i:])
		if err != nil {
			return opts, err
		}
		i += consumed
	}
	if !opts.help && !opts.version && len(opts.paths) == 0 {
		return opts, fmt.Errorf("missing model path")
	}
	return opts, nil
}

func parseOption(opts *options, args []string) (int, error) {
	switch args[0] {
	case "-h", "--help", "-help":
		opts.help = true
	case "-version", "--version":
		opts.version = true
	case "-deadlock":
		opts.deadlockOff = true
	case "-debug-liveness":
		opts.debugLiveness = true
	case "-modern-random-subsets":
		opts.modernRandomSubsets = true
	case "-config", "-seed", "-lncheck", "-dump", "-workers", "-fp":
		return parseValueOption(opts, args)
	default:
		return 0, parsePath(opts, args[0])
	}
	return 0, nil
}

func parseDumpOption(opts *options, args []string) (int, error) {
	if len(args) < 3 || args[2] == "" {
		return 0, fmt.Errorf("-dump requires dot,actionlabels and an output path")
	}
	if args[1] != "dot,actionlabels" {
		return 0, fmt.Errorf("unsupported dump format %s", args[1])
	}
	opts.dumpPath = args[2]
	return 2, nil
}

func parseFixedOption(args []string) (int, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("%s requires an integer", args[0])
	}
	value, err := strconv.Atoi(args[1])
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %s", args[0], args[1])
	}
	if args[0] == "-workers" && value != 1 || args[0] == "-fp" && value != 0 {
		return 0, fmt.Errorf("unsupported %s value %s", args[0], args[1])
	}
	return 1, nil
}

func parseValueOption(opts *options, args []string) (int, error) {
	switch args[0] {
	case "-config":
		if len(args) < 2 {
			return 0, fmt.Errorf("-config requires a path")
		}
		opts.configPath = args[1]
		return 1, nil
	case "-seed":
		return parseSeedOption(opts, args)
	case "-lncheck":
		if len(args) < 2 {
			return 0, errMissingLivenessStrategy
		}
		return 1, nil
	case "-dump", "-workers", "-fp":
		return parseGraphOption(opts, args)
	}
	return 0, nil
}

func parseSeedOption(opts *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("-seed requires an integer")
	}
	seed, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid seed %s", args[1])
	}
	opts.seed = seed
	return 1, nil
}

func parseGraphOption(opts *options, args []string) (int, error) {
	if args[0] == "-dump" {
		return parseDumpOption(opts, args)
	}
	return parseFixedOption(args)
}

func parsePath(opts *options, path string) error {
	if strings.HasPrefix(path, "-") {
		return fmt.Errorf("unsupported option %s", path)
	}
	opts.paths = append(opts.paths, path)
	return nil
}
