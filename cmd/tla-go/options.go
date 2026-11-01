package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/upamune/tla-go/value"
)

// commandLineError is a Java TLC parameter error, which exits with code 1.
type commandLineError string

func (e commandLineError) Error() string { return string(e) }

const errMissingLivenessStrategy = commandLineError("Error: expect a strategy such as final for -lncheck option.")

var flagOptions = map[string]func(*options){
	"-h":                     setHelp,
	"--help":                 setHelp,
	"-help":                  setHelp,
	"-version":               setVersion,
	"--version":              setVersion,
	"-deadlock":              func(opts *options) { opts.deadlockOff = true },
	"-debug-liveness":        func(opts *options) { opts.debugLiveness = true },
	"-modern-random-subsets": func(opts *options) { opts.modernRandomSubsets = true },
	"-tool":                  func(opts *options) { opts.tool = true },
	"-continue":              func(opts *options) { opts.continuation = true },
	"-nowarning":             func(opts *options) { opts.noWarning = true },
	"-view":                  func(opts *options) { opts.view = true },
}

// valueOptions parse an option and its arguments, returning the number of
// arguments consumed after the option name.
var valueOptions = map[string]func(*options, []string) (int, error){
	"-config":     parsePathOption,
	"-metadir":    parsePathOption,
	"-seed":       parseRandomOption,
	"-aril":       parseRandomOption,
	"-lncheck":    parseLivenessOption,
	"-dump":       parseDumpOption,
	"-workers":    parseWorkersOption,
	"-fp":         parseFingerprintOption,
	"-checkpoint": parseCheckpointOption,
	"-coverage":   parseCoverageOption,
	"-maxSetSize": parseMaxSetSizeOption,
	"-I":          parseLibraryOption,
	"-simulate":   parseSimulationOption,
	"-depth":      parseSimulationDepth,
	"-dfid":       parseDepthFirstOption,
}

// parseCoverageOption validates the interval using TLC 1.7.4's 32-bit arithmetic.
func parseCoverageOption(opts *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, commandLineError("Error: coverage report interval required.")
	}
	minutes, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil {
		return 0, commandLineError("Error: An integer for coverage report interval required. But encountered " + args[1])
	}
	if int32(minutes)*60*1000 < 0 {
		return 0, commandLineError("Error: expect a nonnegative integer for -coverage option.")
	}
	if minutes != 0 {
		return 0, fmt.Errorf("periodic -coverage reports are not yet supported; use 0")
	}
	opts.coverage = true
	return 1, nil
}

func setHelp(opts *options)    { opts.help = true }
func setVersion(opts *options) { opts.version = true }

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
	if err := configureSearchModes(&opts); err != nil {
		return opts, err
	}
	if err := configureCoverage(opts); err != nil {
		return opts, err
	}
	return opts, nil
}

func parseOption(opts *options, args []string) (int, error) {
	if set, ok := flagOptions[args[0]]; ok {
		set(opts)
		return 0, nil
	}
	if parse, ok := valueOptions[args[0]]; ok {
		return parse(opts, args)
	}
	return parseCompatOption(opts, args)
}

// missingPathErrors are Java TLC's messages for a path option without its
// argument.
var missingPathErrors = map[string]commandLineError{
	"-config":  "Error: expect a file name for -config option.",
	"-metadir": "Error: need to specify the metadata directory.",
}

func parsePathOption(opts *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, missingPathErrors[args[0]]
	}
	if args[0] == "-config" {
		opts.configPath = args[1]
	} else {
		opts.metaRoot = args[1]
	}
	return 1, nil
}

func parseLivenessOption(_ *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, errMissingLivenessStrategy
	}
	return 1, nil
}

func parseLibraryOption(opts *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("-I requires a path")
	}
	opts.libraryPaths = append(opts.libraryPaths, args[1])
	return 1, nil
}

