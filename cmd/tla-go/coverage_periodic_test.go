package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/internal/javatmp"
)

type manualPeriodicTimer struct{ c chan time.Time }

func (t *manualPeriodicTimer) C() <-chan time.Time { return t.c }
func (t *manualPeriodicTimer) Stop() bool          { return true }

type timerRequest struct {
	delay time.Duration
	timer *manualPeriodicTimer
}

func testTimers() (chan timerRequest, func(time.Duration) periodicTimer) {
	requests := make(chan timerRequest, 10)
	return requests, func(d time.Duration) periodicTimer {
		t := &manualPeriodicTimer{make(chan time.Time, 1)}
		requests <- timerRequest{d, t}
		return t
	}
}

func nextTimer(t *testing.T, requests <-chan timerRequest, d time.Duration) *manualPeriodicTimer {
	t.Helper()
	select {
	case r := <-requests:
		if r.delay != d {
			t.Fatalf("timer %v, want %v", r.delay, d)
		}
		return r.timer
	case <-time.After(5 * time.Second):
		t.Fatal("timer not scheduled")
		return nil
	}
}

type synchronizedOutput struct {
	mu     sync.Mutex
	b      bytes.Buffer
	writes chan string
}

func (w *synchronizedOutput) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.b.Write(b)
	w.writes <- string(b)
	return n, err
}

func TestPeriodicCoverageBlockedCheck(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nA == /\\ x < 2\n     /\\ x' = x + 1\nB == x' = x\nNext == A \\/ B\n===="
	const cfg = "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"
	for _, tool := range []bool{false, true} {
		for _, interval := range []time.Duration{0, 12704 * time.Millisecond, time.Minute, 72704 * time.Millisecond} {
			t.Run(fmtTestName(tool, interval), func(t *testing.T) {
				requests, timer := testTimers()
				output := &synchronizedOutput{writes: make(chan string, 10)}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				var progress checker.LiveProgress
				p := newPeriodicCoverage(ctx, options{tool: tool, coverageInterval: interval}, &progress, &coordinatedEventOutput{out: output}, timer)
				initial, releaseInitial, blocked, release := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
				done := make(chan struct{})
				go func() {
					defer close(done)
					defer p.stop()
					_, _ = checker.CheckWithOptions(ctx, source, cfg, checker.Options{Coverage: true, LiveProgress: &progress, OnSearch: func(e checker.SearchEvent) {
						if e.Kind == checker.InitialStart {
							close(initial)
							<-releaseInitial
						}
						if e.Kind == checker.InitialComplete {
							p.start()
						}
					}, OnGraph: func(e checker.GraphEvent) error {
						if !e.Initial {
							select {
							case <-blocked:
							default:
								close(blocked)
								<-release
							}
						}
						return nil
					}})
				}()
				<-initial
				select {
				case <-requests:
					t.Fatal("timer started during initial evaluation")
				default:
				}
				close(releaseInitial)
				first := nextTimer(t, requests, 3*time.Second)
				<-blocked
				first.c <- time.Now()
				var report string
				select {
				case report = <-output.writes:
				case <-time.After(5 * time.Second):
					t.Fatal("report blocked behind evaluation")
				}
				snapshot := progress.Snapshot()
				for _, field := range []string{
					fmt.Sprintf("Progress(%d)", snapshot.Depth),
					grouped(snapshot.GeneratedStates) + " states generated",
					grouped(snapshot.DistinctStates) + " distinct states found",
					grouped(snapshot.QueuedStates) + " states left on queue",
				} {
					if !strings.Contains(report, field) {
						t.Fatalf("missing %q: %s", field, report)
					}
				}
				wantCoverage := interval < time.Minute
				if strings.Contains(report, "End of statistics.") != wantCoverage {
					t.Fatalf("unexpected coverage: %s", report)
				}
				if tool && (!strings.Contains(report, "STARTMSG 2200:0") || wantCoverage && !strings.Contains(report, "ENDMSG 2202")) {
					t.Fatalf("tool output: %s", report)
				}
				second := nextTimer(t, requests, time.Minute)
				second.c <- time.Now()
				select {
				case report = <-output.writes:
				case <-time.After(5 * time.Second):
					t.Fatal("second tick missing")
				}
				if !strings.Contains(report, "End of statistics.") {
					t.Fatalf("second tick missing coverage: %s", report)
				}
				cancel()
				close(release)
				<-done
				select {
				case <-p.joined:
				default:
					t.Fatal("observer not joined")
				}
			})
		}
	}
}

