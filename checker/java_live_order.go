package checker

// javaLiveOrder reconstructs the successor hash-slot order used by TLC's
// SetOfStates. Callers first ensure that fingerprinting cannot have effects.
// Work on a copy so liveness checking does not change live caches.
func (s *search) javaLiveOrder() ([][]int, bool) {
	copy := *s
	copy.ltlSetMultiplier = 1
	copy.ltlFingerprints = make(map[int]uint64, len(s.nodes))
	copy.ltlVariableOrder = s.model.symmetryVariableOrder()
	order := make([][]int, len(s.nodes))
	for node := range s.nodes {
		choices, err := copy.incrementalSuccessors(node)
		if err != nil {
			return nil, false
		}
		order[node] = choices
	}
	return order, true
}

func (s *search) stableFingerprintStates() bool {
	for _, node := range s.nodes {
		for _, v := range node.state {
			if !v.StableFingerprint() {
				return false
			}
		}
	}
	return true
}
