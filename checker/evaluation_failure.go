package checker

import (
	"fmt"
	"slices"
	"strings"

	"github.com/upamune/tla-go/evaluator"
)

func evaluationDetail(failure *evaluator.RuntimeError) string {
	if failure.Fingerprint != nil {
		return ""
	}
	sources := failure.Sources
	if failure.Unexpected != nil {
		sources = failure.Unexpected.Sources
	}
	var lines strings.Builder
	index := 0
	for _, v := range slices.Backward(sources) {
		r := v.Location.Range
		if r.Start.Line == 0 {
			continue
		}
		fmt.Fprintf(&lines, "%d. Line %d, column %d to line %d, column %d in %s\n", index, r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
		index++
	}
	if index == 0 {
		return ""
	}
	// Like Java TLC's CallStack.toString, end the frames with a blank line.
	return "The error occurred when TLC was evaluating the nested\nexpressions at the following positions:\n" + lines.String() + "\n"
}
