package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/upamune/tla-go/checker"
)

// A timer is owned and stopped by the reporting goroutine.
type periodicTimer interface {
	C() <-chan time.Time
	Stop() bool
}
type systemPeriodicTimer struct{ *time.Timer }

func (t systemPeriodicTimer) C() <-chan time.Time     { return t.Timer.C }
func realPeriodicTimer(d time.Duration) periodicTimer { return systemPeriodicTimer{time.NewTimer(d)} }

func periodicCoverageEligible(o options, out, errout io.Writer) bool {
	return o.coverage && coordinateEventOutput(o.coverageInterval, out) && regularOutputFile(out) && regularOutputFile(errout) &&
		periodicModeEligible(o) && periodicOutputEligible(o)
}

func regularOutputFile(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok || file == nil {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode().IsRegular()
}

func periodicModeEligible(o options) bool {
	return !o.checkpoint && o.checkpointInterval == 0 && o.recoverDir == "" && !o.simulate && !o.dfid && !o.continuation
}

func periodicOutputEligible(o options) bool {
	return !o.debugLiveness && o.tracePath == "" && o.cpuPath == "" && o.heapPath == "" && !o.compat.debug &&
		o.dumpPath == "" && o.compat.userFile == "" && o.compat.specTE == nil
}

type periodicCoverage struct {
	ctx      context.Context
	opts     options
	progress *checker.LiveProgress
	output   *coordinatedEventOutput
	timer    func(time.Duration) periodicTimer
	done     chan struct{}
	joined   chan struct{}
	started  bool
}

func newPeriodicCoverage(ctx context.Context, opts options, progress *checker.LiveProgress, output *coordinatedEventOutput, timer func(time.Duration) periodicTimer) *periodicCoverage {
	return &periodicCoverage{ctx: ctx, opts: opts, progress: progress, output: output, timer: timer, done: make(chan struct{}), joined: make(chan struct{})}
}

func (p *periodicCoverage) start() {
	if p.started {
		return
	}
	p.started = true
	// Create the timer here, not in the goroutine, so scheduling is anchored to
	// initial completion even if the goroutine is not scheduled immediately.
	timer := p.timer(3 * time.Second)
	go p.run(timer)
}

func (p *periodicCoverage) stop() {
	if !p.started {
		return
	}
	close(p.done)
	<-p.joined
}

func (p *periodicCoverage) run(timer periodicTimer) {
	defer close(p.joined)
	defer func() { timer.Stop() }()
	count := int(p.opts.coverageInterval / time.Minute)
	var generated, distinct int
	for {
		select {
		case <-p.done:
			return
		case <-p.ctx.Done():
			return
		case <-timer.C():
		}
		if p.stopping() {
			return
		}
		generated, distinct = p.report(count == 0, generated, distinct)
		if count == 0 {
			count = int(p.opts.coverageInterval / time.Minute)
		} else {
			count--
		}
		timer.Stop()
		timer = p.timer(time.Minute)
	}
}

func (p *periodicCoverage) stopping() bool {
	if p.ctx.Err() != nil {
		return true
	}
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

func (p *periodicCoverage) report(includeCoverage bool, generated, distinct int) (int, int) {
	snap := p.progress.Snapshot()
	var coverage []checker.CoverageAction
	if includeCoverage {
		coverage = p.progress.CoverageSnapshot()
	}
	p.output.render(func(w io.Writer) {
		body := fmt.Sprintf("Progress(%d) at %s: %s states generated (%s s/min), %s distinct states found (%s ds/min), %s states left on queue.", snap.Depth, toolTime(), grouped(snap.GeneratedStates), grouped(snap.GeneratedStates-generated), grouped(snap.DistinctStates), grouped(snap.DistinctStates-distinct), grouped(snap.QueuedStates))
		if p.opts.tool {
			(toolOutput{out: w}).message(2200, 0, body)
			printToolCoverage(toolOutput{out: w}, coverage)
		} else {
			fmt.Fprintln(w, body)
			printCoverage(w, coverage)
		}
	})
	return snap.GeneratedStates, snap.DistinctStates
}

func grouped(n int) string {
	s := strconv.Itoa(n)
	start := 0
	if strings.HasPrefix(s, "-") {
		start = 1
	}
	for i := len(s) - 3; i > start; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
