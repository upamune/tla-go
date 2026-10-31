package checker

import "slices"

// LiveCheck1 checks the disabled branch of strong fairness before the branch
// in which the action occurs. Prefer a valid disabled subcomponent when both
// branches have counterexamples.
func (f *fairSearch) preferSimulationDisabledFairness(component []int) (bool, error) {
	for index, fair := range f.search.model.fairness {
		if !fair.strong || fair.recurrence {
			continue
		}
		found, err := f.preferSimulationDisabledComponent(index, fair, component)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

func (f *fairSearch) preferSimulationDisabledComponent(index int, fair fairnessCondition, component []int) (bool, error) {
	enabled, err := f.enabledStates(index, fair, component)
	if err != nil || enabled == 0 || enabled == len(component) {
		return false, err
	}
	allowed := make([]bool, len(f.search.nodes))
	for _, node := range component {
		allowed[node] = f.enabled[index][node] == -1
	}
	candidate := *f
	candidate.recurrenceWitnesses = slices.Clone(f.recurrenceWitnesses)
	candidate.recurrenceKnown = slices.Clone(f.recurrenceKnown)
	return candidate.findFairCycle(allowed)
}
