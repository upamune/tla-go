package checker

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestCheckpointTimedStopAndRecovery(t *testing.T) {
	ctx := context.Background()
	opts := CheckpointOptions{Path: filepath.Join(t.TempDir(), "checkpoint"), Interval: time.Hour, DebugStopAfter: time.Nanosecond}
	stopped, err := checkWithCheckpointClock(ctx, observerSource, observerConfig, opts, advancingCheckpointClock())
	if err != nil || !stopped.Paused || !stopped.Stopped {
		t.Fatalf("stop: %+v %v", stopped, err)
	}
	before, err := os.ReadFile(opts.Path)
	if err != nil {
		t.Fatal(err)
	}
	opts.Resume = true
	opts.DebugStopAfter = 0
	if _, err := CheckWithCheckpoint(ctx, observerSource+"\n", observerConfig, opts); err == nil {
		t.Fatal("accepted changed model")
	}
	after, err := os.ReadFile(opts.Path)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("rejected recovery changed checkpoint: %v", err)
	}
	resumed, err := CheckWithCheckpoint(ctx, observerSource, observerConfig, opts)
	if err != nil || resumed.Paused || resumed.Stopped {
		t.Fatalf("resume: %+v %v", resumed, err)
	}
	full, err := CheckWithOptions(ctx, observerSource, observerConfig, Options{})
	if err != nil || !reflect.DeepEqual(full, resumed.Result) {
		t.Fatalf("full: %+v %v; resumed: %+v", full, err, resumed.Result)
	}
}

func TestCheckpointTimedStopNamedSpecification(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals
VARIABLES x, y
Step(n) == n + 1
Init == x = 0 /\ y \in 1..3
Next == x < 3 /\ x' = Step(x) /\ UNCHANGED y
Spec == Init /\ [][Next]_<<x, y>>
====`
	config := "SPECIFICATION Spec\nCHECK_DEADLOCK FALSE"
	ctx := context.Background()
	opts := CheckpointOptions{Path: filepath.Join(t.TempDir(), "checkpoint"), Interval: time.Hour, DebugStopAfter: time.Nanosecond}
	stopped, err := checkWithCheckpointClock(ctx, source, config, opts, advancingCheckpointClock())
	if err != nil || !stopped.Paused || !stopped.Stopped {
		t.Fatalf("stop: %+v %v", stopped, err)
	}
	opts.Resume = true
	opts.DebugStopAfter = 0
	resumed, err := CheckWithCheckpoint(ctx, source, config, opts)
	if err != nil || resumed.Paused || resumed.Stopped {
		t.Fatalf("resume: %+v %v", resumed, err)
	}
	full, err := CheckWithOptions(ctx, source, config, Options{})
	if err != nil || !reflect.DeepEqual(full, resumed.Result) {
		t.Fatalf("full: %+v %v; resumed: %+v", full, err, resumed.Result)
	}
}

func advancingCheckpointClock() func() time.Time {
	now := time.Unix(0, 0)
	return func() time.Time {
		now = now.Add(time.Nanosecond)
		return now
	}
}

func TestCheckpointTimedStopEmptyQueueAndFailure(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "checkpoint")
	opts := CheckpointOptions{Path: path, Interval: time.Hour, DebugStopAfter: time.Nanosecond}
	source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n===="
	result, err := CheckWithCheckpoint(ctx, source, observerConfig, opts)
	if err != nil || result.Paused || result.Stopped || result.Result.Status != Success {
		t.Fatalf("completed: %+v %v", result, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("completed search saved checkpoint: %v", err)
	}
	failureSource := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\nInv == x = 0\n===="
	result, err = CheckWithCheckpoint(ctx, failureSource, observerConfig+" INVARIANT Inv", opts)
	if err != nil || result.Paused || result.Stopped || result.Result.Status == Success {
		t.Fatalf("failure: %+v %v", result, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("failed search saved checkpoint: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := CheckWithCheckpoint(canceled, observerSource, observerConfig, opts); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled search: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("canceled search saved checkpoint: %v", err)
	}
}
