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
	path, configPath             string
	paths                        []string
	libraryPaths                 []string
	metaRoot                     string
	metaDir, recoverDir          string
	checkpointInterval           time.Duration
	deadlockOff, help, version   bool
	debugLiveness                bool
	tracePath, cpuPath, heapPath string
	coverage                     bool
	coverageInterval             time.Duration
	checkpoint                   bool
	modernRandomSubsets          bool
	tool                         bool
	noWarning                    bool
	view                         bool
	sanyMetadata                 bool
	compat                       compatOptions
	dumpPath                     string
	dumpDOT                      *dotFormat
	onPause                      func()
	seed                         int64
	aril                         int64
	fingerprint                  int
	workers                      int
	maxSetSize                   int
	simulate, seedSet            bool
	continuation                 bool
	simulationTraces             int
	simulationDepth              int
	simulationUnlimited          bool
	simulationFile               string
	simulationFileSet            bool
	dfid                         bool
	dfidDepth                    int
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage: tla-go [-DTLA-Library=paths] [-metadir dir] [-config file.cfg] [-deadlock] [-seed integer] [-aril integer] [-simulate [num=N,file=prefix]] [-dfid N] [-continue] [-depth N] [-workers N|auto] [-checkpoint minutes] [-recover dir] [-coverage N] [-maxSetSize N] [-fp N] [-tool] [-nowarning] [-view] [-dump [dot[,colorize][,actionlabels]] file] [-debug-liveness] [-modern-random-subsets] <file.tla|file.cfg|glob>...\n       tla-go -translate [pcal.trans options] file.tla\n       tla-go -sany-metadata [-I dir] file.tla\n       tla-go -version\n\nExperimental: supports a subset of TLA+ model checking and simulation.\n-dfid N checks with depth-first iterative deepening up to depth N.\n-continue reports supported violations and continues exploration.\n-translate, which must come first, translates PlusCal like Java's pcal.trans; tla-go -translate -help lists its options.\n-sany-metadata emits a partial SANY XML projection.\n-debug-liveness writes JSON debugging events to stderr.\n-debug-trace FILE, -debug-cpu-profile FILE, and -debug-heap-profile FILE write Go diagnostics to new files.\n-modern-random-subsets selects the newer TLC random-subset algorithm.")
	fmt.Fprintln(w, compatUsage)
}

func run(ctx context.Context, args []string, out, errout io.Writer) int {
	if code, ok := runTranslator(args, out, errout); ok {
		return code
	}
	return runWithInput(ctx, args, os.Stdin, out, errout)
}

func validateOutputPaths(opts options, out io.Writer) error {
	if err := validateProfilePaths(opts); err != nil {
		return err
	}
	return checkDumpPath(opts.dumpPath, out)
}

func runWithInput(ctx context.Context, args []string, input io.Reader, out, errout io.Writer) (code int) {
	opts, err := parseOptions(args)
	if err == nil {
		err = validateOutputPaths(opts, out)
	}
	if err != nil {
		return reportOptionError(opts, err, out, errout)
	}
	profiles, err := startProfiles(opts)
	if err != nil {
		fmt.Fprintln(errout, err)
		return 255
	}
	defer func() {
		if err := profiles.close(); err != nil {
			fmt.Fprintln(errout, err)
			if code == 0 {
				code = 255
			}
		}
	}()
	ctx = profiles.context(ctx)
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
	runOut, finish, err := startCompatOutput(&opts, out)
	if err != nil {
		// Java TLC creates the -userFile file while reading its parameters.
		return reportOptionError(options{tool: opts.compat.userFileTool}, err, out, errout)
	}
	defer finish()
	reader := bufio.NewReader(input)
	opts.onPause = func() { _, _ = reader.ReadString('\n') }
	return runModels(ctx, opts, runOut, errout)
}

