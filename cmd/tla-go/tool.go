package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/syntax/parser"
)

type toolOutput struct {
	out     io.Writer
	started time.Time
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

func (t toolOutput) finish() {
	t.message(2186, 0, fmt.Sprintf("Finished in %dms at (%s)", time.Since(t.started).Milliseconds(), toolTime()))
}

func toolTime() string { return time.Now().Format("2006-01-02 15:04:05") }

func printToolSearch(out io.Writer, event checker.SearchEvent) {
	t := toolOutput{out: out}
	if event.Kind == checker.InitialStart {
		t.message(2189, 0, "Computing initial states...")
		return
	}
	generated, distinct := event.GeneratedStates, event.DistinctStates
	if generated == distinct {
		t.message(2190, 0, fmt.Sprintf("Finished computing initial states: %d distinct state%s generated at %s.", distinct, plural(distinct), toolTime()))
	} else {
		t.message(2191, 0, fmt.Sprintf("Finished computing initial states: %d state%s generated, with %d of them distinct at %s.", generated, plural(generated), distinct, toolTime()))
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func runToolModel(ctx context.Context, opts options, out, errout io.Writer) int {
	t := toolOutput{out: out, started: time.Now()}
	defer t.finish()
	t.message(2262, 0, "tla-go "+version)
	specPath, configPath, err := inputPaths(opts)
	if err != nil {
		t.message(1000, 1, err.Error())
		return 255
	}
	t.message(2220, 0, "Starting SANY-compatible analysis...")
	var diagnostic bytes.Buffer
	module, code := loadModule(specPath, opts.libraryPaths, &diagnostic)
	if code != 0 {
		return toolLoadFailure(t, &diagnostic, code)
	}
	t.message(2219, 0, "Analysis finished.")
	parsed, code := loadConfig(configPath, &diagnostic)
	if code != 0 {
		t.message(1000, 1, strings.TrimSpace(diagnostic.String()))
		return code
	}
	if opts.deadlockOff {
		parsed.CheckDeadlock = false
	}
	t.message(2185, 0, fmt.Sprintf("Starting... (%s)", toolTime()))
	result, err := checkModelWithOutput(ctx, opts, module, parsed, specPath, out, errout)
	if err != nil {
		return toolModelError(t, err)
	}
	printToolResult(t, result, module)
	if result.Failure != nil {
		return result.Failure.ExitCode
	}
	return exitCode(result.Status)
}

func toolLoadFailure(t toolOutput, diagnostic *bytes.Buffer, code int) int {
	fmt.Fprint(t.out, diagnostic.String())
	t.message(2219, 0, "Analysis finished.")
	t.message(3002, 1, "Parsing or semantic analysis failed.")
	return code
}

func toolModelError(t toolOutput, err error) int {
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
		t.message(code, 1, configErr.Message)
		return 151
	}
	t.message(1000, 1, err.Error())
	return 255
}

func printToolResult(t toolOutput, result checker.Result, module *parser.Module) {
	if result.Failure != nil {
		printToolFailure(t, result.Failure, module)
	} else if result.Violation != nil {
		printToolViolation(t, result, module)
	} else {
		var body bytes.Buffer
		printSuccess(&body, result)
		t.message(2193, 0, strings.TrimSuffix(body.String(), "\n"))
	}
	if toolHasStatistics(result) {
		printToolStatistics(t, result)
	}
}

func toolHasStatistics(result checker.Result) bool {
	if result.Failure != nil {
		return !result.Failure.SuppressStatistics
	}
	return result.Violation == nil || result.Status != checker.AssumptionViolation && !initialViolation(result)
}
