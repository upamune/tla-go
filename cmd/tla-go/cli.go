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
	"time"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

type options struct {
	path, configPath           string
	paths                      []string
	libraryPaths               []string
	metaRoot                   string
	deadlockOff, help, version bool
	debugLiveness              bool
	modernRandomSubsets        bool
	tool                       bool
	sanyMetadata               bool
	dumpPath                   string
	onPause                    func()
	seed                       int64
	aril                       int64
	fingerprint                int
	simulate, seedSet          bool
	continuation               bool
	simulationTraces           int
	simulationDepth            int
	simulationUnlimited        bool
	simulationFile             string
	simulationFileSet          bool
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage: tla-go [-DTLA-Library=paths] [-metadir dir] [-config file.cfg] [-deadlock] [-seed integer] [-aril integer] [-simulate [num=N,file=prefix]] [-continue] [-depth N] [-workers 1] [-fp N] [-tool] [-dump dot,actionlabels file.dot] [-debug-liveness] [-modern-random-subsets] <file.tla|file.cfg|glob>...\n       tla-go -sany-metadata [-I dir] file.tla\n       tla-go -version\n\nExperimental: supports a subset of TLA+ model checking and simulation.\n-continue reports supported violations and continues exploration.\n-sany-metadata emits a partial SANY XML projection.\n-debug-liveness writes JSON debugging events to stderr.\n-modern-random-subsets selects the newer TLC random-subset algorithm.")
}

func run(ctx context.Context, args []string, out, errout io.Writer) int {
	if code, ok := runTranslator(args, out, errout); ok {
		return code
	}
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
	if opts.sanyMetadata {
		return runSANYMetadata(ctx, opts, out, errout)
	}
	reader := bufio.NewReader(input)
	opts.onPause = func() { _, _ = reader.ReadString('\n') }
	return runModels(ctx, opts, out, errout)
}

func runModel(ctx context.Context, opts options, out, errout io.Writer) (code int) {
	if opts.metaRoot != "" {
		metaDir, err := makeMetaDir(opts.metaRoot, time.Now())
		if err != nil {
			fmt.Fprintf(errout, "Cannot create metadata directory: %s\n", err)
			return 255
		}
		defer func() {
			if code == 0 {
				_ = os.RemoveAll(metaDir)
			}
		}()
		if after, ok := strings.CutPrefix(opts.dumpPath, "${metadir}"); ok {
			opts.dumpPath = metaDir + after
		}
	}
	return runConfiguredModel(ctx, opts, out, errout)
}

func runConfiguredModel(ctx context.Context, opts options, out, errout io.Writer) int {
	if opts.tool {
		return runToolModel(ctx, opts, out, errout)
	}
	specPath, configPath, err := inputPaths(opts)
	if err != nil {
		fmt.Fprintln(errout, err)
		return 255
	}
	parsed, err := readConfig(configPath)
	if err != nil {
		printConfigError(out, configPath, err)
		return 255
	}
	module, code := loadModule(specPath, opts.libraryPaths, errout)
	if code != 0 {
		return code
	}
	if opts.deadlockOff {
		parsed.CheckDeadlock = false
	}
	printChecking(out, specPath, configPath, opts)
	result, err := checkModelWithOutput(ctx, opts, module, parsed, specPath, out, errout)
	if err != nil {
		return reportModelError(out, errout, err)
	}
	printConfiguredResult(out, result, module, opts)
	if result.Failure != nil {
		return result.Failure.ExitCode
	}
	return exitCode(result.Status)
}

func checkModelWithOutput(ctx context.Context, opts options, module *parser.Module, parsed config.Config, specPath string, out, errout io.Writer) (checker.Result, error) {
	graph, err := openGraphDump(opts.dumpPath)
	if err != nil {
		return checker.Result{}, err
	}
	checkOptions := checker.Options{Loader: directoryLoader(filepath.Dir(specPath), opts.libraryPaths), OnPrint: func(line string) { fmt.Fprintln(out, line) }, OnPause: opts.onPause, ReadFile: readJSONFile, WriteFile: writeJSONFile, Seed: opts.seed, Fingerprint: opts.fingerprint, ModernRandomSubsets: opts.modernRandomSubsets}
	configureCheckingMode(&checkOptions, opts, module, out)
	if graph != nil {
		checkOptions.OnGraph = graph.event
	}
	checkOptions.OnSearch = configuredSearchOutput(opts, out)
	if opts.debugLiveness {
		checkOptions.OnDebug = func(line string) { fmt.Fprintln(errout, line) }
	}
	result, err := checker.CheckModuleWithOptions(ctx, module, parsed, checkOptions)
	if graph != nil {
		if closeErr := graph.close(graphShouldFinish(result, err, graph)); err == nil {
			err = closeErr
		}
	}
	return result, err
}

func graphShouldFinish(result checker.Result, err error, graph *dotDump) bool {
	return err == nil && !(result.Violation != nil && initialViolation(result)) && !(result.Failure != nil && len(graph.levels) == 0)
}

func reportModelError(out, errout io.Writer, err error) int {
	if modelErr, ok := errors.AsType[*checker.ModelValidationError](err); ok {
		fmt.Fprintf(out, "Error: %s\n", modelErr.Message)
		return modelErr.ExitCode
	}
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

func loadModule(specPath string, libraries []string, errout io.Writer) (*parser.Module, int) {
	source, err := os.ReadFile(specPath)
	if errors.Is(err, os.ErrNotExist) && filepath.Dir(specPath) == "." {
		for _, dir := range libraries {
			source, err = os.ReadFile(filepath.Join(dir, specPath))
			if !errors.Is(err, os.ErrNotExist) {
				break
			}
		}
	}
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

func readConfig(configPath string) (config.Config, error) {
	cfg, err := os.ReadFile(configPath)
	if err != nil {
		return config.Config{}, err
	}
	return config.Parse(string(cfg))
}

func directoryLoader(dir string, libraries []string) checker.ModuleLoader {
	return func(ctx context.Context, name string) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if filepath.Base(name) != name || name == "." || name == ".." {
			return "", fmt.Errorf("invalid module name %q", name)
		}
		for _, root := range append([]string{dir}, libraries...) {
			source, err := os.ReadFile(filepath.Join(root, name+".tla"))
			if err == nil {
				return string(source), nil
			}
			if !errors.Is(err, os.ErrNotExist) {
				return "", err
			}
		}
		return "", fmt.Errorf("module %s not found in search path: %w", name, os.ErrNotExist)
	}
}

func makeMetaDir(root string, now time.Time) (string, error) {
	if err := os.MkdirAll(root, 0o777); err != nil {
		return "", err
	}
	dir := filepath.Join(root, now.Format("06-01-02-15-04-05"))
	if err := os.Mkdir(dir, 0o777); err != nil {
		return "", err
	}
	return dir, nil
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