func fmtTestName(tool bool, interval time.Duration) string {
	if tool {
		return "tool-" + interval.String()
	}
	return "plain-" + interval.String()
}

func TestPeriodicCoverageEligibility(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	base := options{coverage: true, coverageInterval: time.Minute}
	if !periodicCoverageEligible(base, file, file) {
		t.Fatal("normal file output ineligible")
	}
	zero := base
	zero.coverageInterval = 0
	if !periodicCoverageEligible(zero, file, file) {
		t.Fatal("explicit zero ineligible")
	}
	zero.coverage = false
	if periodicCoverageEligible(zero, file, file) {
		t.Fatal("absent coverage eligible")
	}
	for name, mutate := range map[string]func(*options){
		"disabled": func(o *options) { o.coverage = false }, "checkpoint": func(o *options) { o.checkpoint = true }, "recover": func(o *options) { o.recoverDir = "x" }, "graph": func(o *options) { o.dumpPath = "x" }, "debug": func(o *options) { o.compat.debug = true }, "profile": func(o *options) { o.cpuPath = "x" }, "userFile": func(o *options) { o.compat.userFile = "x" }, "simulate": func(o *options) { o.simulate = true }, "dfid": func(o *options) { o.dfid = true }, "continue": func(o *options) { o.continuation = true }, "specTE": func(o *options) { o.compat.specTE = &specTE{} },
	} {
		t.Run(name, func(t *testing.T) {
			for _, interval := range []time.Duration{0, time.Minute} {
				o := base
				o.coverageInterval = interval
				mutate(&o)
				if periodicCoverageEligible(o, file, file) {
					t.Fatal("eligible")
				}
			}
		})
	}
	if periodicCoverageEligible(base, io.Discard, file) || periodicCoverageEligible(base, file, io.Discard) {
		t.Fatal("non-file sink eligible")
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	device, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer device.Close()
	closed, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()
	for name, sink := range map[string]*os.File{"pipe": writer, "device": device, "stat-error": closed, "nil": nil} {
		t.Run(name, func(t *testing.T) {
			for _, interval := range []time.Duration{0, time.Minute} {
				o := base
				o.coverageInterval = interval
				if periodicCoverageEligible(o, sink, file) || periodicCoverageEligible(o, file, sink) {
					t.Fatal("unsafe sink eligible")
				}
			}
		})
	}
}

func TestPeriodicCoverageZeroOutputFallback(t *testing.T) {
	dir := t.TempDir()
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nA == /\\ x < 1000000000\n     /\\ x' = x + 1\nB == x' = x\nNext == A \\/ B\n====\n"
	if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, customStdout := range []bool{false, true} {
		t.Run(fmt.Sprint(customStdout), func(t *testing.T) {
			t.Parallel()
			file, err := os.CreateTemp(t.TempDir(), "output")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			var custom bytes.Buffer
			var out, errout io.Writer = file, &custom
			if customStdout {
				out, errout = &custom, file
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan int, 1)
			go func() {
				done <- runWithInput(ctx, []string{"-coverage", "0", filepath.Join(dir, "M.tla")}, strings.NewReader(""), out, errout)
			}()
			select {
			case code := <-done:
				t.Fatalf("zero interval stopped before cancellation: %d", code)
			case <-time.After(3500 * time.Millisecond):
			}
			cancel()
			select {
			case code := <-done:
				if code == 0 {
					t.Fatal("canceled check succeeded")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("canceled check did not stop")
			}
			data, err := os.ReadFile(file.Name())
			if err != nil {
				t.Fatal(err)
			}
			output := string(data) + custom.String()
			if strings.Contains(output, "Progress(") || strings.Contains(output, "The coverage statistics at ") || strings.Contains(output, "-coverage interval elapsed") {
				t.Fatalf("unexpected fallback output: %s", output)
			}
		})
	}
}

// The reader remains open without consuming bytes. Before the eligibility fix,
// the oversized report filled this pipe and SIGINT could not join the reporter.
func TestPeriodicCoverageUnreadPipeInterrupt(t *testing.T) {
	dir := t.TempDir()
	var source strings.Builder
	source.WriteString("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\n")
	for i := range 1000 {
		fmt.Fprintf(&source, "A%d == x' = x + 1\n", i)
	}
	source.WriteString("Next == ")
	for i := range 1000 {
		if i > 0 {
			source.WriteString(" \\/ ")
		}
		fmt.Fprintf(&source, "A%d", i)
	}
	source.WriteString("\n====\n")
	if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "tla-go")
	build := exec.Command("go", "build", "-o", binary, ".")
	if data, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, data)
	}
	type implementation struct {
		name     string
		args     []string
		java     bool
		deadline bool
	}
	implementations := []implementation{
		{"go-deadline", []string{binary, "-workers", "1", "-coverage", "71583", "M.tla"}, false, true},
		{"go-interrupt", []string{binary, "-workers", "1", "-coverage", "1", "M.tla"}, false, false},
		{"go-zero-interrupt", []string{binary, "-workers", "1", "-coverage", "0", "M.tla"}, false, false},
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		implementations = append(implementations, implementation{"java", []string{"java", "-Djava.io.tmpdir=" + t.TempDir(), "-XX:+UseParallelGC", "-Xmx512m", "-cp", jar, "tlc2.TLC", "-workers", "1", "-coverage", "71583", "M.tla"}, true, false})
	}
	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			errout, err := os.CreateTemp(dir, "stderr")
			if err != nil {
				t.Fatal(err)
			}
			defer errout.Close()
			cmd := exec.Command(impl.args[0], impl.args[1:]...)
			cmd.Dir, cmd.Stdout, cmd.Stderr = dir, writer, errout
			if impl.java {
				javatmp.Apply(cmd)
			}
			if err := cmd.Start(); err != nil {
				writer.Close()
				t.Fatal(err)
			}
			writer.Close()
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			defer func() { _ = cmd.Process.Kill(); <-done }()
			if impl.deadline {
				select {
				case err := <-done:
					done <- err
					if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() == 0 {
						t.Fatalf("deadline exit: %v", err)
					}
				case <-time.After(8 * time.Second):
					t.Fatal("fallback deadline did not stop")
				}
				data, err := io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(data), "Progress(") || strings.Contains(string(data), "The coverage statistics at ") {
					t.Fatalf("unexpected periodic output: %s", data)
				}
				stderr, err := os.ReadFile(errout.Name())
				if err != nil || !strings.Contains(string(stderr), "-coverage interval elapsed") {
					t.Fatalf("deadline stderr: %s (%v)", stderr, err)
				}
				return
			}
			wait := 4 * time.Second
			if impl.java {
				wait = 7 * time.Second
			}
			select {
			case err := <-done:
				done <- err
				t.Fatalf("exited before interrupt: %v", err)
			case <-time.After(wait):
			}
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				done <- err
				if impl.java {
					if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 130 {
						t.Fatalf("Java exit: %v", err)
					}
				} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() == 0 {
					t.Fatalf("Go exit: %v", err)
				}
			case <-time.After(4 * time.Second):
				t.Fatal("did not exit after SIGINT with unread pipe")
			}
			data, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if !impl.java && strings.Contains(string(data), "The coverage statistics at ") {
				t.Fatal("pipe received periodic coverage")
			}
			if impl.java && (len(data) < 4096 || !strings.Contains(string(data), "The coverage statistics at ")) {
				t.Fatalf("Java did not start oversized report: %d bytes", len(data))
			}
		})
	}
}

