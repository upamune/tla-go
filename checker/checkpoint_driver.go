package checker

import (
	"context"
	"fmt"
	"time"

	"github.com/upamune/tla-go/config"
)

// checkpointDriver supports elapsed-time and deterministic expansion ticks.
// pauseAt is an absolute expansion count; zero disables deterministic pauses.
type checkpointDriver struct {
	path                string
	every               int
	pauseAt             int
	paused              bool
	interval            time.Duration
	next                time.Time
	pauseOnTick         bool
	persistObservations bool
	stream              chan<- CheckpointObservation
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
	if !d.boundaryDue(s.head) {
		return nil
	}
	snapshot, err := s.checkpoint(*s.model.checkpointIdentity)
	if err != nil {
		return err
	}
	if d.persistObservations {
		snapshot.ObservationLog = true
		snapshot.Observations = exportCheckpointObservations(s.model.checkpointObserver.records)
	}
	if err := checkpointWrite(d.path, snapshot); err != nil {
		return err
	}
	d.paused = d.pauseOnTick || (d.pauseAt != 0 && s.head == d.pauseAt)
	if d.interval > 0 {
		d.next = time.Now().Add(d.interval)
	}
	return nil
}

func (d *checkpointDriver) due(head int) bool {
	if d.interval > 0 {
		return !time.Now().Before(d.next)
	}
	return d.every > 0 && head%d.every == 0
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
	if d.enabled() && d.pauseAt != 0 && d.pauseAt <= snapshot.Head {
		return Result{}, fmt.Errorf("pause tick already passed")
	}
	if err := d.restoreObservations(m, snapshot); err != nil {
		return Result{}, err
	}
	s, err := m.restoreCheckpoint(snapshot, *m.checkpointIdentity)
	if err != nil {
		return Result{}, err
	}
	// Preparation records have already been emitted in the original run and
	// the restored log is cumulative. Only subsequent observations are new.
	m.checkpointObserver.stream = d.stream
	return s.explore()
}

func (d *checkpointDriver) restoreObservations(m *model, snapshot checkpointSnapshot) error {
	if !d.persistObservations {
		return nil
	}
	if !snapshot.ObservationLog {
		return fmt.Errorf("checkpoint has no observation log")
	}
	if err := validateCheckpointObservations(snapshot.Observations); err != nil {
		return err
	}
	m.checkpointObserver.records = importCheckpointObservations(snapshot.Observations)
	return nil
}

func (d *checkpointDriver) boundaryDue(head int) bool {
	return d.due(head) || (d.enabled() && d.pauseAt != 0 && head == d.pauseAt)
}

func (d *checkpointDriver) enabled() bool { return d.interval > 0 || d.every > 0 }
