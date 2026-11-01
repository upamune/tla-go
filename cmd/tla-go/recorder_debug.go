package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/internal/recorderdebug"
)

type recorderConfig struct {
	path string
	file *profileFile
	mu   sync.Mutex
}

const recorderModeError = "TLA_GO_DEBUG_RECORDER requires one model in tool model-checking mode"

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
	if !recorderModelCheckMode(*opts) {
		return fmt.Errorf("%s", recorderModeError)
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

func recorderModelCheckMode(opts options) bool {
	return opts.tool && !opts.simulate && !opts.dfid && !opts.help && !opts.version && !opts.sanyMetadata && len(opts.paths) == 1 && recorderOrdinarySearch(opts)
}

func recorderOrdinarySearch(opts options) bool {
	return opts.recoverDir == "" && opts.checkpointInterval == 0
}

func recorderPath(opts options) string {
	if opts.recorder == nil {
		return ""
	}
	return opts.recorder.path
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

func recordStrings(recorder *recorderConfig, code int, values ...string) {
	if recorder == nil {
		return
	}
	if values == nil {
		values = []string{}
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	// profileFile retains write errors for runWithInput's deferred close.
	_ = json.NewEncoder(recorder.file).Encode(recorderRecord{Kind: "record", Code: code, Array: "strings", Values: values})
}

type recorderRecord struct {
	Kind     string   `json:"kind"`
	Complete bool     `json:"complete"`
	Code     int      `json:"code"`
	Array    string   `json:"array"`
	Values   []string `json:"values"`
}

func recordStatistics(recorder *recorderConfig, result checker.Result) {
	if recorder == nil {
		return
	}
	recordStrings(recorder, 2199, strconv.Itoa(result.GeneratedStates), strconv.Itoa(result.DistinctStates), strconv.Itoa(result.QueuedStates))
}

func recordSuccess(recorder *recorderConfig, result checker.Result) {
	if result.DistinctStates < 0 || result.GeneratedStates < result.DistinctStates || successCollisionProbability(result) >= 1e-10 {
		recordOmission(recorder, 2193, recorderdebug.SuccessProbabilityOmission)
		return
	}
	recordStrings(recorder, 2193, "val = "+successProbability(result))
}

func recordOmission(recorder *recorderConfig, code int, reason string) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	_ = json.NewEncoder(recorder.file).Encode(recorderdebug.Event{Kind: "omission", Code: code, Reason: reason})
}

func recordSearchEvent(recorder *recorderConfig, event checker.SearchEvent) {
	if recorder == nil {
		return
	}
	switch event.Kind {
	case checker.InitialStart:
		recordStrings(recorder, 2189)
	case checker.InitialProgress:
		recordStrings(recorder, 2269, strconv.Itoa(event.GeneratedStates))
	case checker.InitialComplete:
		generated, distinct := event.GeneratedStates, event.DistinctStates
		if generated == distinct {
			recordStrings(recorder, 2190, strconv.Itoa(generated), plural(generated))
		} else {
			recordStrings(recorder, 2191, strconv.Itoa(generated), plural(generated), strconv.Itoa(distinct))
		}
	}
}
