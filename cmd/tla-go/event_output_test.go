package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/upamune/tla-go/checker"
)

type blockedEventWriter struct {
	entered chan struct{}
	release chan struct{}
	mu      sync.Mutex
	writes  []string
}

func (w *blockedEventWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.writes) == 0 {
		close(w.entered)
		<-w.release
	}
	w.writes = append(w.writes, string(p))
	return len(p), nil
}

func TestCoordinatedEventCallbacks(t *testing.T) {
	for _, tool := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "tool"}[tool], func(t *testing.T) {
			opts := options{tool: tool}
			event := checker.SearchEvent{Kind: checker.TemporalCheckStart, Nodes: 3, Branches: 1}
			warning := checker.Warning{Code: 1234, Message: "warning\n@!@!@STARTMSG 99:0 @!@!@\nnested\n@!@!@ENDMSG 99 @!@!@"}
			var expected bytes.Buffer
			configuredSearchOutput(opts, &expected)(event)
			configuredWarningOutput(opts, &expected)(warning)
			w := &blockedEventWriter{entered: make(chan struct{}), release: make(chan struct{})}
			search, warn := newCoordinatedCallbacks(opts, w)
			done := make(chan struct{})
			go func() { search(event); close(done) }()
			<-w.entered
			doneWarning := make(chan struct{})
			go func() { warn(warning); close(doneWarning) }()
			select {
			case <-doneWarning:
				t.Fatal("warning completed during blocked search write")
			case <-time.After(20 * time.Millisecond):
			}
			close(w.release)
			<-done
			<-doneWarning
			clock := regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`)
			if len(w.writes) != 2 || clock.ReplaceAllString(strings.Join(w.writes, ""), "<time>") != clock.ReplaceAllString(expected.String(), "<time>") {
				t.Fatalf("writes %#v\nwant %q", w.writes, expected.String())
			}
			warn(warning)
			if len(w.writes) != 2 {
				t.Fatal("duplicate warning written")
			}
		})
	}
}

type errorEventWriter struct {
	panicNext bool
	calls     int
}

func (w *errorEventWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.panicNext {
		w.panicNext = false
		panic("sink")
	}
	return 0, errors.New("sink")
}

func TestCoordinatedEventFailureUnlocks(t *testing.T) {
	w := &errorEventWriter{panicNext: true}
	search, warn := newCoordinatedCallbacks(options{tool: true}, w)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("missing panic")
			}
		}()
		warn(checker.Warning{Message: "first"})
	}()
	search(checker.SearchEvent{Kind: checker.InitialStart})
	warn(checker.Warning{Message: "second"})
	if w.calls != 3 {
		t.Fatalf("calls %d", w.calls)
	}
}

func TestCoordinatedEventsConcurrent(t *testing.T) {
	var output bytes.Buffer
	search, warning := newCoordinatedCallbacks(options{tool: true}, &output)
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			search(checker.SearchEvent{Kind: checker.InitialStart})
			warning(checker.Warning{Code: 42, Message: "same"})
		})
	}
	wg.Wait()
	var expected bytes.Buffer
	configuredSearchOutput(options{tool: true}, &expected)(checker.SearchEvent{Kind: checker.InitialStart})
	if got := strings.Count(output.String(), expected.String()); got != 100 {
		t.Fatalf("search callbacks: %d", got)
	}
	if got := strings.Count(output.String(), "@!@!@STARTMSG 42:3 @!@!@"); got != 1 {
		t.Fatalf("warnings: %d", got)
	}
}

func TestEventOutputFileAndCustomWriter(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".tla", ".cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "upstream-coverage", "CoverageStatistics"+ext))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "CoverageStatistics"+ext), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tool := range []bool{false, true} {
		args := []string{"-workers", "1", "-coverage", "1", filepath.Join(dir, "CoverageStatistics.tla")}
		if tool {
			args = append([]string{"-tool"}, args...)
		}
		file, err := os.CreateTemp(t.TempDir(), "output")
		if err != nil {
			t.Fatal(err)
		}
		var stderr, fallback, fallbackErr bytes.Buffer
		code := run(context.Background(), args, file, &stderr)
		fallbackCode := run(context.Background(), args, &fallback, &fallbackErr)
		got, err := os.ReadFile(file.Name())
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		// Whole-run timestamps and elapsed times are not stable between executions.
		// Compare the initial-start event, which has no clock-dependent fields.
		if code != fallbackCode || stderr.String() != fallbackErr.String() {
			t.Fatalf("exit/stderr: %d %q vs %d %q", code, stderr.String(), fallbackCode, fallbackErr.String())
		}
		if tool {
			const event = "@!@!@STARTMSG 2189:0 @!@!@\nComputing initial states...\n@!@!@ENDMSG 2189 @!@!@\n"
			if !strings.Contains(string(got), event) || !strings.Contains(fallback.String(), event) {
				t.Fatalf("missing search event: %s", got)
			}
		}
	}
}

var (
	benchmarkSearchOutput  func(checker.SearchEvent)
	benchmarkWarningOutput func(checker.Warning)
)

func BenchmarkDisabledEventOutputSetup(b *testing.B) {
	opts := options{}
	var out io.Writer = io.Discard
	for _, tc := range []struct {
		name  string
		check bool
	}{{"previous", false}, {"current", true}} {
		b.Run(tc.name, func(b *testing.B) {
			for b.Loop() {
				search := configuredSearchOutput(opts, out)
				warning := configuredWarningOutput(opts, out)
				if tc.check && coordinateEventOutput(opts.coverageInterval, out) {
					search, warning = newCoordinatedCallbacks(opts, out)
				}
				benchmarkSearchOutput, benchmarkWarningOutput = search, warning
			}
		})
	}
}