// reportOptionError follows TLC.printErrorMsg, which reports Java TLC's
// parameter errors (WRONG_COMMANDLINE_PARAMS_TLC) with TLC's usage line on
// standard output, as message 1102 when -tool precedes the failing option.
// Other errors are tla-go's own and go to standard error with its usage.
func reportOptionError(opts options, err error, out, errout io.Writer) int {
	if _, ok := errors.AsType[commandLineError](err); !ok {
		fmt.Fprintln(errout, err)
		usage(errout)
		return 255
	}
	body := err.Error() + "\nUsage: java tlc2.TLC [-option] inputfile"
	if !opts.tool {
		fmt.Fprintf(out, "Error: %s\n", body)
		return 1
	}
	t := toolOutput{out: out}
	t.message(2262, 0, "tla-go "+version)
	t.message(1102, 1, body)
	return 1
}

func runModel(ctx context.Context, opts options, out, errout io.Writer) (code int) {
	if opts.recoverDir != "" {
		opts.metaDir = opts.recoverDir
	} else if opts.metaRoot != "" || opts.checkpointInterval > 0 {
		root := opts.metaRoot
		if root == "" {
			root = filepath.Join(filepath.Dir(opts.path), "states")
		}
		metaDir, err := makeMetaDir(root, time.Now())
		if err != nil {
			return checkpointMetaError(ctx, opts, out, errout, err)
		}
		opts.metaDir = metaDir
		defer func() {
			if code == 0 {
				_ = os.RemoveAll(metaDir)
			}
		}()
		if after, ok := strings.CutPrefix(opts.dumpPath, "${metadir}"); ok {
			opts.dumpPath = metaDir + after
		}
	}
	if err := protectProfileOutput(opts, opts.dumpPath); err != nil {
		fmt.Fprintln(errout, err)
		return 255
	}
	return runConfiguredModel(ctx, opts, out, errout)
}

func checkpointMetaError(ctx context.Context, opts options, out, errout io.Writer, err error) int {
	if opts.metaRoot == "" && opts.checkpointInterval > 0 {
		fmt.Fprintf(errout, "Checkpointing unavailable (cannot create metadata directory: %s); checking normally without checkpoints.\n", err)
		opts.checkpointInterval = 0
		return runConfiguredModel(ctx, opts, out, errout)
	}
	fmt.Fprintf(errout, "Cannot create metadata directory: %s\n", err)
	return 255
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
	endParsing := profileRegion(ctx, "parsing and config")
	parsed, err := readConfig(configPath)
	if err != nil {
		endParsing()
		printConfigError(out, configPath, err)
		return 255
	}
	module, source, code := loadModuleWithSource(specPath, opts.libraryPaths, errout)
	endParsing()
	if code != 0 {
		return code
	}
	if opts.deadlockOff {
		parsed.CheckDeadlock = false
	}
	printChecking(out, specPath, configPath, opts)
	result, err := checkModelWithOutput(ctx, opts, module, source, parsed, specPath, out, errout)
	if err != nil {
		return reportModelError(out, errout, err)
	}
	printConfiguredResult(out, result, module, opts)
	if result.Failure != nil {
		return result.Failure.ExitCode
	}
	return exitCode(result.Status)
}

func checkModelWithOutput(ctx context.Context, opts options, module *parser.Module, source []byte, parsed config.Config, specPath string, out, errout io.Writer) (checker.Result, error) {
	defer profileRegion(ctx, "model checking")()
	if opts.checkpointInterval > 0 || opts.recoverDir != "" {
		result, handled, err := checkCLIWithCheckpoint(ctx, opts, parsed, source, specPath, out, errout)
		if handled {
			return result, err
		}
	}
	return checkModelNormally(ctx, opts, module, parsed, specPath, out, errout)
}

