package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/syntax/parser"
)

func printToolFailure(t toolOutput, failure *checker.Failure, module *parser.Module) {
	code := failure.ToolCode
	if code == 0 {
		code = 1000
	}
	if failure.NestedToolCode != 0 {
		if failure.NestedMessage != "" {
			heading := strings.TrimSuffix(failure.Message, failure.NestedMessage)
			heading = strings.TrimSuffix(heading, " ")
			t.nestedInlineError(code, heading, failure.NestedToolCode, failure.NestedMessage)
		} else {
			heading, detail, _ := strings.Cut(failure.Message, "\n")
			t.nestedError(code, heading, failure.NestedToolCode, detail)
		}
	} else {
		t.message(code, 1, failure.Message)
	}
	if failure.InitialState != nil {
		var body bytes.Buffer
		printStateOrdered(&body, failure.InitialState, failure.InitialStateOrder)
		t.message(2102, 1, "The initial state is:\n"+body.String())
	}
	if len(failure.Trace) > 0 {
		t.message(2121, 1, "The behavior up to this point is:")
		printToolTrace(t, failure.Trace, module)
	}
	if failure.FingerprintDetail != "" {
		t.message(1000, 1, failure.FingerprintDetail)
	}
	if failure.EvaluationDetail != "" {
		t.message(2103, 1, failure.EvaluationDetail)
	}
}

func printToolViolation(t toolOutput, result checker.Result, module *parser.Module) {
	v := result.Violation
	if result.Status == checker.AssumptionViolation {
		printToolAssumption(t, v)
		return
	}
	if initialViolation(result) {
		printToolInitialViolation(t, result)
		return
	}
	if result.Status == checker.PostconditionViolation {
		t.message(1000, 1, v.Message)
		return
	}
	printToolViolationHeadline(t, result)
	printToolTrace(t, v.Trace, module)
	printToolLoop(t, v, module)
	if v.Unwitnessed != nil {
		t.message(1000, 1, v.Unwitnessed.Message)
	}
}

func printToolAssumption(t toolOutput, v *checker.Violation) {
	if v.Message != "" {
		heading := fmt.Sprintf("Evaluating assumption %s failed.", v.Name)
		if v.ToolCode != 0 {
			t.nestedError(2105, heading, v.ToolCode, v.Message)
		} else {
			t.message(2105, 1, heading+"\n"+v.Message)
		}
	} else {
		t.message(2104, 1, fmt.Sprintf("Assumption %s is false.", v.Name))
	}
}

func printToolInitialViolation(t toolOutput, result checker.Result) {
	v := result.Violation
	code, kind := 2107, "Invariant"
	if result.Status == checker.LivenessViolation {
		code, kind = 2108, "Property"
	}
	var body bytes.Buffer
	fmt.Fprintf(&body, "%s %s is violated by the initial state:\n", kind, v.Name)
	printStateOrdered(&body, v.Trace[0].State, v.Trace[0].StateOrder)
	t.message(code, 1, body.String())
}

func printToolViolationHeadline(t toolOutput, result checker.Result) {
	v := result.Violation
	if result.Status == checker.Deadlock {
		t.message(2114, 1, "Deadlock reached.")
	} else if result.Status == checker.InvariantViolation {
		t.message(2110, 1, fmt.Sprintf("Invariant %s is violated.", v.Name))
	} else if result.Status == checker.LivenessViolation && v.LoopStart == nil {
		t.message(2112, 1, fmt.Sprintf("Action property %s is violated.", v.Name))
	} else {
		t.message(2116, 1, "Temporal properties were violated.\n")
	}
	if v.LoopStart == nil && result.Status != checker.PropertyViolation {
		t.message(2121, 1, "The behavior up to this point is:")
	} else {
		t.message(2264, 1, "The following behavior constitutes a counter-example:\n")
	}
}

func printToolTrace(t toolOutput, trace []checker.TraceStep, module *parser.Module) {
	for i, step := range trace {
		var body bytes.Buffer
		fmt.Fprintf(&body, "%d: <%s>\n", i+1, actionLabelWithRange(module, step.Action, step.ActionRange))
		printStateOrdered(&body, step.State, step.StateOrder)
		t.message(2217, 4, body.String())
	}
}

func printToolLoop(t toolOutput, v *checker.Violation, module *parser.Module) {
	if v.LoopStart == nil {
		return
	}
	if *v.LoopStart == len(v.Trace)-1 {
		t.message(2218, 4, fmt.Sprintf("%d: Stuttering", *v.LoopStart+2))
		return
	}
	t.message(2122, 4, fmt.Sprintf("%d: Back to state: <%s>\n", *v.LoopStart+1, actionLabelWithRange(module, v.LoopAction, v.LoopActionRange)))
}

func printToolStatistics(t toolOutput, result checker.Result) {
	counts := fmt.Sprintf("%d states generated, %d distinct states found, %d states left on queue.", result.GeneratedStates, result.DistinctStates, result.QueuedStates)
	t.message(2200, 0, fmt.Sprintf("Progress(%d) at %s: %s", result.Depth, toolTime(), counts))
	t.message(2199, 0, counts)
	if result.Failure != nil && result.Failure.SuppressDepth {
		return
	}
	t.message(2194, 0, fmt.Sprintf("The depth of the complete state graph search is %d.", result.Depth))
	if result.HasOutDegrees {
		if summary := outDegreeSummary(result.OutDegrees[:]); strings.TrimSpace(summary) != "" {
			t.message(2268, 0, summary)
		}
	}
}
