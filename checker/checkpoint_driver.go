package checker

import (
	"context"
	"fmt"

	"github.com/upamune/tla-go/config"
)

// checkpointDriver uses expansion counts as deterministic ticks. PauseAt is an
// absolute expansion count and must coincide with a tick; zero never pauses.
// Observations belong to this invocation, not the snapshot. The caller owns
// their delivery and must retain them across a pause to preserve output order.
type checkpointDriver struct {
	path    string
	every   int
	pauseAt int
	paused  bool
}

func (s *search) checkpointBoundary() (bool, error) {
	d := s.model.checkpointDriver
	if d == nil {
		return false, nil
	}
	err := d.boundary(s)
	return err != nil || d.paused, err
}

func (d *checkpointDriver) boundary(s *search) error {
	if s.head%d.every != 0 {
		return nil
	}
	snapshot, err := s.checkpoint(*s.model.checkpointIdentity)
	if err != nil {
		return err
	}
	if err := checkpointWrite(d.path, snapshot); err != nil {
		return err
	}
	d.paused = s.head == d.pauseAt
	return nil
}

// runCheckpointSearch always prepares a fresh model. Resume reads and validates
// the snapshot before exploration; it never silently starts over on read error.
// Paused results are deliberately distinct from completed search results.
func runCheckpointSearch(ctx context.Context, source, configSource string, cfg config.Config, options Options, path string, every, pauseAt int, resume bool) (Result, []checkpointObservation, bool, error) {
	if !validCheckpointSchedule(path, every, pauseAt) {
		return Result{}, nil, false, fmt.Errorf("invalid checkpoint schedule")
	}
	observer := &checkpointObserver{}
	m, err := prepareCheckpointModelObserved(ctx, source, configSource, cfg, options, observer)
	if err != nil {
		return Result{}, nil, false, err
	}
	if err := m.checkpointSupported(); err != nil {
		return Result{}, nil, false, err
	}
	driver := &checkpointDriver{path: path, every: every, pauseAt: pauseAt}
	m.checkpointDriver = driver
	result, err := driver.run(m, resume)
	if err != nil || driver.paused {
		return result, observer.records, driver.paused, err
	}
	m.adjustContinuedEvaluationFailure(&result)
	result, err = m.finalizeCheckResult(result, nil, nil)
	return result, observer.records, false, err
}

func validCheckpointSchedule(path string, every, pauseAt int) bool {
	return path != "" && every > 0 && pauseAt >= 0 && (pauseAt == 0 || pauseAt%every == 0)
}

func (d *checkpointDriver) run(m *model, resume bool) (Result, error) {
	if !resume {
		return m.check()
	}
	snapshot, err := checkpointRead(d.path, *m.checkpointIdentity)
	if err != nil {
		return Result{}, err
	}
	if d.pauseAt != 0 && d.pauseAt <= snapshot.Head {
		return Result{}, fmt.Errorf("pause tick already passed")
	}
	s, err := m.restoreCheckpoint(snapshot, *m.checkpointIdentity)
	if err != nil {
		return Result{}, err
	}
	return s.explore()
}