func checkModelNormally(ctx context.Context, opts options, module *parser.Module, parsed config.Config, specPath string, out, errout io.Writer) (checker.Result, error) {
	graph, err := openStateDump(opts.dumpPath, opts.dumpDOT)
	if err != nil {
		return checker.Result{}, err
	}
	checkOptions := checker.Options{
		Loader: directoryLoader(filepath.Dir(specPath), opts.libraryPaths), OnPrint: func(line string) { fmt.Fprintln(out, line) }, OnPause: opts.onPause, ReadFile: readJSONFile, WriteFile: func(path, text string) error {
			if err := protectProfileOutput(opts, path); err != nil {
				return err
			}
			return writeJSONFile(path, text)
		}, Seed: opts.seed, Fingerprint: opts.fingerprint, ModernRandomSubsets: opts.modernRandomSubsets, MaxSetSize: opts.maxSetSize, Workers: opts.workers, ViewStates: opts.view,
		Coverage:          opts.coverage,
		SerialExploration: opts.checkpoint,
	}
	configureCheckingMode(&checkOptions, opts, module, out)
	configureCompatOutput(&checkOptions, opts)
	if graph != nil {
		checkOptions.OnGraph = styledGraphEvents(graph.event, traceStyle{terse: opts.compat.terse})
	}
	checkOptions.OnSearch = configuredSearchOutput(opts, out)
	checkOptions.OnTemporalViolation = func(result checker.Result) {
		result = newTraceStyle(opts).apply(result)
		if opts.tool {
			printToolViolation(toolOutput{out: out}, result, module)
		} else {
			printViolation(out, result, module)
		}
	}
	checkOptions.OnTemporalCheck = func(result checker.Result) {
		if result.Status == checker.LivenessViolation && result.Violation != nil && result.Violation.Unwitnessed != nil {
			if opts.tool {
				failure := result.Violation.Unwitnessed
				if failure.Counts != "" {
					fmt.Fprintln(out, failure.Counts)
				}
				(toolOutput{out: out}).message(1000, 1, failure.Message)
			} else {
				printUnwitnessed(out, result.Violation.Unwitnessed)
			}
		}
		printTemporalCheckEnd(out, opts.tool, *result.TemporalCheckDuration)
	}
	checkOptions.OnWarning = configuredWarningOutput(opts, out)
	if opts.debugLiveness {
		checkOptions.OnDebug = func(line string) { fmt.Fprintln(errout, line) }
	}
	var result checker.Result
	if opts.coverageInterval == 0 {
		result, err = checker.CheckModuleWithOptions(ctx, module, parsed, checkOptions)
	} else {
		result, err = checkWithCoverageDeadline(ctx, opts.coverageInterval, context.WithTimeout, func(ctx context.Context) (checker.Result, error) {
			return checker.CheckModuleWithOptions(ctx, module, parsed, checkOptions)
		})
	}
	if graph != nil {
		if closeErr := graph.close(graphShouldFinish(result, err)); err == nil {
			err = closeErr
		}
	}
	return result, err
}

func graphShouldFinish(result checker.Result, err error) bool {
	// Incomplete initial states and initial violations take TLC's early return
	// path; exceptions during initial evaluation close the writer instead.
	return err == nil && !(result.Violation != nil && initialViolation(result)) &&
		!(result.Failure != nil && result.Failure.IncompleteInitialState)
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
	module, _, code := loadModuleWithSource(specPath, libraries, errout)
	return module, code
}

func loadModuleWithSource(specPath string, libraries []string, errout io.Writer) (*parser.Module, []byte, int) {
	source, err := readRootModule(specPath, libraries)
	if err != nil {
		fmt.Fprintf(errout, "Cannot find the specification file %s\n", specPath)
		return nil, nil, 151
	}
	module, err := parser.ParseModule(string(source))
	if err != nil {
		return nil, nil, reportError(errout, &checker.SyntaxError{Err: err})
	}
	if err := validateModuleName(specPath, module.Name); err != nil {
		return nil, nil, reportError(errout, &checker.SyntaxError{Err: err})
	}
	return module, source, 0
}

func readRootModule(specPath string, libraries []string) ([]byte, error) {
	source, err := os.ReadFile(specPath)
	if errors.Is(err, os.ErrNotExist) && filepath.Dir(specPath) == "." {
		for _, dir := range libraries {
			source, err = os.ReadFile(filepath.Join(dir, specPath))
			if !errors.Is(err, os.ErrNotExist) {
				break
			}
		}
	}
	return source, err
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
	if err := os.Mkdir(dir, 0o777); err == nil {
		return dir, nil
	} else if !errors.Is(err, os.ErrExist) {
		return "", err
	}
	return os.MkdirTemp(root, now.Format("06-01-02-15-04-05")+"-")
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
	} else {
		cfgPath = configFilePath(path, cfgPath, opts.libraryPaths)
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
	case checker.Success, checker.DepthLimit:
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
