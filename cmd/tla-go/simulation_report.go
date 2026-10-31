package main

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/syntax/parser"
)

func printChecking(out io.Writer, specPath, configPath string, opts options) {
	fmt.Fprintf(out, "Checking %s with %s\n", filepath.Base(specPath), filepath.Base(configPath))
	if opts.simulate {
		fmt.Fprintf(out, "Running Random Simulation with seed %d.\n", opts.seed)
	}
}

func configureCheckingMode(checkOpts *checker.Options, opts options, module *parser.Module, out io.Writer) {
	if opts.simulate {
		checkOpts.Simulation = &checker.SimulationOptions{Traces: opts.simulationTraces, Depth: opts.simulationDepth, Aril: opts.aril, Continue: opts.continuation}
		if opts.continuation {
			checkOpts.Simulation.OnViolation = simulationViolationOutput(opts, module, out)
		}
	} else if opts.continuation {
		checkOpts.Continue = true
		checkOpts.OnViolation = modelViolationOutput(opts, module, out)
	}
}

func configuredSearchOutput(opts options, out io.Writer) func(checker.SearchEvent) {
	if opts.tool {
		return func(event checker.SearchEvent) { printToolSearch(out, event) }
	}
	if opts.simulate {
		return func(event checker.SearchEvent) {
			if event.Kind == checker.SimulationInitialComplete {
				fmt.Fprintf(out, "Computed %d initial states...\n", event.GeneratedStates)
			}
		}
	}
	return nil
}

func printConfiguredResult(out io.Writer, result checker.Result, module *parser.Module, opts options) {
	if opts.simulate {
		printSimulationResult(out, result, module, opts.seed, opts.aril)
	} else {
		printResult(out, result, module)
	}
}

func printConfiguredToolResult(t toolOutput, result checker.Result, module *parser.Module, opts options) {
	if opts.simulate {
		printToolSimulationResult(t, result, module, opts.seed, opts.aril)
	} else {
		printToolResult(t, result, module)
	}
}

func simulationViolationOutput(opts options, module *parser.Module, out io.Writer) func(checker.Result) {
	return func(result checker.Result) {
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
	if result.Status != checker.Success && result.Status != checker.ContinuedViolation {
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
	if failure.InitialState == nil || failure.ToolCode != 2102 && failure.ToolCode != 2106 {
		printToolFailure(t, failure, module)
		return
	}
	var body bytes.Buffer
	if failure.ToolCode == 2106 {
		fmt.Fprintln(&body, "State is not completely specified by the initial predicate:")
	} else {
		fmt.Fprintf(&body, "%s\nWhile working on the initial state:\n", failure.Message)
	}
	printStateOrdered(&body, failure.InitialState, failure.InitialStateOrder)
	t.message(failure.ToolCode, 1, body.String())
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
	if result.Status != checker.Success && result.Status != checker.ContinuedViolation {
		fmt.Fprintf(out, "The number of states generated: %d\nSimulation using seed %d and aril %d\n", result.GeneratedStates, seed, max(aril, 0))
	}
}
