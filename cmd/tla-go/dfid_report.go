package main

import (
	"bytes"
	"fmt"
	"io"

	"github.com/upamune/tla-go/checker"
)

// depthFirstMode reports whether TLC's -dfid search applies; -simulate wins.
func depthFirstMode(opts options) bool { return opts.dfid && !opts.simulate }

func depthFirstBanner(opts options) string {
	return fmt.Sprintf("Running depth-first search Model-Checking with fp %d and seed %d", opts.fingerprint, opts.seed)
}

// configureDepthFirstMode prints errors when TLC's depth-first checker
// reports them, because exploration can continue and print afterwards.
// TLC applies -terse but not -difftrace to depth-first traces.
func configureDepthFirstMode(checkOpts *checker.Options, opts options, out io.Writer) {
	style := traceStyle{terse: opts.compat.terse}
	report := func(result checker.Result) { printDepthFirstError(out, style.apply(result), opts.tool) }
	checkOpts.DepthFirst = &checker.DepthFirstOptions{MaxDepth: opts.dfidDepth, Workers: opts.workers, OnError: report}
	if opts.continuation {
		checkOpts.Continue, checkOpts.OnViolation = true, report
	}
}

func depthFirstSearchOutput(tool bool, out io.Writer) func(checker.SearchEvent) {
	return func(event checker.SearchEvent) {
		var code int
		var text string
		switch event.Kind {
		case checker.InitialComplete:
			code, text = 2208, fmt.Sprintf("Finished computing initial states: %d states generated, with %d of them distinct.", event.GeneratedStates, event.DistinctStates)
		case checker.DepthFirstLevel:
			code, text = 2205, fmt.Sprintf("Starting level %d: %d states generated, %d distinct states found.", event.Level, event.GeneratedStates, event.DistinctStates)
		default:
			return
		}
		if tool {
			toolOutput{out: out}.message(code, 0, text)
		} else {
			fmt.Fprintln(out, text)
		}
	}
}

func printDepthFirstError(out io.Writer, result checker.Result, tool bool) {
	if tool {
		printToolDepthFirstError(toolOutput{out: out}, result)
		return
	}
	if result.Failure != nil {
		printDepthFirstFailure(out, result.Failure)
	} else if initialViolation(result) {
		printInitialViolation(out, result)
	} else {
		printViolationHeadline(out, result)
		printDepthFirstTrace(out, result.Violation.Trace)
	}
}

func printDepthFirstFailure(w io.Writer, failure *checker.Failure) {
	if failure.Message == "" {
		return
	}
	fmt.Fprintf(w, "Error: %s\n", failure.Message)
	if failure.InitialState != nil {
		if failure.ToolCode == 2102 {
			fmt.Fprintln(w, "While working on the initial state:")
		}
		printStateOrdered(w, failure.InitialState, failure.InitialStateOrder)
	}
	if len(failure.Trace) > 0 {
		fmt.Fprintln(w, "Error: The behavior up to this point is:")
		printDepthFirstTrace(w, failure.Trace)
	}
}

// printDepthFirstTrace prints TLC's depth-first stack, whose states have no
// action labels.
func printDepthFirstTrace(w io.Writer, trace []checker.TraceStep) {
	for i, step := range trace {
		fmt.Fprintf(w, "State %d:\n", i+1)
		printTraceState(w, step)
	}
}

func printToolDepthFirstError(t toolOutput, result checker.Result) {
	if result.Failure != nil {
		printToolDepthFirstFailure(t, result.Failure)
	} else if initialViolation(result) {
		printToolInitialViolation(t, result)
	} else {
		printToolViolationHeadline(t, result)
		printToolDepthFirstTrace(t, result.Violation.Trace)
	}
}

func printToolDepthFirstFailure(t toolOutput, failure *checker.Failure) {
	if failure.Message == "" || printToolDepthFirstInitialFailure(t, failure) {
		return
	}
	printToolFailureHeader(t, failure)
	if len(failure.Trace) > 0 {
		t.message(2121, 1, "The behavior up to this point is:")
		printToolDepthFirstTrace(t, failure.Trace)
	}
}

// printToolDepthFirstInitialFailure prints TLC_INITIAL_STATE, which nests the
// tool envelope of a coded exception message.
func printToolDepthFirstInitialFailure(t toolOutput, failure *checker.Failure) bool {
	if failure.ToolCode == 2102 && failure.InitialState != nil && failure.NestedToolCode != 0 {
		printToolInitialNestedFailure(t, failure, failure.NestedToolCode, failure.NestedMessage)
		return true
	}
	return printToolInitialStateFailure(t, failure)
}

func printToolDepthFirstTrace(t toolOutput, trace []checker.TraceStep) {
	for i, step := range trace {
		t.message(2216, 4, depthFirstState(i+1, step))
	}
}

func depthFirstState(number int, step checker.TraceStep) string {
	var body bytes.Buffer
	fmt.Fprintf(&body, "%d:\n", number)
	printTraceState(&body, step)
	return body.String()
}

// printDepthFirstResult prints what TLC reports after the search: the nested
// expressions of a replayed error, success, and the state counts. Other
// errors were printed when TLC reported them.
func printDepthFirstResult(w io.Writer, result checker.Result) {
	if result.Status == checker.AssumptionViolation {
		printViolation(w, result, nil)
		return
	}
	if result.Failure != nil && result.Failure.EvaluationDetail != "" {
		fmt.Fprintf(w, "Error: %s\n", result.Failure.EvaluationDetail)
	}
	if result.Status == checker.Success {
		printDepthFirstSuccess(w, result)
	}
	if toolHasStatistics(result) {
		fmt.Fprintln(w, depthFirstCounts(result))
	}
}

func printToolDepthFirstResult(t toolOutput, result checker.Result) {
	if result.Status == checker.AssumptionViolation {
		printToolAssumption(t, result.Violation)
		return
	}
	if result.Failure != nil && result.Failure.EvaluationDetail != "" {
		t.message(2103, 1, result.Failure.EvaluationDetail)
	}
	if result.Status == checker.Success {
		var body bytes.Buffer
		printDepthFirstSuccess(&body, result)
		t.message(2193, 0, body.String())
	}
	if toolHasStatistics(result) {
		t.message(2206, 0, fmt.Sprintf("Progress(-1) at %s: %d states generated, %d distinct states found, -1 states left on queue.", toolTime(), result.GeneratedStates, result.DistinctStates))
		t.message(2204, 0, depthFirstCounts(result))
	}
}

func depthFirstCounts(result checker.Result) string {
	return fmt.Sprintf("%d states generated, %d distinct states found.", result.GeneratedStates, result.DistinctStates)
}

// printDepthFirstSuccess adds TLC's probability based on the closest pair of
// state fingerprints.
func printDepthFirstSuccess(w io.Writer, result checker.Result) {
	printSuccess(w, result)
	observed := "0.0"
	if result.GeneratedStates != 0 || result.DistinctStates != 0 {
		observed = formatProbability(observedProbability(result.FingerprintDistance))
	}
	if result.HasFingerprintDistance {
		fmt.Fprintf(w, "  based on the actual fingerprints:  val = %s\n", observed)
	}
}