func TestPeriodicCoveragePipeCancel(t *testing.T) {
	dir := t.TempDir()
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nA == /\\ x < 1000000000\n     /\\ x' = x + 1\nB == x' = x\nNext == A \\/ B\n====\n"
	if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	errout, err := os.CreateTemp(dir, "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer errout.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan int, 1)
	go func() {
		done <- runWithInput(ctx, []string{"-workers", "1", "-coverage", "71583", filepath.Join(dir, "M.tla")}, strings.NewReader(""), writer, errout)
	}()
	select {
	case code := <-done:
		t.Fatalf("check exited before cancel: %d", code)
	case <-time.After(time.Second):
	}
	cancel()
	select {
	case code := <-done:
		if code == 0 {
			t.Fatal("canceled check succeeded")
		}
	case <-time.After(5 * time.Second):
		reader.Close()
		t.Fatal("canceled check did not stop")
	}
	writer.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Progress(") || strings.Contains(string(data), "The coverage statistics at ") {
		t.Fatalf("unexpected periodic output: %s", data)
	}
}

func TestPeriodicCoveragePipeFinalBytes(t *testing.T) {
	dir := t.TempDir()
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nA == /\\ x < 2\n     /\\ x' = x + 1\nB == x' = x\nNext == A \\/ B\n====\n"
	if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var reports []string
	for _, interval := range []string{"0", "1"} {
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		code := runWithInput(context.Background(), []string{"-workers", "1", "-coverage", interval, filepath.Join(dir, "M.tla")}, strings.NewReader(""), writer, &stderr)
		writer.Close()
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil || code != 0 || stderr.Len() != 0 {
			t.Fatalf("interval %s: code %d, stderr %q, read %v", interval, code, stderr.String(), err)
		}
		reports = append(reports, coverageReport(t, string(data)))
	}
	if reports[0] != reports[1] {
		t.Fatalf("coverage bytes differ:\n%s\n%s", reports[0], reports[1])
	}
}

func TestPeriodicCoverageZeroShortRegularFile(t *testing.T) {
	dir := t.TempDir()
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nA == /\\ x < 2\n     /\\ x' = x + 1\nB == x' = x\nNext == A \\/ B\n====\n"
	if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(dir, "out")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errout, err := os.CreateTemp(dir, "err")
	if err != nil {
		t.Fatal(err)
	}
	defer errout.Close()
	args := []string{"-workers", "1", "-coverage", "0", filepath.Join(dir, "M.tla")}
	if code := runWithInput(context.Background(), args, strings.NewReader(""), out, errout); code != 0 {
		t.Fatalf("regular file exit: %d", code)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	var fallback, stderr bytes.Buffer
	if code := runWithInput(context.Background(), args, strings.NewReader(""), &fallback, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("fallback exit: %d, stderr: %s", code, stderr.String())
	}
	if strings.Contains(string(data), "Progress(") || coverageReport(t, string(data)) != coverageReport(t, fallback.String()) {
		t.Fatalf("short regular-file output differs from fallback: %s", data)
	}
}

func BenchmarkPeriodicCoverageEligibility(b *testing.B) {
	file, err := os.CreateTemp(b.TempDir(), "output")
	if err != nil {
		b.Fatal(err)
	}
	defer file.Close()
	for name, opts := range map[string]options{
		"absent":   {},
		"zero":     {coverage: true},
		"positive": {coverage: true, coverageInterval: time.Minute},
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = periodicCoverageEligible(opts, file, file)
			}
		})
	}
}

