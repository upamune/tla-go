package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/internal/recorderdebug"
)

type recorderConfig struct {
	path string
	file *profileFile
}

func startRecorderOutput(file *profileFile) error {
	if file == nil {
		return nil
	}
	if err := json.NewEncoder(file).Encode(recorderdebug.Event{Kind: "header", Version: 1}); err != nil {
		return fmt.Errorf("start recorder diagnostics: %w", err)
	}
	return nil
}

func configureRecorder(opts *options) error {
	path := os.Getenv("TLA_GO_DEBUG_RECORDER")
	if path == "" {
		return nil
	}
	opts.recorder = &recorderConfig{path: path}
	if !opts.tool || opts.simulate || opts.dfid || len(opts.paths) != 1 {
		return fmt.Errorf("TLA_GO_DEBUG_RECORDER requires one model in tool model-checking mode")
	}
	paths, err := expandModelPaths(opts.paths, opts.configPath != "")
	if err != nil {
		return err
	}
	if len(paths) != 1 {
		return fmt.Errorf("TLA_GO_DEBUG_RECORDER requires exactly one expanded model")
	}
	return nil
}

func recorderPath(opts options) string {
	if opts.recorder == nil {
		return ""
	}
	return opts.recorder.path
}

func recorderFile(opts options) *profileFile {
	if opts.recorder == nil {
		return nil
	}
	return opts.recorder.file
}

func bindRecorder(opts *options, file *profileFile) {
	if opts.recorder != nil {
		opts.recorder.file = file
	}
}

func configureDiagnosticOptions(opts *options, out io.Writer) error {
	if err := configureRecorder(opts); err != nil {
		return err
	}
	return validateOutputPaths(*opts, out)
}

func recordStrings(recorder *profileFile, code int, values ...string) {
	if recorder == nil {
		return
	}
	// profileFile retains write errors for runWithInput's deferred close.
	_ = json.NewEncoder(recorder).Encode(recorderdebug.Event{Kind: "record", Code: code, Array: "strings", Values: values})
}

func recordStatistics(recorder *profileFile, result checker.Result) {
	if recorder == nil {
		return
	}
	recordStrings(recorder, 2199, strconv.Itoa(result.GeneratedStates), strconv.Itoa(result.DistinctStates), strconv.Itoa(result.QueuedStates))
}
