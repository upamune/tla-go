package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/config"
)

// checkCLIWithCheckpoint falls back only before exploration, and never on recovery.
func checkCLIWithCheckpoint(ctx context.Context, opts options, cfg config.Config, source []byte, specPath string, out, errout io.Writer) (checker.Result, bool, error) {
	if checkpointCLIUnsupported(opts) {
		if opts.recoverDir != "" {
			return checker.Result{}, true, fmt.Errorf("checkpoint recovery does not support these options")
		}
		fmt.Fprintln(errout, "Checkpointing unavailable for these options; checking normally without checkpoints.")
		return checker.Result{}, false, nil
	}
	configSource, interval, debug, err := checkpointInputs(opts)
	if err != nil {
		return checker.Result{}, true, err
	}
	fmt.Fprintf(errout, "Checkpoint directory (if model is supported): %s\n", opts.metaDir)
	stream := make(chan checker.CheckpointObservation)
	type answer struct {
		run checker.CheckpointRun
		err error
	}
	done := make(chan answer, 1)
	checkpointOpts := checker.CheckpointOptions{Path: filepath.Join(opts.metaDir, "tla-go.chkpt"), Interval: interval, Resume: opts.recoverDir != "", Config: &cfg, DebugPauseAfterExpansions: debug, Stream: stream, Options: checker.Options{Loader: directoryLoader(filepath.Dir(specPath), opts.libraryPaths), Seed: opts.seed, Fingerprint: opts.fingerprint, ModernRandomSubsets: opts.modernRandomSubsets, MaxSetSize: opts.maxSetSize, Workers: opts.workers, SerialExploration: true}}
	go func() {
		run, e := checker.CheckWithCheckpoint(ctx, string(source), string(configSource), checkpointOpts)
		close(stream)
		done <- answer{run, e}
	}()
	deliverCheckpointEvents(stream, opts, out)
	answerValue := <-done
	var unsupportedErr *checker.CheckpointUnsupportedError
	if errors.As(answerValue.err, &unsupportedErr) && opts.recoverDir == "" {
		fmt.Fprintf(errout, "Checkpointing unavailable (%v); checking normally without checkpoints.\n", answerValue.err)
		return checker.Result{}, false, nil
	}
	if answerValue.err != nil {
		return checker.Result{}, true, answerValue.err
	}
	if answerValue.run.Paused {
		return checker.Result{}, true, fmt.Errorf("debug checkpoint saved in %s; search paused", opts.metaDir)
	}
	return answerValue.run.Result, true, nil
}

func checkpointCLIUnsupported(opts options) bool {
	return opts.simulate || opts.dfid || opts.continuation || opts.coverage || opts.view || opts.dumpPath != "" || checkpointExtraUnsupported(opts)
}

func deliverCheckpointEvents(stream <-chan checker.CheckpointObservation, opts options, out io.Writer) {
	search := configuredSearchOutput(opts, out)
	warning := configuredWarningOutput(opts, out)
	for event := range stream {
		if event.Search != nil && search != nil {
			search(*event.Search)
		}
		if event.Warning != nil && warning != nil {
			warning(*event.Warning)
		}
		if event.Print != nil {
			fmt.Fprintln(out, *event.Print)
		}
	}
}

func checkpointInputs(opts options) ([]byte, time.Duration, int, error) {
	_, cfgPath, err := inputPaths(opts)
	if err != nil {
		return nil, 0, 0, err
	}
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, 0, 0, err
	}
	interval := opts.checkpointInterval
	if !opts.checkpoint {
		// TLCGlobals.DEFAULT_CHECKPOINT_DURATION in Java TLC 1.7.4.
		interval = 30*time.Minute + 42*time.Millisecond
	}
	if interval == 0 {
		return cfg, interval, 0, nil
	}
	debug, err := debugCheckpointExpansion()
	return cfg, interval, debug, err
}

func debugCheckpointExpansion() (int, error) {
	raw := os.Getenv("TLA_GO_DEBUG_CHECKPOINT_AFTER")
	if raw == "" {
		return 0, nil
	}
	debug, err := strconv.Atoi(raw)
	if err != nil || debug <= 0 {
		return 0, fmt.Errorf("invalid TLA_GO_DEBUG_CHECKPOINT_AFTER")
	}
	return debug, nil
}

func checkpointExtraUnsupported(opts options) bool {
	return opts.debugLiveness || opts.workers != 1 || opts.compat.userFile != "" || opts.compat.specTE != nil
}
