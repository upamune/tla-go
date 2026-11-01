package main

import (
	"bytes"
	"io"
	"os"
	"sync"
	"time"

	"github.com/upamune/tla-go/checker"
)

// coordinatedEventOutput only serializes complete search and warning callbacks
// within one check. Print, violations, temporal callbacks, debug, final/error
// output and timers remain barriers: they must be coordinated before periodic
// reporting can safely be added. Custom writers (which may recursively invoke
// callbacks) use the original path, avoiding a lock around their Write method.
type coordinatedEventOutput struct {
	mu  sync.Mutex
	out io.Writer
}

func (c *coordinatedEventOutput) commit(data []byte) {
	if len(data) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// Existing CLI output ignores write errors and does not retry short writes.
	_, _ = c.out.Write(data)
}

func coordinateEventOutput(interval time.Duration, out io.Writer) bool {
	if interval <= 0 {
		return false
	}
	file, ok := out.(*os.File)
	return ok && file != nil
}

// out must be an *os.File in production; tests inject controlled writers here.
func newCoordinatedCallbacks(opts options, out io.Writer) (func(checker.SearchEvent), func(checker.Warning)) {
	c := &coordinatedEventOutput{out: out}
	search := func(event checker.SearchEvent) {
		var buf bytes.Buffer
		configuredSearchOutput(opts, &buf)(event)
		c.commit(buf.Bytes())
	}
	if opts.noWarning {
		return search, nil
	}
	// The warning formatter remembers which warnings have been printed. Keep
	// its state and first-warning hint in order through the corresponding write.
	var warningMu sync.Mutex
	var buf bytes.Buffer
	warning := configuredWarningOutput(opts, &buf)
	return search, func(event checker.Warning) {
		warningMu.Lock()
		defer warningMu.Unlock()
		buf.Reset()
		warning(event)
		c.commit(buf.Bytes())
	}
}
