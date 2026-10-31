package main

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func printResult(w io.Writer, result checker.Result, module *parser.Module) {
	if result.Failure != nil {
		printFailure(w, result.Failure, module)
		if result.Failure.SuppressStatistics {
			return
		}
	} else if result.Violation == nil {
		printSuccess(w, result)
	} else if printViolation(w, result, module) {
		return
	}
	printStatistics(w, result)
}

func printFailure(w io.Writer, failure *checker.Failure, module *parser.Module) {
	fmt.Fprintf(w, "Error: %s\n", failure.Message)
	if failure.InitialState != nil {
		fmt.Fprintln(w, "While working on the initial state:")
		printStateOrdered(w, failure.InitialState, failure.InitialStateOrder)
	}
	if len(failure.Trace) > 0 {
		fmt.Fprintln(w, "Error: The behavior up to this point is:")
		printTrace(w, failure.Trace, module)
	}
	if failure.FingerprintDetail != "" {
		fmt.Fprintf(w, "Error: %s\n", failure.FingerprintDetail)
	}
	if failure.EvaluationDetail != "" {
		fmt.Fprintf(w, "Error: %s\n", failure.EvaluationDetail)
	}
}

func printSuccess(w io.Writer, result checker.Result) {
	probability := float64(result.DistinctStates) * (float64(result.GeneratedStates-result.DistinctStates) / 18446744073709551616.0)
	fmt.Fprintln(w, "Model checking completed. No error has been found.")
	fmt.Fprintln(w, "  Estimates of the probability that TLC did not check all reachable states")
	fmt.Fprintln(w, "  because two distinct states had the same fingerprint:")
	fmt.Fprintf(w, "  calculated (optimistic):  val = %s\n", formatProbability(probability))
}

// printViolation returns true when TLC suppresses statistics for the result.
func printViolation(w io.Writer, result checker.Result, module *parser.Module) bool {
	violation := result.Violation
	if result.Status == checker.PostconditionViolation {
		if violation.Counts != "" {
			fmt.Fprintln(w, violation.Counts)
		}
		fmt.Fprintf(w, "Error: %s\n", violation.Message)
		return false
	}
	if result.Status == checker.AssumptionViolation {
		if violation.Message != "" {
			fmt.Fprintf(w, "Error: Evaluating assumption %s failed.\n%s\n", violation.Name, violation.Message)
		} else {
			fmt.Fprintf(w, "Error: Assumption %s is false.\n", violation.Name)
		}
		return true
	}
	if initialViolation(result) {
		printInitialViolation(w, result)
		return true
	}
	printViolationHeadline(w, result)
	printTrace(w, violation.Trace, module)
	printLoop(w, violation, module)
	printUnwitnessed(w, violation.Unwitnessed)
	return false
}

func printInitialViolation(w io.Writer, result checker.Result) {
	violation := result.Violation
	kind := "Invariant"
	if result.Status == checker.LivenessViolation {
		kind = "Property"
	}
	fmt.Fprintf(w, "Error: %s %s is violated by the initial state:\n", kind, violation.Name)
	printTraceState(w, violation.Trace[0])
	printUnwitnessed(w, violation.Unwitnessed)
}

func printUnwitnessed(w io.Writer, failure *checker.PossibleViolation) {
	if failure == nil {
		return
	}
	if failure.Counts != "" {
		fmt.Fprintln(w, failure.Counts)
	}
	fmt.Fprintf(w, "Error: %s\n", failure.Message)
}

func initialViolation(result checker.Result) bool {
	return !result.Violation.AfterInitial && result.Violation.LoopStart == nil && len(result.Violation.Trace) == 1 && (result.Status == checker.InvariantViolation || result.Status == checker.LivenessViolation)
}

func printViolationHeadline(w io.Writer, result checker.Result) {
	v := result.Violation
	switch result.Status {
	case checker.Deadlock:
		fmt.Fprintln(w, "Error: Deadlock reached.")
	case checker.InvariantViolation:
		fmt.Fprintf(w, "Error: Invariant %s is violated.\n", v.Name)
	case checker.PropertyViolation:
		fmt.Fprintln(w, temporalHeadline(v))
		fmt.Fprintln(w)
	case checker.LivenessViolation:
		if v.LoopStart == nil {
			fmt.Fprintf(w, "Error: Action property %s is violated.\n", v.Name)
		} else {
			fmt.Fprintln(w, temporalHeadline(v))
			fmt.Fprintln(w)
		}
	}
	if v.LoopStart == nil && result.Status != checker.PropertyViolation {
		fmt.Fprintln(w, "Error: The behavior up to this point is:")
	} else {
		fmt.Fprint(w, "Error: The following behavior constitutes a counter-example:\n\n")
	}
}