// parseDumpOption follows TLC: an argument starting with "dot" selects a DOT
// graph when a file name follows it; otherwise the argument names a plain
// state dump.
func parseDumpOption(opts *options, args []string) (int, error) {
	if len(args) > 2 && strings.HasPrefix(args[1], "dot") {
		format, err := parseDOTFormat(args[1])
		if err != nil {
			return 0, err
		}
		opts.dumpDOT = &format
		return 2, setDumpPath(opts, args[2], ".dot")
	}
	if len(args) < 2 {
		return 0, commandLineError("Error: A file name for dumping states required.")
	}
	opts.dumpDOT = nil
	return 1, setDumpPath(opts, args[1], ".dump")
}

func parseDOTFormat(text string) (dotFormat, error) {
	text = strings.ToLower(text)
	if strings.Contains(text, "snapshot") {
		return dotFormat{}, fmt.Errorf("unsupported dump option snapshot")
	}
	return dotFormat{colorize: strings.Contains(text, "colorize"), actionLabels: strings.Contains(text, "actionlabels")}, nil
}

func setDumpPath(opts *options, path, suffix string) error {
	if path == "" {
		return fmt.Errorf("-dump requires an output path")
	}
	opts.dumpPath = path
	if !strings.HasSuffix(path, suffix) {
		opts.dumpPath += suffix
	}
	return nil
}

func parseFingerprintOption(opts *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, fmt.Errorf("%s requires an integer", args[0])
	}
	parsed, err := strconv.Atoi(args[1])
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %s", args[0], args[1])
	}
	if parsed < 0 || parsed >= value.TLCFingerprintPolynomialCount {
		return 0, fmt.Errorf("unsupported %s value %s", args[0], args[1])
	}
	opts.fingerprint = parsed
	return 1, nil
}

// parseWorkersOption accepts Java TLC's worker counts. Exploration is
// sequential regardless of the count.
func parseWorkersOption(opts *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, commandLineError("Error: expect an integer or 'auto' for -workers option.")
	}
	if strings.ToLower(strings.TrimSpace(args[1])) == "auto" {
		opts.workers = runtime.NumCPU()
		return 1, nil
	}
	workers, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil {
		return 0, commandLineError("Error: worker number or 'auto' required. But encountered " + args[1])
	}
	if workers < 1 {
		return 0, commandLineError("Error: at least one worker required.")
	}
	opts.workers = int(workers)
	return 1, nil
}

// parseCheckpointOption validates Java TLC's checkpoint interval in minutes.
// tla-go does not write checkpoints.
func parseCheckpointOption(_ *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, commandLineError("Error: checkpoint interval required.")
	}
	minutes, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil {
		return 0, commandLineError("Error: An integer for checkpoint interval is required. But encountered " + args[1])
	}
	// Java computes the interval in milliseconds with 32-bit arithmetic and
	// rejects intervals that wrap around to a negative value.
	if int32(minutes)*1000*60 < 0 {
		return 0, commandLineError("Error: expect a nonnegative integer for -checkpoint option.")
	}
	return 1, nil
}

// parseMaxSetSizeOption accepts Java TLC's bound on enumerated set sizes.
func parseMaxSetSizeOption(opts *options, args []string) (int, error) {
	if len(args) < 2 {
		return 0, commandLineError("Error: maxSetSize required.")
	}
	bound, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil {
		return 0, commandLineError("Error: An integer for maxSetSize required. But encountered " + args[1])
	}
	if bound < 1 {
		return 0, commandLineError("Error: Value in interval [0, 2147483647] for maxSetSize required. But encountered " + args[1])
	}
	opts.maxSetSize = int(bound)
	return 1, nil
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

func configureCoverage(opts options) error {
	if opts.coverage && (opts.simulate || opts.dfid || opts.continuation) {
		return fmt.Errorf("-coverage is not yet supported with -simulate, -dfid, or -continue")
	}
	return nil
}

func configureSearchModes(opts *options) error {
	if err := configureSimulation(opts); err != nil {
		return err
	}
	return configureDepthFirst(opts)
}
