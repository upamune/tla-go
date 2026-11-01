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
