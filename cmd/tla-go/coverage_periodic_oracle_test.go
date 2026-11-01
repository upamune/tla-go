package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Stop TLC as soon as the relevant complete report has arrived. The deadline
// remains a backstop for startup failures and stalled progress threads.
type periodicOutput struct {
	buf    bytes.Buffer
	done   func()
	marker string
	count  int
}

func (o *periodicOutput) Write(p []byte) (int, error) {
	n, err := o.buf.Write(p)
	if strings.Count(o.buf.String(), o.marker) >= o.count {
		o.done()
	}
	return n, err
}

// Java's workers continue updating the cost tree while the progress thread
// reads it. A wrapped 12.704-second interval makes the first report observable
// at the initial three-second progress tick.
func TestJavaTLCCoveragePeriodicOracle(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to Java TLC 1.7.4")
	}
	for _, tc := range []struct {
		name, interval string
		tool, coverage bool
	}{
		{"zero-tool", "0", true, true},
		{"zero-plain", "0", false, true},
		{"minute-tool", "1", true, false},
		{"wrapped-plain", "71583", false, true},
		{"wrapped-tool", "71583", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			// Each successor increases x by one. The bounded choice slows state
			// generation to keep the fingerprint set small while waiting for the
			// progress thread, without introducing a short cycle.
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1 + ((CHOOSE n \\in 1..10000 : n = (x % 10000) + 1) % 1)\n====\n"
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, timeout := context.WithTimeout(context.Background(), 150*time.Second)
			defer timeout()
			ctx, stop := context.WithCancel(ctx)
			defer stop()
			args := []string{"-XX:+UseParallelGC", "-Xmx512m", "-cp", jar, "tlc2.TLC", "-workers", "1", "-coverage", tc.interval}
			if tc.tool {
				args = append(args, "-tool")
			}
			args = append(args, "M.tla")
			cmd := exec.CommandContext(ctx, "java", args...)
			cmd.Dir = dir
			marker, count := "End of statistics.", 1
			if tc.tool && tc.coverage {
				marker = "ENDMSG 2202"
			} else if !tc.coverage {
				// The second progress message proves the first reporting cycle
				// has finished without producing a coverage report.
				marker, count = "ENDMSG 2200", 2
			}
			out := &periodicOutput{done: stop, marker: marker, count: count}
			cmd.Stdout, cmd.Stderr = out, out
			err := cmd.Run()
			if ctx.Err() != context.Canceled {
				t.Fatalf("Java stopped before the expected report (context: %v, error: %v):\n%s", ctx.Err(), err, out.buf.String())
			}
			text := out.buf.String()
			// A final report and final progress are not periodic output. Reject a
			// completed model even if its output happened to satisfy the marker.
			if strings.Contains(text, "Model checking completed.") {
				t.Fatalf("Java finished the model before the expected periodic output:\n%s", text)
			}
			cycle := text
			if !tc.coverage {
				// Java may emit coverage immediately after the second progress
				// message, before cancellation takes effect. Only inspect the
				// completed first cycle.
				const progressMarker = "STARTMSG 2200:0"
				first := strings.Index(text, progressMarker)
				if first < 0 {
					t.Fatalf("missing first progress message:\n%s", text)
				}
				second := strings.Index(text[first+len(progressMarker):], progressMarker)
				if second < 0 {
					t.Fatalf("missing second progress message:\n%s", text)
				}
				cycle = text[:first+len(progressMarker)+second]
			}
			progress, start, end := strings.Index(cycle, "Progress("), strings.Index(cycle, "The coverage statistics at "), strings.Index(cycle, "End of statistics.")
			if progress < 0 || tc.coverage && (start <= progress || end <= start) || !tc.coverage && start >= 0 {
				t.Fatalf("missing or misordered periodic output:\n%s", text)
			}
			if tc.tool {
				codes := []string{"STARTMSG 2200:0"}
				if tc.coverage {
					codes = append(codes, "STARTMSG 2201:0", "STARTMSG 2773:0", "STARTMSG 2772:0", "STARTMSG 2202:0")
				}
				after := 0
				for _, code := range codes {
					index := strings.Index(text[after:], code)
					if index < 0 {
						t.Fatalf("missing or misordered %s:\n%s", code, text)
					}
					after += index + len(code)
				}
			}
		})
	}
}
