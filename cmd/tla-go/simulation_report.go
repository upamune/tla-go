package main

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"runtime"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/syntax/parser"
)

func printChecking(out io.Writer, specPath, configPath string, opts options) {
	fmt.Fprintf(out, "Checking %s with %s\n", filepath.Base(specPath), filepath.Base(configPath))
	if opts.simulate {
		fmt.Fprintf(out, "Running Random Simulation with seed %d.\n", opts.seed)
	} else if opts.dfid {
		fmt.Fprintln(out, depthFirstBanner(opts)+".")
	}
}

func printToolModeBanner(t toolOutput, opts options) {
	workers := fmt.Sprintf(" with %d worker%s on %d cores.", opts.workers, plural(opts.workers), runtime.NumCPU())
	switch {
	case opts.simulate:
		t.message(2188, 0, fmt.Sprintf("Running Random Simulation with seed %d with 1 worker.", opts.seed))
	case opts.dfid:
		t.message(2271, 0, depthFirstBanner(opts)+workers)
	default:
		t.message(2187, 0, fmt.Sprintf("Running breadth-first search Model-Checking with fp %d and seed %d", opts.fingerprint, opts.seed)+workers)
	}
}

func configureCheckingMode(checkOpts *checker.Options, opts options, module *parser.Module, out io.Writer) {
	if opts.simulate {
		checkOpts.Simulation = &checker.SimulationOptions{Traces: opts.simulationTraces, Depth: opts.simulationDepth, UnlimitedTraces: opts.simulationUnlimited, Aril: opts.aril, Continue: opts.continuation}
		if opts.continuation {
			checkOpts.Simulation.OnViolation = simulationViolationOutput(opts, module, out)
		}
		if opts.simulationFileSet {
			checkOpts.Simulation.OnTrace = simulationTraceFile(opts.simulationFile)
		}
	} else if opts.dfid {
		configureDepthFirstMode(checkOpts, opts, out)
	} else if opts.continuation {
		checkOpts.Continue = true
		checkOpts.OnViolation = modelViolationOutput(opts, module, out)
	}
}

func configuredSearchOutput(opts options, out io.Writer) func(checker.SearchEvent) {
	if depthFirstMode(opts) {
		return depthFirstSearchOutput(opts.tool, out)
	}
	if opts.tool {
		return func(event checker.SearchEvent) { printToolSearch(out, event) }
	}
	if opts.simulate {
		return func(event checker.SearchEvent) {
			if event.Kind == checker.SimulationInitialComplete {
				printHumanSearch(out, event)
			}
		}
	}
	return nil
}

func printConfiguredResult(out io.Writer, result checker.Result, module *parser.Module, opts options) {
	result = newTraceStyle(opts).apply(result)
	if opts.simulate {
		printSimulationResult(out, result, module, opts.seed, opts.aril)
	} else if opts.dfid {
		printDepthFirstResult(out, result)
	} else {
		printResult(out, result, module)
	}
}

func printConfiguredToolResult(t toolOutput, result checker.Result, module *parser.Module, opts options) {
	result = newTraceStyle(opts).apply(result)
	if opts.simulate {
		printToolSimulationResult(t, result, module, opts.seed, opts.aril)
	} else if opts.dfid {
		printToolDepthFirstResult(t, result)
	} else {
		printToolResult(t, result, module)
	}
}

func simulationViolationOutput(opts options, module *parser.Module, out io.Writer) func(checker.Result) {
	return func(result checker.Result) {
		result = newTraceStyle(opts).apply(result)
		if opts.tool {
			t := toolOutput{out: out}
			printToolViolation(t, result, module)
			printToolSimulationProgress(t, result.GeneratedStates)
			t.message(2210, 0, fmt.Sprintf("The number of states generated: %d\nSimulation using seed %d and aril %d", result.GeneratedStates, opts.seed, max(opts.aril, 0)))
			return
		}
		printViolation(out, result, module)
		fmt.Fprintf(out, "Progress: %d states generated.\nThe number of states generated: %d\nSimulation using seed %d and aril %d\n", result.GeneratedStates, result.GeneratedStates, opts.seed, max(opts.aril, 0))
	}
}

