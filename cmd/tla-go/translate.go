package main

import (
	"fmt"
	"io"
	"os"

	"github.com/upamune/tla-go/pcal"
)

// translateFlag selects the PlusCal translator, which takes the options of
// Java's pcal.trans.
const translateFlag = "-translate"

// runTranslator handles "tla-go -translate [options] File.tla", which
// behaves like "java pcal.trans [options] File.tla".
func runTranslator(args []string, out, errout io.Writer) (int, bool) {
	if len(args) == 0 || args[0] != translateFlag {
		return 0, false
	}
	if os.Getenv("TLA_GO_DEBUG_RECORDER") != "" {
		fmt.Fprintln(errout, recorderModeError)
		return 255, true
	}
	return pcal.Run(args[1:], out, errout), true
}
