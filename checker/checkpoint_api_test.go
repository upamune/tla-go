package checker

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestPublicCheckpointDisabledRecovery(t *testing.T) {
	ctx := context.Background()
	opts := CheckpointOptions{Path: filepath.Join(t.TempDir(), "checkpoint"), Interval: time.Hour, DebugPauseAfterExpansions: 1}
	if run, err := CheckWithCheckpoint(ctx, observerSource, observerConfig, opts); err != nil || !run.Paused {
		t.Fatalf("save: %+v %v", run, err)
	}
	before, err := os.ReadFile(opts.Path)
	if err != nil {
		t.Fatal(err)
	}
	opts.Resume, opts.Interval, opts.PauseOnCheckpoint = true, 0, true
	missing := opts
	missing.Path += ".missing"
	if _, err := CheckWithCheckpoint(ctx, observerSource, observerConfig, missing); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing recovery: %v", err)
	}
	if _, err := CheckWithCheckpoint(ctx, observerSource+"\n", observerConfig, opts); err == nil {
		t.Fatal("accepted changed model during disabled recovery")
	}
	// A previously passed debug tick must not prevent read-only recovery.
	if run, err := CheckWithCheckpoint(ctx, observerSource, observerConfig, opts); err != nil || run.Paused {
		t.Fatalf("recover: %+v %v", run, err)
	}
	after, err := os.ReadFile(opts.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("snapshot changed: %v", err)
	}
	opts.Resume = false
	if _, err := CheckWithCheckpoint(ctx, observerSource, observerConfig, opts); err == nil {
		t.Fatal("accepted zero interval for a new search")
	}
	opts.Resume, opts.Interval = true, -time.Second
	if _, err := CheckWithCheckpoint(ctx, observerSource, observerConfig, opts); err == nil {
		t.Fatal("accepted negative interval")
	}
}

func TestPublicCheckpointRecovery(t *testing.T) {
	for _, invariant := range []string{"x <= 4", "x < 4"} {
		t.Run(invariant, func(t *testing.T) {
			source := observerSource[:len(observerSource)-4] + "Inv == " + invariant + "\n===="
			cfg := observerConfig + " INVARIANT Inv"
			opts := CheckpointOptions{Path: filepath.Join(t.TempDir(), "checkpoint"), Interval: time.Nanosecond}
			full, err := CheckWithCheckpoint(context.Background(), source, cfg, opts)
			if err != nil || full.Paused {
				t.Fatalf("full: %+v %v", full, err)
			}
			opts.PauseOnCheckpoint = true
			first, err := CheckWithCheckpoint(context.Background(), source, cfg, opts)
			if err != nil || !first.Paused {
				t.Fatalf("pause: %+v %v", first, err)
			}
			opts.Resume = true
			second, err := CheckWithCheckpoint(context.Background(), source, cfg, opts)
			if err != nil || !second.Paused {
				t.Fatalf("second: %+v %v", second, err)
			}
			opts.PauseOnCheckpoint = false
			got, err := CheckWithCheckpoint(context.Background(), source, cfg, opts)
			if err != nil || got.Paused || !reflect.DeepEqual(got.Result, full.Result) || !reflect.DeepEqual(got.Observations, full.Observations) {
				t.Fatalf("recovery: %+v %v; full: %+v", got, err, full)
			}
			if len(first.Observations) == 0 || len(second.Observations) < len(first.Observations) || !reflect.DeepEqual(second.Observations[:len(first.Observations)], first.Observations) {
				t.Fatal("observation prefix changed")
			}
		})
	}
}

