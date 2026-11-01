package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/syntax/parser"
)

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
		code := 2772
		if i == 0 {
			code = 2773
		}
		if a.Property {
			t.message(2774, 0, fmt.Sprintf("<%s %s>", a.Name, coverageLocation(a.Range)))
		} else {
			t.message(code, 0, fmt.Sprintf("<%s %s>: %d:%d", a.Name, coverageLocation(a.Range), a.Distinct, a.Generated))
		}
		for _, e := range a.Expressions {
			printToolCoverageExpression(t, e)
		}
	}
	t.message(2202, 0, "End of statistics.")
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
	t.message(code, 0, fmt.Sprintf("  %s%s", strings.Repeat("|", e.Depth), coverageExpression(e)))
}
