package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/syntax/parser"
)

// Positive intervals are currently limited to runs completed before the first
// interval. Cancel rather than emit an incomplete periodic coverage stream.
func checkWithCoverageDeadline(ctx context.Context, interval time.Duration, withTimeout func(context.Context, time.Duration) (context.Context, context.CancelFunc), check func(context.Context) (checker.Result, error)) (checker.Result, error) {
	return checkWithCoverageDeadlineClock(ctx, interval, withTimeout, time.Now, check)
}

func checkWithCoverageDeadlineClock(ctx context.Context, interval time.Duration, withTimeout func(context.Context, time.Duration) (context.Context, context.CancelFunc), now func() time.Time, check func(context.Context) (checker.Result, error)) (checker.Result, error) {
	if interval == 0 {
		return check(ctx)
	}
	// Java 1.7.4 starts with count = interval / progressInterval, checks it
	// after the initial three-second wait, and decrements it after each check.
	// Use the earlier of that first report and the effective interval.
	firstReport := 3*time.Second + (interval/time.Minute)*time.Minute
	if firstReport < interval {
		interval = firstReport
	}
	start := now()
	timed, cancel := withTimeout(ctx, interval)
	defer cancel()
	result, err := check(timed)
	if ctx.Err() == nil && (timed.Err() == context.DeadlineExceeded || now().Sub(start) >= interval) {
		return checker.Result{}, fmt.Errorf("-coverage interval elapsed; periodic coverage reports are not yet supported")
	}
	return result, err
}

func coverageLocation(r parser.Range) string {
	return fmt.Sprintf("line %d, col %d to line %d, col %d of module %s", r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
}

func printCoverage(w io.Writer, actions []checker.CoverageAction) {
	if len(actions) == 0 {
		return
	}
	fmt.Fprintf(w, "The coverage statistics at %s\n", time.Now().Format("2006-01-02 15:04:05"))
	for _, a := range actions {
		if a.Property {
			fmt.Fprintf(w, "<%s %s>\n", a.Name, coverageLocation(a.Range))
		} else {
			fmt.Fprintf(w, "<%s %s>: %d:%d\n", a.Name, coverageLocation(a.Range), a.Distinct, a.Generated)
		}
		for _, e := range a.Expressions {
			fmt.Fprintf(w, "  %s%s\n", strings.Repeat("|", e.Depth), coverageExpression(e))
		}
	}
	fmt.Fprintln(w, "End of statistics.")
}

func printToolCoverage(t toolOutput, actions []checker.CoverageAction) {
	if len(actions) == 0 {
		return
	}
	t.message(2201, 0, "The coverage statistics at "+toolTime())
	for i, a := range actions {
		printToolCoverageAction(t, a, i == 0)
	}
	t.message(2202, 0, "End of statistics.")
}

func printToolCoverageAction(t toolOutput, a checker.CoverageAction, first bool) {
	code := 2772
	if first {
		code = 2773
	}
	location := fmt.Sprintf("<%s %s>", a.Name, coverageLocation(a.Range))
	if a.Property {
		recordStrings(t.recorder, 2774, location)
		t.message(2774, 0, location)
	} else {
		if t.recorder != nil {
			recordStrings(t.recorder, code, location, fmt.Sprint(a.Distinct), fmt.Sprint(a.Generated))
		}
		t.message(code, 0, fmt.Sprintf("%s: %d:%d", location, a.Distinct, a.Generated))
	}
	for _, e := range a.Expressions {
		printToolCoverageExpression(t, e)
	}
}

func coverageExpression(e checker.CoverageExpression) string {
	result := fmt.Sprintf("%s: %d", coverageLocation(e.Range), e.Count)
	if e.Cost > 0 {
		result += fmt.Sprintf(":%d", e.Cost)
	}
	return result
}

func printToolCoverageExpression(t toolOutput, e checker.CoverageExpression) {
	code := 2221
	if e.Cost > 0 {
		code = 2775
	}
	if t.recorder != nil {
		recordCoverageExpression(t.recorder, code, e)
	}
	t.message(code, 0, fmt.Sprintf("  %s%s", strings.Repeat("|", e.Depth), coverageExpression(e)))
}

func recordCoverageExpression(recorder *recorderConfig, code int, e checker.CoverageExpression) {
	location := strings.Repeat("|", e.Depth) + coverageLocation(e.Range)
	if e.Cost > 0 {
		recordStrings(recorder, code, location, fmt.Sprint(e.Count), fmt.Sprint(e.Cost))
	} else {
		recordStrings(recorder, code, location, fmt.Sprint(e.Count))
	}
}
