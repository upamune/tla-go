package checker

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime/trace"
	"testing"
	"time"
)

func TestCheckpointTraceRegions(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	opts := CheckpointOptions{Path: filepath.Join(dir, "checkpoint"), Interval: time.Hour, DebugPauseAfterExpansions: 1}
	traceRun := func(name string) CheckpointRun {
		t.Helper()
		path := filepath.Join(dir, name)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := trace.Start(f); err != nil {
			t.Fatal(err)
		}
		run, runErr := CheckWithCheckpoint(ctx, observerSource, observerConfig, opts)
		trace.Stop()
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		if runErr != nil {
			t.Fatal(runErr)
		}
		output, err := exec.Command("go", "tool", "trace", "-d=parsed", path).CombinedOutput()
		if err != nil {
			t.Fatalf("parse trace: %v %s", err, output)
		}
		for _, phase := range []string{"model initialization", "exploration and liveness"} {
			if !bytes.Contains(output, []byte(phase)) {
				t.Errorf("%s missing %s", name, phase)
			}
		}
		return run
	}
	first := traceRun("first.trace")
	if !first.Paused {
		t.Fatal("did not pause")
	}
	opts.Resume, opts.DebugPauseAfterExpansions = true, 0
	resumed := traceRun("resumed.trace")
	if resumed.Paused {
		t.Fatal("still paused")
	}
	opts.Resume = false
	plain, err := CheckWithCheckpoint(ctx, observerSource, observerConfig, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(resumed.Result, plain.Result) || !reflect.DeepEqual(resumed.Observations, plain.Observations) {
		t.Fatal("recovered result differs from uninterrupted run")
	}
}

func TestFinalLivenessTraceRegion(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = 1 - x\nP == <> (x = 1)\n====\n"
	cfg := "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
	path := filepath.Join(t.TempDir(), "live.trace")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := trace.Start(f); err != nil {
		t.Fatal(err)
	}
	result, checkErr := CheckContext(context.Background(), source, cfg)
	trace.Stop()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if checkErr != nil || result.Status != LivenessViolation {
		t.Fatalf("check: %+v %v", result, checkErr)
	}
	output, err := exec.Command("go", "tool", "trace", "-d=parsed", path).CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte("final liveness check")) {
		t.Fatalf("trace: %v %s", err, output)
	}
}
