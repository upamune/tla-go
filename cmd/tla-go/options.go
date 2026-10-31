package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/upamune/tla-go/value"
)

var errMissingLivenessStrategy = errors.New("Error: expect a strategy such as final for -lncheck option.")

func parseOptions(args []string) (options, error) {
	opts := options{simulationUnlimited: true, simulationDepth: 100, workers: 1}
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
	if err := configureSimulation(&opts); err != nil {
		return opts, err
	}
	if err := configureDepthFirst(&opts); err != nil {
		return opts, err
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
	case "-tool":
		opts.tool = true
	case "-config", "-seed", "-aril", "-lncheck", "-dump", "-workers", "-fp", "-metadir", "-I":
		return parseValueOption(opts, args)
	default:
		return parseOtherOption(opts, args)
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
	if !strings.HasSuffix(opts.dumpPath, ".dot") {
		opts.dumpPath += ".dot"
	}
	return 2, nil
}

func parseFixedOption(opts *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("%s requires an integer", args[0])
	}
	parsed, err := strconv.Atoi(args[1])
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %s", args[0], args[1])
	}
	if unsupportedFixedOption(args[0], parsed) {
		return 0, fmt.Errorf("unsupported %s value %s", args[0], args[1])
	}
	if args[0] == "-fp" {
		opts.fingerprint = parsed
	} else {
		opts.workers = parsed
	}
	return 1, nil
}

func unsupportedFixedOption(option string, number int) bool {
	if option == "-workers" {
		return number < 1
	}
	return number < 0 || number >= value.TLCFingerprintPolynomialCount
}

func parseValueOption(opts *options, args []string) (int, error) {
	switch args[0] {
	case "-config", "-metadir":
		if len(args) < 2 {
			return 0, fmt.Errorf("%s requires a path", args[0])
		}
		if args[0] == "-config" {
			opts.configPath = args[1]
		} else {
			opts.metaRoot = args[1]
		}
		return 1, nil
	case "-seed", "-aril":
		return parseRandomOption(opts, args)
	case "-lncheck":
		if len(args) < 2 {
			return 0, errMissingLivenessStrategy
		}
		return 1, nil
	case "-dump", "-workers", "-fp", "-I":
		return parseAuxiliaryOption(opts, args)
	}
	return 0, nil
}

func parseRandomOption(opts *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("%s requires an integer", args[0])
	}
	number, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %s", strings.TrimPrefix(args[0], "-"), args[1])
	}
	if args[0] == "-aril" {
		opts.aril = number
	} else {
		opts.seed = number
		opts.seedSet = true
	}
	return 1, nil
}

func parseAuxiliaryOption(opts *options, args []string) (int, error) {
	if args[0] == "-I" {
		if len(args) < 2 {
			return 0, fmt.Errorf("-I requires a path")
		}
		opts.libraryPaths = append(opts.libraryPaths, args[1])
		return 1, nil
	}
	if args[0] == "-dump" {
		return parseDumpOption(opts, args)
	}
	return parseFixedOption(opts, args)
}

func parsePath(opts *options, path string) error {
	if path == "-sany-metadata" {
		opts.sanyMetadata = true
		return nil
	}
	if library, ok := strings.CutPrefix(path, "-DTLA-Library="); ok {
		opts.libraryPaths = filepath.SplitList(library)
		return nil
	}
	if strings.HasPrefix(path, "-") {
		return fmt.Errorf("unsupported option %s", path)
	}
	opts.paths = append(opts.paths, path)
	return nil
}