func temporalHeadline(violation *checker.Violation) string {
	names := violation.Properties
	if len(names) == 0 {
		names = []string{violation.Name}
	}
	if len(names) == 1 {
		return fmt.Sprintf("Error: Temporal property %s was violated.", names[0])
	}
	separator := " and "
	if len(names) > 2 {
		separator = ", and "
	}
	return fmt.Sprintf("Error: Temporal properties %s%s%s were violated.", strings.Join(names[:len(names)-1], ", "), separator, names[len(names)-1])
}

func printTrace(w io.Writer, trace []checker.TraceStep, module *parser.Module) {
	for i, step := range trace {
		fmt.Fprintf(w, "State %d: <%s>\n", i+1, actionLabelWithRange(module, step.Action, step.ActionRange))
		printTraceState(w, step)
	}
}

// printTraceState prints a trace state, or its VIEW text under -view.
func printTraceState(w io.Writer, step checker.TraceStep) {
	if step.View != "" {
		fmt.Fprintln(w, step.View)
		return
	}
	printStateOrdered(w, step.State, step.StateOrder)
}

func printStateOrdered(w io.Writer, state evaluator.State, order []string) {
	names := stateNames(state, order)
	prefix := ""
	if len(names) > 1 || len(order) > 1 {
		prefix = "/\\ "
	}
	for _, name := range names {
		fmt.Fprintf(w, "%s%s = %s\n", prefix, name, stateValueText(state[name]))
	}
	fmt.Fprintln(w)
}

// stateValueText renders a variable's value as Java TLC's
// TLCStateMut.toString does, in Values.ppr's 80-column layout; a value
// that is not yet defined prints as null.
func stateValueText(v value.Value) string {
	if v.Kind() == value.Invalid {
		return "null"
	}
	return value.PrettyPrint(v.String())
}

// stateNames returns the variables to print in display order. An order
// longer than the state lists every variable of a -difftrace step whose
// state holds only the changed variables.
func stateNames(state evaluator.State, order []string) []string {
	if len(order) == len(state) {
		return order
	}
	if len(order) > len(state) {
		return slices.DeleteFunc(slices.Clone(order), func(name string) bool {
			_, ok := state[name]
			return !ok
		})
	}
	return sortedStateNames(state)
}

func printLoop(w io.Writer, violation *checker.Violation, module *parser.Module) {
	if violation.LoopStart == nil {
		return
	}
	if *violation.LoopStart == len(violation.Trace)-1 {
		fmt.Fprintf(w, "State %d: Stuttering\n", *violation.LoopStart+2)
		return
	}
	fmt.Fprintf(w, "Back to state %d: <%s>\n\n", *violation.LoopStart+1, actionLabelWithRange(module, violation.LoopAction, violation.LoopActionRange))
}

func actionLabelWithRange(module *parser.Module, action string, location parser.Range) string {
	if location.Start.Line > 0 {
		return formatActionLocation(action, location)
	}
	return actionLabel(module, action)
}

func actionLabel(module *parser.Module, action string) string {
	if action == "" {
		return "Initial predicate"
	}
	for _, definition := range module.Definitions {
		if definition.Name == action && definition.BodyRange.Start.Line > 0 {
			r := actionBodyRange(definition)
			return formatActionLocation(action, r)
		}
	}
	return action
}

func formatActionLocation(action string, r parser.Range) string {
	return fmt.Sprintf("%s line %d, col %d to line %d, col %d of module %s", action, r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
}

func actionBodyRange(definition parser.Definition) parser.Range {
	body := definition.Body
	for body.Kind == "Let" && len(body.Args) > 0 {
		body = body.Args[0]
	}
	if body.Location.Range.Start.Line > 0 {
		return body.Location.Range
	}
	return definition.BodyRange
}

func printStatistics(w io.Writer, result checker.Result) {
	fmt.Fprintf(w, "%d states generated, %d distinct states found, %d states left on queue.\n", result.GeneratedStates, result.DistinctStates, result.QueuedStates)
	if result.Failure != nil && result.Failure.SuppressDepth {
		return
	}
	fmt.Fprintf(w, "The depth of the complete state graph search is %d.\n", result.Depth)
	if result.HasOutDegrees {
		if summary := outDegreeSummary(result.OutDegrees[:]); summary != "" {
			fmt.Fprintln(w, summary)
		}
	}
}
