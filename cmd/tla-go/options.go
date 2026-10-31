package main

import (
	"fmt"
	"path/filepath"
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
}

// valueOptions parse an option and its arguments, returning the number of
// arguments consumed after the option name.
var valueOptions = map[string]func(*options, []string) (int, error){
	"-config":   parsePathOption,
	"-metadir":  parsePathOption,
	"-seed":     parseRandomOption,
	"-aril":     parseRandomOption,
	"-lncheck":  parseLivenessOption,
	"-dump":     parseDumpOption,
	"-workers":  parseFixedOption,
	"-fp":       parseFixedOption,
	"-I":        parseLibraryOption,
	"-simulate": parseSimulationOption,
	"-depth":    parseSimulationDepth,
}

func setHelp(opts *options)    { opts.help = true }
func setVersion(opts *options) { opts.version = true }

func parseOptions(args []string) (options, error) {
	opts := options{simulationUnlimited: true, simulationDepth: 100}
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
	}
	return 1, nil
}

func unsupportedFixedOption(option string, number int) bool {
	if option == "-workers" {
		return number != 1
	}
	return number < 0 || number >= value.TLCFingerprintPolynomialCount
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
