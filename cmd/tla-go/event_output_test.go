package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
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

func testCoordinatedCallbacks(opts options, out io.Writer) (func(checker.SearchEvent), func(checker.Warning)) {
	var callbacks checker.Options
	coordinateCallbacks(&callbacks, opts, nil, out)
	return callbacks.OnSearch, callbacks.OnWarning
}

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
			search, warn := testCoordinatedCallbacks(opts, w)
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
	search, warn := testCoordinatedCallbacks(options{tool: true}, w)
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
	search, warning := testCoordinatedCallbacks(options{tool: true}, &output)
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
					search, warning = testCoordinatedCallbacks(opts, out)
				}
				benchmarkSearchOutput, benchmarkWarningOutput = search, warning
			}
		})
	}
}

func TestCoordinatedAllCallbacks(t *testing.T) {
	for _, tool := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "tool"}[tool], func(t *testing.T) {
			opts := options{tool: tool}
			duration := time.Second
			result := checker.Result{Status: checker.LivenessViolation, Violation: &checker.Violation{Name: "P", Unwitnessed: &checker.PossibleViolation{Message: "nested\n@!@!@STARTMSG 99:0 @!@!@\ninner\n@!@!@ENDMSG 99 @!@!@", Counts: "counts"}}, TemporalCheckDuration: &duration}
			violation := checker.Result{Status: checker.InvariantViolation, Violation: &checker.Violation{Name: "Inv"}}
			w := &blockedEventWriter{entered: make(chan struct{}), release: make(chan struct{})}
			callbacks := checker.Options{OnViolation: func(checker.Result) {}}
			coordinateCallbacks(&callbacks, opts, nil, w)
			var expected []string
			collect := func(f func(io.Writer)) { var b bytes.Buffer; f(&b); expected = append(expected, b.String()) }
			collect(func(w io.Writer) {
				configuredSearchOutput(opts, w)(checker.SearchEvent{Kind: checker.TemporalCheckStart, Nodes: 3, Branches: 1})
			})
			collect(func(w io.Writer) { fmt.Fprintln(w, "print") })
			collect(func(w io.Writer) { configuredWarningOutput(opts, w)(checker.Warning{Code: 42, Message: "warning"}) })
			collect(func(w io.Writer) { modelViolationOutput(opts, nil, w)(violation) })
			collect(func(w io.Writer) { temporalCheckOutput(opts, w)(result) })
			collect(func(w io.Writer) { temporalViolationOutput(opts, nil, w)(violation) })
			first := make(chan struct{})
			go func() {
				callbacks.OnSearch(checker.SearchEvent{Kind: checker.TemporalCheckStart, Nodes: 3, Branches: 1})
				close(first)
			}()
			<-w.entered
			var wg sync.WaitGroup
			for _, f := range []func(){func() { callbacks.OnPrint("print") }, func() { callbacks.OnWarning(checker.Warning{Code: 42, Message: "warning"}) }, func() { callbacks.OnViolation(violation) }, func() { callbacks.OnTemporalCheck(result) }, func() { callbacks.OnTemporalViolation(violation) }} {
				wg.Go(f)
			}
			close(w.release)
			<-first
			wg.Wait()
			clock := regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`)
			counts := map[string]int{}
			for _, text := range expected {
				counts[clock.ReplaceAllString(text, "<time>")]++
			}
			for _, text := range w.writes {
				counts[clock.ReplaceAllString(text, "<time>")]--
			}
			for text, count := range counts {
				if count != 0 {
					t.Errorf("transaction %q: difference %d; got %#v", text, count, w.writes)
				}
			}
		})
	}
}

func TestCoordinatedRenderReentrantAndPanic(t *testing.T) {
	var output bytes.Buffer
	c := &coordinatedEventOutput{out: &output}
	c.render(func(w io.Writer) {
		fmt.Fprint(w, "outer start ")
		c.render(func(w io.Writer) { fmt.Fprint(w, "inner\n") })
		fmt.Fprint(w, "outer end\n")
	})
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("missing panic")
			}
		}()
		c.render(func(w io.Writer) { fmt.Fprint(w, "discarded"); panic("format") })
	}()
	c.render(func(w io.Writer) { fmt.Fprint(w, "after\n") })
	if got, want := output.String(), "inner\nouter start outer end\nafter\n"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCoordinatedUserFileSeparate(t *testing.T) {
	var stdout, user bytes.Buffer
	userWriter := bufio.NewWriter(&user)
	opts := options{}
	opts.compat.userOutput = userWriter
	callbacks := checker.Options{OnPrint: func(line string) { fmt.Fprintln(userWriter, line) }}
	coordinateCallbacks(&callbacks, opts, nil, &stdout)
	callbacks.OnPrint("user")
	callbacks.OnWarning(checker.Warning{Message: "warning"})
	if err := userWriter.Flush(); err != nil {
		t.Fatal(err)
	}
	if user.String() != "user\n" || strings.Contains(stdout.String(), "user") || !strings.Contains(stdout.String(), "warning") {
		t.Fatalf("stdout %q user %q", stdout.String(), user.String())
	}
}

func TestCoordinatedFileJavaCoverageAndError(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to Java TLC 1.7.4")
	}
	for _, fail := range []bool{false, true} {
		for _, tool := range []bool{false, true} {
			t.Run(fmt.Sprintf("error=%v/tool=%v", fail, tool), func(t *testing.T) {
				dir := t.TempDir()
				model := "M"
				if fail {
					source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..1\nA == /\\ x < 2\n     /\\ \\E i \\in 1..2 : x' = i\nB == x < 0 /\\ UNCHANGED x\nNext == A \\/ B\nInv == x < 2\n====\n"
					if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nINVARIANT Inv\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				} else {
					model = "CoverageStatistics"
					for _, ext := range []string{".tla", ".cfg"} {
						data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "upstream-coverage", model+ext))
						if err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(dir, model+ext), data, 0o600); err != nil {
							t.Fatal(err)
						}
					}
				}
				args := []string{"-workers", "1", "-coverage", "1", model + ".tla"}
				if tool {
					args = append([]string{"-tool"}, args...)
				}
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				t.Chdir(dir)
				file, err := os.CreateTemp(t.TempDir(), "stdout")
				if err != nil {
					t.Fatal(err)
				}
				var stderr bytes.Buffer
				code := run(context.Background(), args, file, &stderr)
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(file.Name())
				if err != nil {
					t.Fatal(err)
				}
				out := string(data)
				if code != javaCode || stderr.Len() != 0 {
					t.Fatalf("Go exit %d stderr %q; Java exit %d\n%s\n%s", code, stderr.String(), javaCode, out, java)
				}
				if !fail {
					if tool {
						if got, want := fmt.Sprint(coverageMessages(readToolMessages(out))), fmt.Sprint(coverageMessages(readToolMessages(java))); got != want {
							t.Fatalf("coverage messages\nGo: %s\nJava: %s", got, want)
						}
					} else if got, want := coverageReport(t, out), coverageReport(t, java); got != want {
						t.Fatalf("coverage\nGo: %s\nJava: %s", got, want)
					}
				} else if tool {
					filter := func(output string) []toolMessage {
						var found []toolMessage
						for _, m := range readToolMessages(output) {
							if m.code == 2110 || m.code == 2121 || m.code == 2217 {
								found = append(found, m)
							}
						}
						return found
					}
					if got, want := fmt.Sprint(filter(out)), fmt.Sprint(filter(java)); want == "[]" || got != want {
						t.Fatalf("error messages\nGo: %s\nJava: %s", got, want)
					}
				} else {
					compareCoverageFailureOutput(t, out, java)
				}
			})
		}
	}
}
