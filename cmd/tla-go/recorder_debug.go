package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/internal/recorderdebug"
)

type recorderConfig struct {
	path string
	file *profileFile
	mu   sync.Mutex
}

const recorderModeError = "TLA_GO_DEBUG_RECORDER requires one model in model-checking mode"

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
	return !opts.simulate && !opts.dfid && !opts.help && !opts.version && !opts.sanyMetadata && len(opts.paths) == 1 && recorderOrdinarySearch(opts)
}

// javaHumanDuration follows TLC 1.7.4's UTC SimpleDateFormat selection.
func javaHumanDuration(elapsed time.Duration) string {
	millis := elapsed.Milliseconds()
	second := millis / 1000
	switch {
	case millis > 24*60*60*1000:
		date := time.UnixMilli(millis - 24*60*60*1000).UTC()
		return fmt.Sprintf("%dd %02dh", date.YearDay(), date.Hour())
	case millis > 60*60*1000:
		return fmt.Sprintf("%02dh %02dmin", second/3600%24, second/60%60)
	case millis > 60*1000:
		return fmt.Sprintf("%02dmin %02ds", second/60%60, second%60)
	default:
		return fmt.Sprintf("%02ds", second%60)
	}
}

// recordHumanResult records only values already emitted by the checker. The
// recorder header remains incomplete; failures and startup are not complete.
func recordHumanResult(recorder *recorderConfig, result checker.Result) {
	if recorder == nil {
		return
	}
	if result.Failure == nil && result.Violation == nil {
		recordSuccess(recorder, result)
	}
	if toolHasStatistics(result) {
		printToolCoverage(toolOutput{out: io.Discard, recorder: recorder}, result.Coverage)
		recordStatistics(recorder, result)
		// Java's final liveness check returns an error from modelCheck before
		// printSummary, suppressing depth. Safety and deadlock violations can
		// still leave the internal result successful and report depth.
		if result.Status != checker.LivenessViolation && (result.Failure == nil || !result.Failure.SuppressDepth) {
			recordStrings(recorder, 2194, strconv.Itoa(result.Depth))
		}
	}
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
	case checker.ModelCheckingLiveness, checker.TemporalCheckStart:
		recordLivenessSearch(recorder, event)
	case checker.TemporalCheckUncertain:
		recordOmission(recorder, 2192, recorderdebug.TemporalCheckUncertain)
	default:
		recordInitialSearch(recorder, event)
	}
}

func recordLivenessSearch(recorder *recorderConfig, event checker.SearchEvent) {
	switch event.Kind {
	case checker.ModelCheckingLiveness:
		recordStrings(recorder, 2212, strconv.Itoa(event.Branches))
	case checker.TemporalCheckStart:
		branches := ""
		if event.Branches != 1 {
			branches = strconv.Itoa(event.Branches) + " branches of "
		}
		recordStrings(recorder, 2192, "complete", strconv.Itoa(event.Nodes), branches)
	}
}

func recordInitialSearch(recorder *recorderConfig, event checker.SearchEvent) {
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
