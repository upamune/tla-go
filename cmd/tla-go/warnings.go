package main

import (
	"fmt"
	"io"

	"github.com/upamune/tla-go/checker"
)

// configuredWarningOutput prints warnings as Java TLC's MP.printWarning does:
// identical warnings are printed once, and outside tool mode the first one is
// followed by a hint about -nowarning.
func configuredWarningOutput(opts options, out io.Writer) func(checker.Warning) {
	if opts.noWarning {
		return nil
	}
	printed := map[string]bool{}
	return func(warning checker.Warning) {
		if printed[warning.Message] {
			return
		}
		first := len(printed) == 0
		printed[warning.Message] = true
		if opts.tool {
			toolOutput{out: out}.message(warning.Code, 3, warning.Message)
			return
		}
		fmt.Fprintf(out, "Warning: %s\n", warning.Message)
		if first {
			fmt.Fprintln(out, "(Use the -nowarning option to disable this warning.)")
		}
	}
}