func modelViolationOutput(opts options, module *parser.Module, out io.Writer) func(checker.Result) {
	return func(result checker.Result) {
		result = newTraceStyle(opts).apply(result)
		if opts.tool {
			printToolViolation(toolOutput{out: out}, result, module)
		} else {
			printViolation(out, result, module)
		}
	}
}

func printToolSimulationResult(t toolOutput, result checker.Result, module *parser.Module, seed, aril int64) {
	if result.Failure != nil {
		printToolSimulationFailure(t, result.Failure, module)
	} else if result.Violation != nil {
		printToolViolation(t, result, module)
	}
	if !simulationHasStatistics(result) {
		return
	}
	printToolSimulationProgress(t, result.GeneratedStates)
	if simulationHasSummary(result) {
		t.message(2210, 0, fmt.Sprintf("The number of states generated: %d\nSimulation using seed %d and aril %d", result.GeneratedStates, seed, max(aril, 0)))
		if result.Failure == nil || result.Failure.InitialState == nil {
			printToolSimulationProgress(t, result.GeneratedStates)
		}
	}
}

func printToolSimulationProgress(t toolOutput, generated int) {
	t.message(2209, 0, fmt.Sprintf("Progress(-1) at %s: %d states generated, -1 distinct states found, -1 states left on queue.", toolTime(), generated))
}

func printToolSimulationFailure(t toolOutput, failure *checker.Failure, module *parser.Module) {
	if failure.ToolCode == 2111 || failure.ToolCode == 2113 {
		t.message(failure.ToolCode, 1, failure.Message)
		t.message(2121, 1, "The behavior up to this point is:")
		printToolTrace(t, failure.Trace, module)
		return
	}
	if !printToolInitialStateFailure(t, failure) {
		printToolFailure(t, failure, module)
	}
}

// printToolInitialStateFailure prints an initial state that failed to
// evaluate or left variables unassigned.
func printToolInitialStateFailure(t toolOutput, failure *checker.Failure) bool {
	if failure.InitialState == nil || failure.ToolCode != 2102 && failure.ToolCode != 2106 {
		return false
	}
	if printToolSimulationInitialNested(t, failure) {
		return true
	}
	var body bytes.Buffer
	if failure.ToolCode == 2106 {
		fmt.Fprintln(&body, "State is not completely specified by the initial predicate:")
	} else {
		fmt.Fprintf(&body, "%s\nWhile working on the initial state:\n", failure.Message)
	}
	printStateOrdered(&body, failure.InitialState, failure.InitialStateOrder)
	t.message(failure.ToolCode, 1, body.String())
	return true
}

func printToolSimulationInitialNested(t toolOutput, failure *checker.Failure) bool {
	if failure.ToolCode != 2102 || failure.NestedToolCode != 2215 {
		return false
	}
	printToolInitialNestedFailure(t, failure, 2215, failure.NestedMessage)
	return true
}

// simulationHasSummary reports whether Java TLC's simulator prints its summary
// after a stopped run. It omits the summary after a TLC assertion, such as a
// non-boolean action (2247) or a failed Java override (2154), raised while
// generating successors.
func simulationHasSummary(result checker.Result) bool {
	if result.Status == checker.Success || result.Status == checker.ContinuedViolation {
		return false
	}
	failure := result.Failure
	return failure == nil || len(failure.Trace) == 0 || failure.ToolCode != 2247 && failure.ToolCode != 2154
}

func simulationHasStatistics(result checker.Result) bool {
	if result.Failure != nil {
		return !result.Failure.SuppressStatistics
	}
	return result.Violation == nil || result.Status != checker.AssumptionViolation && !initialViolation(result)
}

func printSimulationResult(out io.Writer, result checker.Result, module *parser.Module, seed, aril int64) {
	if result.Failure != nil {
		printFailure(out, result.Failure, module)
	} else if result.Violation != nil {
		printViolation(out, result, module)
	}
	if !simulationHasStatistics(result) {
		return
	}
	fmt.Fprintf(out, "Progress: %d states generated.\n", result.GeneratedStates)
	if simulationHasSummary(result) {
		fmt.Fprintf(out, "The number of states generated: %d\nSimulation using seed %d and aril %d\n", result.GeneratedStates, seed, max(aril, 0))
	}
}
