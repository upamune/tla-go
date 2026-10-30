package checker

import (
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

// Random reseeds the worker from each expanded state's TLC fingerprint.
func (r *tlcRuntime) Random() (*evaluator.JavaRandom, error) {
	if r.randomStartup || r.search == nil || r.traceCurrent < 0 {
		return r.startupRandom, nil
	}
	if r.randomIndex == r.traceCurrent {
		return r.workerRandom, nil
	}
	if r.randomOrder == nil {
		r.randomOrder = r.model.symmetryVariableOrder()
	}
	state := r.search.nodes[r.traceCurrent].state
	values := make([]value.Value, len(r.randomOrder))
	for i, name := range r.randomOrder {
		values[i] = state[name]
	}
	rank := func(text string) (int, bool) { n, ok := r.model.valueRanks[text]; return n, ok }
	fingerprint, err := value.TLCFingerprintWithOrdering(values, rank, r.model.compareOrderedValues)
	if err != nil {
		return nil, err
	}
	r.workerRandom.SetSeed(int64(fingerprint ^ uint64(r.options.Seed)))
	r.randomIndex = r.traceCurrent
	return r.workerRandom, nil
}

// EvaluateAtStartup keeps constant-level random definitions on the startup stream.
func (r *tlcRuntime) EvaluateAtStartup(evaluate func() (value.Value, error)) (value.Value, error) {
	previous := r.randomStartup
	r.randomStartup = true
	defer func() { r.randomStartup = previous }()
	return evaluate()
}