func TestPublicCheckpointErrors(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "checkpoint")
	opts := CheckpointOptions{Path: path, Interval: time.Nanosecond, PauseOnCheckpoint: true}
	if _, err := CheckWithCheckpoint(ctx, observerSource, observerConfig, opts); err != nil {
		t.Fatal(err)
	}
	opts.Resume = true
	if _, err := CheckWithCheckpoint(ctx, observerSource+"\n", observerConfig, opts); err == nil {
		t.Fatal("accepted changed model")
	}
	missing := opts
	missing.Path += "missing"
	if _, err := CheckWithCheckpoint(ctx, observerSource, observerConfig, missing); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing: %v", err)
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckWithCheckpoint(ctx, observerSource, observerConfig, opts); err == nil {
		t.Fatal("accepted corrupt file")
	}
	opts.Resume = false
	opts.Path = filepath.Join(t.TempDir(), "missing", "file")
	if _, err := CheckWithCheckpoint(ctx, observerSource, observerConfig, opts); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("write: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := CheckWithCheckpoint(canceled, observerSource, observerConfig, opts); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	calls := 0
	opts.Options.OnSearch = func(SearchEvent) { calls++ }
	if _, err := CheckWithCheckpoint(ctx, observerSource, observerConfig, opts); err == nil || calls != 0 {
		t.Fatalf("callback: %v %d", err, calls)
	}
	opts.Options = Options{Continue: true}
	if _, err := CheckWithCheckpoint(ctx, observerSource, observerConfig, opts); err == nil {
		t.Fatal("accepted continue")
	}
}

