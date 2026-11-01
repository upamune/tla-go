package checker

import "sync"

// ProgressSnapshot is a copy of the breadth-first search's existing counters.
// InitialComplete becomes true immediately before the InitialComplete event.
// Updates to different counters may be observed separately.
type ProgressSnapshot struct {
	GeneratedStates, DistinctStates, QueuedStates, Depth int
	InitialComplete                                      bool
}

// LiveProgress publishes snapshots for a single coverage check. Its zero value
// is ready to use. Snapshot may be called concurrently with the check and after
// it returns, including on error or cancellation. Use a fresh value per check;
// do not copy it after first use or share it between checks. The caller owns
// observer goroutines and must stop and join them before reusing any output.
// A zero snapshot can also mean the check failed before search began.
type LiveProgress struct {
	mu            sync.RWMutex
	snapshot      ProgressSnapshot
	coverage      *coverageCollector
	finished      bool
	finalCoverage []CoverageAction
}

// Snapshot returns a coherent copy at a publication point.
func (p *LiveProgress) Snapshot() ProgressSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snapshot
}

// CoverageSnapshot returns an independent, synchronized coverage report, or
// nil before coverage is initialized or when the completed check has no
// reliable coverage. Coverage and Snapshot are sampled separately and need not
// represent the same instant. After return, coverage matches Result.Coverage;
// a panic leaves no final coverage.
func (p *LiveProgress) CoverageSnapshot() []CoverageAction {
	p.mu.RLock()
	if p.finished {
		defer p.mu.RUnlock()
		return cloneCoverage(p.finalCoverage)
	}
	coverage := p.coverage
	p.mu.RUnlock()
	if coverage == nil {
		return nil
	}
	report := coverage.report()
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.finished {
		return cloneCoverage(p.finalCoverage)
	}
	return report
}

func cloneCoverage(actions []CoverageAction) []CoverageAction {
	if actions == nil {
		return nil
	}
	copyActions := make([]CoverageAction, len(actions))
	copy(copyActions, actions)
	for i := range copyActions {
		if actions[i].Expressions != nil {
			copyActions[i].Expressions = append([]CoverageExpression{}, actions[i].Expressions...)
		}
	}
	return copyActions
}

func (p *LiveProgress) finishCoverage(coverage []CoverageAction) {
	p.mu.Lock()
	p.finished = true
	p.coverage = nil
	p.finalCoverage = cloneCoverage(coverage)
	p.mu.Unlock()
}

func (p *LiveProgress) attachCoverage(coverage *coverageCollector) {
	p.mu.Lock()
	p.coverage = coverage
	p.mu.Unlock()
}

func (m *model) configureLiveProgress(progress *LiveProgress) {
	m.progress = progress
	if progress != nil {
		progress.attachCoverage(m.coverage)
	}
}

func (p *LiveProgress) publish(snapshot ProgressSnapshot) {
	p.mu.Lock()
	p.snapshot = snapshot
	p.mu.Unlock()
}

func (s *search) publishProgress() {
	if p := s.model.progress; p != nil {
		p.publish(ProgressSnapshot{
			GeneratedStates: s.result.GeneratedStates,
			DistinctStates:  s.result.DistinctStates,
			QueuedStates:    s.result.QueuedStates,
			Depth:           s.result.Depth,
			InitialComplete: s.initialComplete,
		})
	}
}
