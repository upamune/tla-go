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
	mu       sync.RWMutex
	snapshot ProgressSnapshot
}

// Snapshot returns a coherent copy at a publication point.
func (p *LiveProgress) Snapshot() ProgressSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snapshot
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
