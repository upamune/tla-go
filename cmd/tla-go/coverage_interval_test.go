package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/checker"
)

func TestCoverageDeadline(t *testing.T) {
	for _, tc := range []struct {
		name      string
		interval  time.Duration
		window    time.Duration
		elapsed   time.Duration
		expired   bool
		checkErr  error
		wantError string
		wantTimer bool
	}{
		{name: "disabled or zero", interval: 0},
		{name: "short success", interval: time.Minute, elapsed: 59 * time.Second, wantTimer: true},
		{name: "short failure", interval: time.Minute, elapsed: time.Second, checkErr: errors.New("model error"), wantError: "model error", wantTimer: true},
		{name: "interval elapsed", interval: time.Minute, elapsed: time.Minute, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
		{name: "wrapped interval first progress", interval: 12704 * time.Millisecond, window: 3 * time.Second, elapsed: 3 * time.Second, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
		{name: "below three seconds", interval: 3*time.Second - time.Millisecond, elapsed: 3*time.Second - time.Millisecond, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
		{name: "three seconds", interval: 3 * time.Second, elapsed: 3 * time.Second, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
		{name: "above three seconds", interval: 3*time.Second + time.Millisecond, window: 3 * time.Second, elapsed: 3*time.Second - time.Nanosecond, wantTimer: true},
		{name: "below one minute", interval: time.Minute - time.Millisecond, window: 3 * time.Second, elapsed: 3 * time.Second, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
		{name: "above one minute", interval: time.Minute + time.Millisecond, elapsed: time.Minute + time.Millisecond, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
		{name: "below rounded boundary", interval: 63*time.Second - time.Millisecond, elapsed: 63*time.Second - time.Millisecond, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
		{name: "at rounded boundary", interval: 63 * time.Second, elapsed: 63 * time.Second, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
		{name: "above rounded boundary", interval: 63*time.Second + time.Millisecond, window: 63 * time.Second, elapsed: 63 * time.Second, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
		{name: "wrapped before first report", interval: 72704 * time.Millisecond, window: 63 * time.Second, elapsed: 63*time.Second - time.Nanosecond, wantTimer: true},
		{name: "wrapped first report", interval: 72704 * time.Millisecond, window: 63 * time.Second, elapsed: 63 * time.Second, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
		{name: "two minutes", interval: 2 * time.Minute, elapsed: 2 * time.Minute, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
		{name: "above two minutes plus three seconds", interval: 123*time.Second + time.Millisecond, window: 123 * time.Second, elapsed: 123 * time.Second, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
		{name: "largest unwrapped minute interval", interval: 2147460000 * time.Millisecond, elapsed: 2147460000 * time.Millisecond, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
		{name: "largest positive wrapped interval", interval: 2147483632 * time.Millisecond, window: 2147463000 * time.Millisecond, elapsed: 2147463000 * time.Millisecond, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
		{name: "multiple intervals elapsed", interval: time.Minute, elapsed: 3 * time.Minute, expired: true, wantError: "periodic coverage reports are not yet supported", wantTimer: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			timerCalled := false
			elapsed := time.Duration(0)
			timer := func(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
				timerCalled = true
				window := tc.interval
				if tc.window != 0 {
					window = tc.window
				}
				if d != window {
					t.Fatalf("interval %v", d)
				}
				return intervalTestContext{Context: ctx, elapsed: &elapsed, interval: d}, func() {}
			}
			want := checker.Result{Status: checker.Success, Coverage: []checker.CoverageAction{{Name: "Init"}}}
			got, err := checkWithCoverageDeadlineClock(context.Background(), tc.interval, timer, func() time.Time {
				return time.Unix(0, 0).Add(elapsed)
			}, func(context.Context) (checker.Result, error) {
				elapsed = tc.elapsed
				return want, tc.checkErr
			})
			if timerCalled != tc.wantTimer {
				t.Fatalf("timer called: %v", timerCalled)
			}
			if tc.wantError == "" && err != nil || tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("error: %v", err)
			}
			if tc.expired && len(got.Coverage) != 0 {
				t.Fatal("returned incomplete coverage")
			}
			if !tc.expired && len(got.Coverage) != 1 {
				t.Fatal("lost coverage")
			}
		})
	}
}

type intervalTestContext struct {
	context.Context
	elapsed  *time.Duration
	interval time.Duration
}

func (c intervalTestContext) Err() error {
	if *c.elapsed >= c.interval {
		return context.DeadlineExceeded
	}
	return c.Context.Err()
}

func TestCoverageIntervalParsing(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  time.Duration
	}{
		{"0", 0},
		{"1", time.Minute},
		{"35791", 2147460000 * time.Millisecond},
		{"71583", 12704 * time.Millisecond},
		{"71584", 72704 * time.Millisecond},
		{"1073741824", 0},
		{"-2147483648", 0},
	} {
		t.Run(tc.input, func(t *testing.T) {
			opts, err := parseOptions([]string{"-coverage", tc.input, "M.tla"})
			if err != nil || !opts.coverage || opts.coverageInterval != tc.want {
				t.Fatalf("coverage %v interval %v error %v", opts.coverage, opts.coverageInterval, err)
			}
		})
	}
}

func TestCoverageDeadlineDoesNotWaitForTimerDelivery(t *testing.T) {
	start := time.Unix(0, 0)
	now := start
	timer := func(ctx context.Context, _ time.Duration) (context.Context, context.CancelFunc) {
		return ctx, func() {}
	}
	result, err := checkWithCoverageDeadlineClock(context.Background(), time.Minute, timer, func() time.Time { return now }, func(context.Context) (checker.Result, error) {
		now = start.Add(time.Minute)
		return checker.Result{Coverage: []checker.CoverageAction{{Name: "Init"}}}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "periodic coverage reports are not yet supported") || len(result.Coverage) != 0 {
		t.Fatalf("result: %+v, error: %v", result, err)
	}
}

// A real check whose initialization takes longer than the first Java progress
// tick must still finish for zero and for a positive, unexpired interval.
func TestCoverageLongInitializationPreservesResults(t *testing.T) {
	for _, interval := range []time.Duration{0, time.Minute} {
		t.Run(interval.String(), func(t *testing.T) {
			t.Parallel()
			const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..1\nA == x' = x\nB == x' = x\nNext == A \\/ B\n====\n"
			result, err := checkWithCoverageDeadline(context.Background(), interval, context.WithTimeout, func(ctx context.Context) (checker.Result, error) {
				return checker.CheckWithOptions(ctx, source, "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n", checker.Options{Coverage: true, OnSearch: func(event checker.SearchEvent) {
					if event.Kind == checker.InitialStart {
						time.Sleep(3100 * time.Millisecond)
					}
				}})
			})
			if err != nil || result.Status != checker.Success || len(result.Coverage) == 0 {
				t.Fatalf("result: %+v, error: %v", result, err)
			}
		})
	}
}

func TestCoverageDeadlinePreservesExternalCancellation(t *testing.T) {
	for _, interval := range []time.Duration{0, time.Minute} {
		ctx, cancel := context.WithCancel(context.Background())
		_, err := checkWithCoverageDeadline(ctx, interval, context.WithTimeout, func(ctx context.Context) (checker.Result, error) {
			cancel()
			return checker.Result{}, ctx.Err()
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("interval %v: %v", interval, err)
		}
	}
}