func TestPeriodicCoverageStopJoinsOutput(t *testing.T) {
	for _, mode := range []string{"return", "error", "cancel", "panic"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			requests, timer := testTimers()
			writer := &blockedEventWriter{entered: make(chan struct{}), release: make(chan struct{})}
			var progress checker.LiveProgress
			p := newPeriodicCoverage(ctx, options{coverageInterval: time.Minute}, &progress, &coordinatedEventOutput{out: writer}, timer)
			done := make(chan struct{})
			exit := make(chan struct{})
			go func() {
				defer close(done)
				defer func() { _ = recover() }()
				defer p.stop()
				p.start()
				<-exit
				if mode == "panic" {
					panic("test")
				}
			}()
			first := nextTimer(t, requests, 3*time.Second)
			first.c <- time.Now()
			<-writer.entered
			if mode == "cancel" {
				cancel()
			}
			close(exit)
			select {
			case <-done:
				t.Fatal("returned before output joined")
			case <-time.After(10 * time.Millisecond):
			}
			close(writer.release)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("did not join observer")
			}
			writer.mu.Lock()
			count := len(writer.writes)
			writer.mu.Unlock()
			if count != 1 {
				t.Fatalf("writes %d", count)
			}
			select {
			case <-p.joined:
			default:
				t.Fatal("observer not joined")
			}
		})
	}
}

func TestPeriodicCoverageRealFileCLI(t *testing.T) {
	dir := t.TempDir()
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nA == /\\ x < 1000000000\n     /\\ x' = x + 1\nB == x' = x\nNext == A \\/ B\n====\n"
	if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "tla-go")
	if data, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, data)
	}
	out, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errout, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	defer errout.Close()
	cmd := exec.Command(binary, "-tool", "-coverage", "0", filepath.Join(dir, "M.tla"))
	cmd.Stdout, cmd.Stderr = out, errout
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() { _ = cmd.Process.Kill() }()
	timeout := time.After(15 * time.Second)
	var output string
	for {
		data, readErr := os.ReadFile(out.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		output = string(data)
		if strings.Contains(output, "ENDMSG 2202") {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("CLI returned %v before periodic report: %s", err, output)
		case <-timeout:
			t.Fatalf("report timed out: %s", output)
		case <-time.After(20 * time.Millisecond):
		}
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("interrupted CLI succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("CLI did not stop")
	}
	codes := []string{"STARTMSG 2200:0", "STARTMSG 2201:0", "STARTMSG 2773:0", "STARTMSG 2772:0", "ENDMSG 2202"}
	offset := 0
	for _, code := range codes {
		i := strings.Index(output[offset:], code)
		if i < 0 {
			t.Fatalf("missing or misordered %s: %s", code, output)
		}
		offset += i + len(code)
	}
}