func TestCheckpointObservationLogRoundTrip(t *testing.T) {
	print := "line"
	warning := Warning{Code: 7, Message: "warning"}
	event := SearchEvent{Kind: InitialStart}
	original := []CheckpointObservation{{Search: &event}, {Warning: &warning}, {Print: &print}, {Warning: &warning}}
	snapshot := checkpointFixture()
	snapshot.ObservationLog = true
	snapshot.Observations = original
	data, err := checkpointEncode(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := checkpointDecode(data, snapshot.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCheckpointObservations(restored.Observations); err != nil || !reflect.DeepEqual(restored.Observations, original) {
		t.Fatalf("observations: %+v %v", restored.Observations, err)
	}
	if err := validateCheckpointObservations([]CheckpointObservation{{}}); err == nil {
		t.Fatal("accepted empty observation")
	}
}

func TestCheckpointElapsedSchedule(t *testing.T) {
	var disabled checkpointDriver
	if disabled.due(1) || disabled.boundaryDue(1) {
		t.Fatal("zero schedule must be disabled")
	}
	disabled.pauseAt = 1
	if disabled.boundaryDue(1) {
		t.Fatal("debug pause must not enable a disabled schedule")
	}
	d := checkpointDriver{interval: time.Hour, next: time.Now().Add(time.Hour)}
	if d.due(1) || d.due(1000000) {
		t.Fatal("elapsed schedule used expansion counts")
	}
	d.next = time.Now().Add(-time.Second)
	if !d.due(1) {
		t.Fatal("elapsed schedule missed deadline")
	}
	d = checkpointDriver{every: 3}
	if d.due(2) || !d.due(3) {
		t.Fatal("deterministic schedule changed")
	}
}

func TestCheckpointStreamMatchesOrderedLog(t *testing.T) {
	stream := make(chan CheckpointObservation)
	type response struct {
		run CheckpointRun
		err error
	}
	done := make(chan response, 1)
	opts := CheckpointOptions{Path: filepath.Join(t.TempDir(), "checkpoint"), Interval: time.Hour, DebugPauseAfterExpansions: 1, Stream: stream}
	go func() {
		run, err := CheckWithCheckpoint(context.Background(), observerSource, observerConfig, opts)
		close(stream)
		done <- response{run, err}
	}()
	var received []CheckpointObservation
	for event := range stream {
		received = append(received, event)
	}
	first := <-done
	if first.err != nil || !first.run.Paused || !reflect.DeepEqual(received, first.run.Observations) {
		t.Fatalf("first: %+v %v stream %+v", first.run, first.err, received)
	}
	opts.Resume, opts.DebugPauseAfterExpansions = true, 0
	stream = make(chan CheckpointObservation)
	opts.Stream = stream
	go func() {
		run, err := CheckWithCheckpoint(context.Background(), observerSource, observerConfig, opts)
		close(stream)
		done <- response{run, err}
	}()
	received = nil
	for event := range stream {
		received = append(received, event)
	}
	second := <-done
	if second.err != nil || !slices.EqualFunc(received, second.run.Observations[len(first.run.Observations):], func(a, b CheckpointObservation) bool { return reflect.DeepEqual(a, b) }) {
		t.Fatalf("recovery: %+v %v stream %+v", second.run, second.err, received)
	}
}

func TestCheckpointStreamPreparationWarning(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nUnused == [<<0>> EXCEPT ![2] = 0]\nInit == x = 0\nNext == /\\ x' = x + 1 /\\ x < 4\n===="
	const cfg = "INIT Init NEXT Next CHECK_DEADLOCK FALSE"
	path := filepath.Join(t.TempDir(), "checkpoint")
	stream := make(chan CheckpointObservation, 100)
	opts := CheckpointOptions{Path: path, Interval: time.Hour, DebugPauseAfterExpansions: 1, Stream: stream}
	first, err := CheckWithCheckpoint(context.Background(), source, cfg, opts)
	if err != nil || !first.Paused {
		t.Fatalf("first: %+v %v", first, err)
	}
	drain := func() []CheckpointObservation {
		var events []CheckpointObservation
		for len(stream) > 0 {
			events = append(events, <-stream)
		}
		return events
	}
	initial := drain()
	if !reflect.DeepEqual(initial, first.Observations) || !slices.ContainsFunc(initial, func(o CheckpointObservation) bool { return o.Warning != nil }) {
		t.Fatalf("fresh run did not stream preparation warning: %+v", initial)
	}
	opts.Resume, opts.DebugPauseAfterExpansions = true, 0
	for _, tc := range []struct{ name, source, path string }{
		{"missing", source, path + "-missing"},
		{"mismatch", source + "\n", path},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts.Path = tc.path
			if _, err := CheckWithCheckpoint(context.Background(), tc.source, cfg, opts); err == nil || len(stream) != 0 {
				t.Fatalf("error: %v, streamed: %+v", err, drain())
			}
		})
	}
	opts.Path = path
	resumed, err := CheckWithCheckpoint(context.Background(), source, cfg, opts)
	newEvents := drain()
	if err != nil || len(resumed.Observations) < len(initial) || !reflect.DeepEqual(resumed.Observations[:len(initial)], initial) || !slices.EqualFunc(newEvents, resumed.Observations[len(initial):], func(a, b CheckpointObservation) bool { return reflect.DeepEqual(a, b) }) {
		t.Fatalf("recovery: %+v %v", resumed, err)
	}
	fullOpts := CheckpointOptions{Path: filepath.Join(t.TempDir(), "checkpoint"), Interval: time.Hour}
	full, err := CheckWithCheckpoint(context.Background(), source, cfg, fullOpts)
	if err != nil || !reflect.DeepEqual(full.Observations, resumed.Observations) || !reflect.DeepEqual(full.Result, resumed.Result) {
		t.Fatalf("full: %+v %v; resumed: %+v", full, err, resumed)
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckWithCheckpoint(context.Background(), source, cfg, opts); err == nil || len(stream) != 0 {
		t.Fatalf("corrupt: %v, streamed: %+v", err, drain())
	}
	opts.Resume, opts.DebugPauseAfterExpansions = false, 1
	opts.Path = filepath.Join(t.TempDir(), "missing", "checkpoint")
	if _, err := CheckWithCheckpoint(context.Background(), source, cfg, opts); err == nil || !slices.ContainsFunc(drain(), func(o CheckpointObservation) bool { return o.Warning != nil }) {
		t.Fatalf("fresh write error lost warning: %v", err)
	}
}
