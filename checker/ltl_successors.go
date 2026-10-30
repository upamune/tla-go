package checker

import "github.com/upamune/tla-go/value"

// liveSuccessorSet follows TLC's SetOfStates insertion and slot iteration.
// It retains distinct concrete successors even when VIEW merges their IDs.
type liveSuccessorSet struct {
	search *search
	index  int
	slots  []int
	count  int
}

func (s *search) incrementalSuccessors(index int) ([]int, error) {
	set := liveSuccessorSet{search: s, index: index, slots: emptyLiveSlots(16 * s.ltlSetMultiplier)}
	if err := set.addEdges(); err != nil {
		return nil, err
	}
	if len(set.slots) > 16*s.ltlSetMultiplier {
		s.ltlSetMultiplier++
	}
	return set.choices(), nil
}

func (set *liveSuccessorSet) addEdges() error {
	s, index := set.search, set.index
	stutter := 0
	if s.model.hasReduction() {
		stutter = len(s.edges[index]) - 1
	}
	for choice := range s.edges[index] {
		if choice != stutter {
			if err := set.put(choice); err != nil {
				return err
			}
		}
	}
	return set.put(stutter)
}

func (set *liveSuccessorSet) choices() []int {
	result := make([]int, 0, set.count)
	for _, choice := range set.slots {
		if choice >= 0 {
			result = append(result, choice)
		}
	}
	return result
}

func emptyLiveSlots(size int) []int {
	slots := make([]int, size)
	for i := range slots {
		slots[i] = -1
	}
	return slots
}

func (set *liveSuccessorSet) put(choice int) error {
	if set.count >= len(set.slots)>>1 {
		if err := set.grow(); err != nil {
			return err
		}
	}
	added, err := set.insert(choice)
	if added {
		set.count++
	}
	return err
}

func (set *liveSuccessorSet) grow() error {
	old := set.slots
	set.slots = emptyLiveSlots(2*len(old) + 1)
	for _, choice := range old {
		if choice >= 0 {
			if _, err := set.insert(choice); err != nil {
				return err
			}
		}
	}
	return nil
}

func (set *liveSuccessorSet) insert(choice int) (bool, error) {
	edge := set.search.edges[set.index][choice]
	fingerprint, err := set.fingerprint(edge.to)
	if err != nil {
		return false, err
	}
	for slot := int(fingerprint&0x7fffffff) % len(set.slots); ; slot = (slot + 1) % len(set.slots) {
		other := set.slots[slot]
		if other < 0 {
			set.slots[slot] = choice
			return true, nil
		}
		duplicate, err := set.sameSuccessor(choice, other)
		if err != nil || duplicate {
			return false, err
		}
	}
}

func (set *liveSuccessorSet) sameSuccessor(a, b int) (bool, error) {
	edges := set.search.edges[set.index]
	if edges[a].to != edges[b].to {
		return false, nil
	}
	return set.search.sameConcreteState(set.search.edgeNext(edges[a]), set.search.edgeNext(edges[b]))
}

func (set *liveSuccessorSet) fingerprint(index int) (uint64, error) {
	if fingerprint, ok := set.search.ltlFingerprints[index]; ok {
		return fingerprint, nil
	}
	m := set.search.model
	values, err := set.fingerprintValues(index)
	if err != nil {
		return 0, err
	}
	rank := func(text string) (int, bool) { n, ok := m.valueRanks[text]; return n, ok }
	fingerprint, err := m.fingerprintAlgorithm.Fingerprint(values, rank, m.compareOrderedValues)
	if err == nil {
		set.search.ltlFingerprints[index] = fingerprint
	}
	return fingerprint, err
}

func (set *liveSuccessorSet) fingerprintValues(index int) ([]value.Value, error) {
	m := set.search.model
	state := set.search.nodes[index].state
	if m.cfg.Symmetry != "" {
		var err error
		state, err = m.symmetryRepresentative(state)
		if err != nil {
			return nil, err
		}
	}
	order := set.search.ltlVariableOrder
	values := make([]value.Value, len(order))
	for i, name := range order {
		values[i] = state[name]
	}
	if m.cfg.View != "" {
		expr, err := m.definition(m.cfg.View)
		if err != nil {
			return nil, err
		}
		v, err := m.eval(expr, state, nil)
		if err != nil {
			return nil, err
		}
		values = []value.Value{v}
	}
	return values, nil
}
