package checker

import (
	"context"
	"fmt"
	"runtime/trace"
	"time"

	"github.com/upamune/tla-go/config"
)

// CheckpointOptions configures a supported pure serial breadth-first search.
// Interval must be positive for a new search; zero on recovery disables writes,
// including DebugPauseAfterExpansions and PauseOnCheckpoint. Checkpoints are written atomically at the first
// expansion boundary after each interval, not while evaluating a state.
// PauseOnCheckpoint returns after the first successful checkpoint write.
// Resume requires an existing matching checkpoint and never starts over.
// Options must not contain callbacks other than Loader, which is used only
// while resolving source before exploration. Unsupported models are rejected.
type CheckpointOptions struct {
	Path              string
	Interval          time.Duration
	Resume            bool
	PauseOnCheckpoint bool
	Options           Options
	// Config, if non-nil, is the effective configuration including overrides.
	// Configuration source is still required and participates in identity.
	Config *config.Config
	// DebugPauseAfterExpansions saves and pauses at an absolute expansion count.
	DebugPauseAfterExpansions int
	// Stream receives new observations synchronously in order. The caller must
	// drain it until this call returns. Previously persisted records are not replayed.
	// Delivery occurs before persistence and may be repeated after a crash.
	// Consumers must coordinate acknowledgments themselves; no exactly-once delivery
	// or equivalence with non-checkpointed output is guaranteed.
	Stream chan<- CheckpointObservation
}

// CheckpointObservation is one entry in the ordered output log. Exactly one
// field is set. Values can be streamed or returned after exploration, never through callbacks.
type CheckpointObservation struct {
	Search  *SearchEvent
	Warning *Warning
	Print   *string
}

// CheckpointRun contains a completed Result, or Paused when a checkpoint was
// saved. Observations are cumulative from the start of the search, including
// on recovery. A consumer that has delivered a prefix can skip that many
// entries after recovery; repeated identical warnings are distinct entries.
// Deliver only after a successful return: observations returned with an error
// may extend beyond the last durable checkpoint. The caller owns delivery and
// its acknowledgment, so atomic exactly-once external delivery is not implied.
type CheckpointRun struct {
	Result       Result
	Observations []CheckpointObservation
	Paused       bool
}

// CheckWithCheckpoint starts or recovers a checkpointable search. Each call
// prepares a fresh model and validates the exact source, dependencies, config,
// and semantic options against the checkpoint before resuming.
func CheckWithCheckpoint(ctx context.Context, source, configuration string, options CheckpointOptions) (CheckpointRun, error) {
	var out CheckpointRun
	if !validPublicCheckpointOptions(options) {
		return out, fmt.Errorf("invalid checkpoint schedule")
	}
	cfg, err := checkpointEffectiveConfig(configuration, options.Config)
	if err != nil {
		return out, err
	}
	observer := &checkpointObserver{}
	m, err := func() (*model, error) {
		if trace.IsEnabled() {
			defer trace.StartRegion(ctx, "model initialization").End()
		}
		return prepareCheckpointModelObserved(ctx, source, configuration, cfg, options.Options, observer)
	}()
	if err != nil {
		return out, err
	}
	defer searchHeapCapture(ctx, m)()
	if err = m.checkpointSupported(); err != nil {
		return out, &CheckpointUnsupportedError{Err: err}
	}
	// Fresh runs deliver preparation observations after the subset check.
	// Recovery starts delivery only after the checkpoint is fully restored.
	if !options.Resume {
		observer.startStream(options.Stream)
	}
	d := &checkpointDriver{path: options.Path, interval: options.Interval, next: time.Now().Add(options.Interval), pauseOnTick: options.PauseOnCheckpoint, persistObservations: true, pauseAt: options.DebugPauseAfterExpansions, stream: options.Stream}
	m.checkpointDriver = d
	defer traceExploration(ctx)()
	result, err := d.run(m, options.Resume)
	out.Observations = exportCheckpointObservations(observer.records)
	out.Paused = d.paused
	if err != nil || d.paused {
		return out, err
	}
	m.adjustContinuedEvaluationFailure(&result)
	out.Result, err = m.finalizeCheckResult(result, nil, nil)
	out.Observations = exportCheckpointObservations(observer.records)
	return out, err
}

func checkpointEffectiveConfig(source string, effective *config.Config) (config.Config, error) {
	cfg, err := config.Parse(source)
	if err != nil {
		return cfg, err
	}
	if effective != nil {
		cfg = *effective
	}
	return cfg, nil
}

func exportCheckpointObservations(records []checkpointObservation) []CheckpointObservation {
	var out []CheckpointObservation
	for _, r := range records {
		out = append(out, CheckpointObservation{Search: r.searchEvent, Warning: r.warning, Print: r.print})
	}
	return out
}

func importCheckpointObservations(records []CheckpointObservation) []checkpointObservation {
	var out []checkpointObservation
	for _, r := range records {
		out = append(out, checkpointObservation{searchEvent: r.Search, warning: r.Warning, print: r.Print})
	}
	return out
}

func validateCheckpointObservations(records []CheckpointObservation) error {
	for _, r := range records {
		count := 0
		if r.Search != nil {
			count++
		}
		if r.Warning != nil {
			count++
		}
		if r.Print != nil {
			count++
		}
		if count != 1 {
			return fmt.Errorf("invalid checkpoint observation")
		}
	}
	return nil
}

// CheckpointUnsupportedError identifies a model rejected before exploration.
type CheckpointUnsupportedError struct{ Err error }

func (e *CheckpointUnsupportedError) Error() string { return e.Err.Error() }
func (e *CheckpointUnsupportedError) Unwrap() error { return e.Err }

func validPublicCheckpointOptions(options CheckpointOptions) bool {
	return options.Path != "" && (options.Interval > 0 || (options.Resume && options.Interval == 0)) && options.DebugPauseAfterExpansions >= 0
}
