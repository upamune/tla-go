package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

type toolOutput struct {
	out      io.Writer
	started  time.Time
	recorder *recorderConfig
}

func (t toolOutput) message(code, level int, body string) {
	fmt.Fprintf(t.out, "@!@!@STARTMSG %d:%d @!@!@\n%s", code, level, body)
	if !strings.HasSuffix(body, "\n") {
		fmt.Fprintln(t.out)
	}
	fmt.Fprintf(t.out, "@!@!@ENDMSG %d @!@!@\n", code)
}

func (t toolOutput) nestedError(code int, heading string, nestedCode int, detail string) {
	fmt.Fprintf(t.out, "@!@!@STARTMSG %d:1 @!@!@\n%s\n", code, heading)
	t.message(nestedCode, 0, detail)
	fmt.Fprintf(t.out, "@!@!@ENDMSG %d @!@!@\n", code)
}

func (t toolOutput) nestedInlineError(code int, heading string, nestedCode int, detail string) {
	fmt.Fprintf(t.out, "@!@!@STARTMSG %d:1 @!@!@\n", code)
	if heading != "" {
		fmt.Fprintf(t.out, "%s ", heading)
	}
	t.message(nestedCode, 0, detail)
	fmt.Fprintf(t.out, "@!@!@ENDMSG %d @!@!@\n", code)
}

func (t toolOutput) finish() {
	if t.recorder == nil {
		t.message(2186, 0, fmt.Sprintf("Finished in %dms at (%s)", time.Since(t.started).Milliseconds(), toolTime()))
		return
	}
	duration := fmt.Sprintf("%dms", time.Since(t.started).Milliseconds())
	recordStrings(t.recorder, 2186, duration)
	t.message(2186, 0, fmt.Sprintf("Finished in %s at (%s)", duration, toolTime()))
}

func toolTime() string { return time.Now().Format("2006-01-02 15:04:05") }

func printToolSearch(out io.Writer, event checker.SearchEvent) {
	t := toolOutput{out: out}
	if event.Kind == checker.TemporalCheckStart {
		t.message(2200, 0, temporalProgressStats(event))
	}
	code, text := searchEventMessage(event)
	t.message(code, 0, text)
}

// printHumanSearch prints an initial-state message without the tool envelope.
func printHumanSearch(out io.Writer, event checker.SearchEvent) {
	_, text := searchEventMessage(event)
	fmt.Fprintln(out, text)
}

func searchEventMessage(event checker.SearchEvent) (int, string) {
	switch event.Kind {
	case checker.SimulationInitialComplete, checker.InitialProgress:
		return 2269, fmt.Sprintf("Computed %d initial states...", event.GeneratedStates)
	case checker.InitialStart:
		return 2189, "Computing initial states..."
	case checker.SimulationLiveness, checker.ModelCheckingLiveness:
		return 2212, livenessBranches(event)
	case checker.TemporalCheckStart:
		return 2192, temporalStartMessage(event)
	}
	generated, distinct := event.GeneratedStates, event.DistinctStates
	if generated == distinct {
		return 2190, fmt.Sprintf("Finished computing initial states: %d distinct state%s generated at %s.", distinct, plural(distinct), toolTime())
	}
	return 2191, fmt.Sprintf("Finished computing initial states: %d state%s generated, with %d of them distinct at %s.", generated, plural(generated), distinct, toolTime())
}

