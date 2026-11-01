package checker

import (
	"context"
	"fmt"
	"time"

	"github.com/upamune/tla-go/config"
)

// CheckpointOptions configures a supported pure serial breadth-first search.
// Interval must be positive. Checkpoints are written atomically at the first
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
}

// CheckpointObservation is one entry in the ordered output log. Exactly one
// field is set. Values are returned after exploration, never through callbacks.
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
	if options.Path == "" || options.Interval <= 0 {
		return out, fmt.Errorf("invalid checkpoint schedule")
	}
	cfg, err := checkpointEffectiveConfig(configuration, options.Config)
	if err != nil {
		return out, err
	}
	observer := &checkpointObserver{}
	m, err := prepareCheckpointModelObserved(ctx, source, configuration, cfg, options.Options, observer)
	if err != nil {
		return out, err
	}
	if err = m.checkpointSupported(); err != nil {
		return out, err
	}
	d := &checkpointDriver{path: options.Path, interval: options.Interval, next: time.Now().Add(options.Interval), pauseOnTick: options.PauseOnCheckpoint, persistObservations: true}
	m.checkpointDriver = d
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
