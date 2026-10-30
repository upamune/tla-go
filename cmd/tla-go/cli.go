package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

type options struct {
	path, configPath           string
	paths                      []string
	deadlockOff, help, version bool
	debugLiveness              bool
	modernRandomSubsets        bool
	onPause                    func()
	seed                       int64
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage: tla-go [-config file.cfg] [-deadlock] [-seed integer] [-debug-liveness] [-modern-random-subsets] <file.tla|file.cfg|glob>...\n       tla-go -version\n\nExperimental: supports a subset of TLA+ model checking.\n-debug-liveness writes JSON debugging events to stderr.\n-modern-random-subsets selects the newer TLC random-subset algorithm.")
}

func run(ctx context.Context, args []string, out, errout io.Writer) int {
	return runWithInput(ctx, args, os.Stdin, out, errout)
}

func runWithInput(ctx context.Context, args []string, input io.Reader, out, errout io.Writer) int {
	opts, err := parseOptions(args)
	if err != nil {
		fmt.Fprintln(errout, err)
		if errors.Is(err, errMissingLivenessStrategy) {
			return 1
		}
		usage(errout)
		return 255
	}
	if opts.help {
		usage(out)
		return 0
	}
	if opts.version {
		fmt.Fprintln(out, "tla-go", version)
		return 0
	}
	reader := bufio.NewReader(input)
	opts.onPause = func() { _, _ = reader.ReadString('\n') }
	return runModels(ctx, opts, out, errout)
}

func runModel(ctx context.Context, opts options, out, errout io.Writer) int {
	specPath, configPath, err := inputPaths(opts)
	if err != nil {
		fmt.Fprintln(errout, err)
		return 255
	}
	module, code := loadModule(specPath, errout)
	if code != 0 {
		return code
	}
	parsed, code := loadConfig(configPath, errout)
	if code != 0 {
		return code
	}
	if opts.deadlockOff {
		parsed.CheckDeadlock = false
	}
	fmt.Fprintf(out, "Checking %s with %s\n", filepath.Base(specPath), filepath.Base(configPath))
	checkOptions := checker.Options{Loader: directoryLoader(filepath.Dir(specPath)), OnPrint: func(line string) { fmt.Fprintln(out, line) }, OnPause: opts.onPause, ReadFile: readJSONFile, WriteFile: writeJSONFile, Seed: opts.seed, ModernRandomSubsets: opts.modernRandomSubsets}
	if opts.debugLiveness {
		checkOptions.OnDebug = func(line string) { fmt.Fprintln(errout, line) }
	}
	result, err := checker.CheckModuleWithOptions(ctx, module, parsed, checkOptions)
	if err != nil {
		return reportModelError(out, errout, err)
	}
	printResult(out, result, module)
	if result.Failure != nil {
		return result.Failure.ExitCode
	}
	return exitCode(result.Status)
}

func reportModelError(out, errout io.Writer, err error) int {
	if configErr, ok := errors.AsType[*checker.ConfigurationError](err); ok {
		fmt.Fprintf(out, "Error: %s\n", configErr.Message)
		return 151
	}
	return reportError(errout, err)
}

func readJSONFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}

func writeJSONFile(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o666)
}

func loadModule(specPath string, errout io.Writer) (*parser.Module, int) {
	source, err := os.ReadFile(specPath)
	if err != nil {
		fmt.Fprintf(errout, "Cannot find the specification file %s\n", specPath)
		return nil, 151
	}
	module, err := parser.ParseModule(string(source))
	if err != nil {
		return nil, reportError(errout, &checker.SyntaxError{Err: err})
	}
	if err := validateModuleName(specPath, module.Name); err != nil {
		return nil, reportError(errout, &checker.SyntaxError{Err: err})
	}
	return module, 0
}

func loadConfig(configPath string, errout io.Writer) (config.Config, int) {
	cfg, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Fprintf(errout, "Cannot find the configuration file %s\n", configPath)
		return config.Config{}, 151
	}
	parsed, err := config.Parse(string(cfg))
	if err != nil {
		fmt.Fprintln(errout, err)
		return config.Config{}, 151
	}
	return parsed, 0
}

func directoryLoader(dir string) checker.ModuleLoader {
	return func(ctx context.Context, name string) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if filepath.Base(name) != name || name == "." || name == ".." {
			return "", fmt.Errorf("invalid module name %q", name)
		}
		source, err := os.ReadFile(filepath.Join(dir, name+".tla"))
		return string(source), err
	}
}

func reportError(w io.Writer, err error) int {
	fmt.Fprintln(w, err)
	if _, ok := errors.AsType[*checker.SyntaxError](err); ok {
		return 150
	}
	if _, ok := errors.AsType[*checker.SemanticError](err); ok {
		return 150
	}
	return 255
}

func inputPaths(opts options) (string, string, error) {
	path := opts.path
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".cfg" {
		path = path[:len(path)-len(ext)] + ".tla"
	} else if ext == "" {
		path += ".tla"
	} else if ext != ".tla" {
		return "", "", fmt.Errorf("expected a .tla or .cfg file")
	}
	cfgPath := opts.configPath
	if cfgPath == "" {
		cfgPath = path[:len(path)-len(filepath.Ext(path))] + ".cfg"
	}
	return path, cfgPath, nil
}

func validateModuleName(path, module string) error {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if name != module {
		return fmt.Errorf("TlaSyntaxError: File name '%s' does not match the name '%s' of the top level module it contains.", name, module)
	}
	return nil
}

func exitCode(status checker.Status) int {
	switch status {
	case checker.Success:
		return 0
	case checker.AssumptionViolation, checker.PostconditionViolation:
		return 10
	case checker.Deadlock:
		return 11
	case checker.InvariantViolation:
		return 12
	case checker.PropertyViolation:
		return 12
	case checker.LivenessViolation:
		return 13
	}
	return 255
}