func temporalStartMessage(event checker.SearchEvent) string {
	branches := ""
	if event.Branches != 1 {
		branches = fmt.Sprintf("%d branches of ", event.Branches)
	}
	return fmt.Sprintf("Checking %stemporal properties for the complete state space with %d total distinct states at (%s)", branches, event.Nodes, toolTime())
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func runToolModel(ctx context.Context, opts options, out, errout io.Writer) int {
	t := toolOutput{out: out, started: time.Now(), recorder: opts.recorder}
	defer t.finish()
	t.message(2262, 0, "tla-go "+version)
	printToolModeBanner(t, opts)
	specPath, configPath, err := inputPaths(opts)
	if err != nil {
		t.message(1000, 1, err.Error())
		return 255
	}
	endParsing := profileRegion(ctx, "parsing and config")
	parsed, err := readConfig(configPath)
	if err != nil {
		endParsing()
		printToolConfigError(t, configPath, err)
		return 255
	}
	recordStrings(t.recorder, 2220)
	t.message(2220, 0, "Starting SANY...")
	var diagnostic bytes.Buffer
	module, source, code := loadModuleWithSource(specPath, opts.libraryPaths, &diagnostic)
	endParsing()
	if code != 0 {
		return toolLoadFailure(t, &diagnostic, code)
	}
	recordStrings(t.recorder, 2219)
	t.message(2219, 0, "SANY finished.")
	if opts.deadlockOff {
		parsed.CheckDeadlock = false
	}
	recordStrings(t.recorder, 2185)
	t.message(2185, 0, fmt.Sprintf("Starting... (%s)", toolTime()))
	result, err := checkModelWithOutput(ctx, opts, module, source, parsed, specPath, out, errout)
	if err != nil {
		return toolModelError(t, err)
	}
	printConfiguredToolResult(t, result, module, opts)
	opts.compat.afterCheck(t, result, specTERun{module, specPath, configPath, parsed, directoryLoader(filepath.Dir(specPath), opts.libraryPaths)}, opts.simulate || opts.dfid)
	if result.Failure != nil {
		return result.Failure.ExitCode
	}
	return exitCode(result.Status)
}

func toolLoadFailure(t toolOutput, diagnostic *bytes.Buffer, code int) int {
	fmt.Fprint(t.out, diagnostic.String())
	recordStrings(t.recorder, 2219)
	t.message(2219, 0, "SANY finished.")
	recordStrings(t.recorder, 2185)
	t.message(2185, 0, fmt.Sprintf("Starting... (%s)", toolTime()))
	t.message(3002, 1, "Parsing or semantic analysis failed.")
	return code
}

func toolModelError(t toolOutput, err error) int {
	if errors.Is(err, errDebugStopped) {
		return 0
	}
	return toolModelErrorDetail(t, err)
}

func toolModelErrorDetail(t toolOutput, err error) int {
	if behaviorErr, ok := errors.AsType[*checker.SimulationBehaviorError](err); ok {
		printToolFailureHeader(t, behaviorErr.Failure)
		return 255
	}
	if modelErr, ok := errors.AsType[*checker.ModelValidationError](err); ok {
		t.message(modelErr.Code, 1, modelErr.Message)
		return modelErr.ExitCode
	}
	var syntaxErr *checker.SyntaxError
	var semanticErr *checker.SemanticError
	if errors.As(err, &syntaxErr) || errors.As(err, &semanticErr) {
		fmt.Fprintln(t.out, err)
		t.message(3002, 1, "Parsing or semantic analysis failed.")
		return 150
	}
	if configErr, ok := errors.AsType[*checker.ConfigurationError](err); ok {
		code := configErr.Code
		if code == 0 {
			code = 1000
		}
		printToolConfigurationError(t, code, configErr)
		return 151
	}
	t.message(1000, 1, err.Error())
	return 255
}

func printToolConfigurationError(t toolOutput, code int, err *checker.ConfigurationError) {
	if _, ok := errors.AsType[*evaluator.UnboundVariable](err.Err); code == 2281 && ok {
		if heading, detail, found := strings.Cut(err.Message, " - specifically: "); found {
			t.nestedInlineError(code, heading+" - specifically:", 2280, detail)
			return
		}
	}
	t.message(code, 1, err.Message)
}

func printToolResult(t toolOutput, result checker.Result, module *parser.Module) {
	if result.Status != checker.LivenessViolation && result.TemporalCheckDuration != nil && !result.TemporalCheckReported {
		printTemporalCheckEnd(t.out, true, *result.TemporalCheckDuration)
	}
	if result.Failure != nil {
		printToolFailure(t, result.Failure, module)
	} else if result.Violation != nil {
		printToolResultViolation(t, result, module)
	} else {
		var body bytes.Buffer
		printSuccess(&body, result)
		if t.recorder != nil {
			recordSuccess(t.recorder, result)
		}
		t.message(2193, 0, strings.TrimSuffix(body.String(), "\n"))
	}
	if toolHasStatistics(result) {
		printToolCoverage(t, result.Coverage)
		printToolStatistics(t, result)
	}
}

func printToolResultViolation(t toolOutput, result checker.Result, module *parser.Module) {
	if result.TemporalCheckReported && result.Status == checker.LivenessViolation {
		return
	}
	printToolViolation(t, result, module)
	if result.Status == checker.LivenessViolation && result.TemporalCheckDuration != nil {
		printTemporalCheckEnd(t.out, true, *result.TemporalCheckDuration)
	}
}

func toolHasStatistics(result checker.Result) bool {
	if result.Failure != nil {
		return !result.Failure.SuppressStatistics
	}
	return result.Violation == nil || result.Status != checker.AssumptionViolation && !initialViolation(result)
}
