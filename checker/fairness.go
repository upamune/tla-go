package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
)

func (f *fairSearch) checkFairness(index int, fair fairnessCondition, component []int, members []bool) (bool, []bool, error) {
	if fair.recurrence {
		return f.checkRecurrence(index, fair, component, members)
	}
	return f.checkWeakStrongFairness(index, fair, component, members)
}

func (f *fairSearch) checkWeakStrongFairness(index int, fair fairnessCondition, component []int, members []bool) (bool, []bool, error) {
	enabled, err := f.enabledStates(index, fair, component)
	if err != nil {
		return false, nil, err
	}
	if fairnessSatisfiedByStates(fair.strong, enabled, len(component)) {
		return true, nil, nil
	}
	fires, err := f.fairEdgeExists(index, fair, component, members)
	if err != nil || fires {
		return fires, nil, err
	}
	if !fair.strong {
		return false, nil, nil
	}
	pruned := make([]bool, len(members))
	for _, node := range component {
		pruned[node] = f.enabled[index][node] == -1
	}
	return false, pruned, nil
}

func fairnessSatisfiedByStates(strong bool, enabled, total int) bool {
	if strong {
		return enabled == 0
	}
	return enabled < total
}

func (f *fairSearch) enabledStates(index int, fair fairnessCondition, component []int) (int, error) {
	count := 0
	for _, node := range component {
		if err := f.cacheEnabled(index, fair, node); err != nil {
			return 0, err
		}
		if f.enabled[index][node] == 1 {
			count++
		}
	}
	return count, nil
}

func (f *fairSearch) cacheEnabled(index int, fair fairnessCondition, node int) error {
	if f.enabled[index][node] != 0 {
		return nil
	}
	ok, cached := f.search.cachedFairEnabled(index, node)
	var err error
	if !cached {
		ok, err = f.search.model.enabledWithLocals(fairAngle(fair), f.search.nodes[node].state, nil, fair.locals)
	}
	if err != nil {
		return err
	}
	f.enabled[index][node] = -1
	if ok {
		f.enabled[index][node] = 1
	}
	return nil
}

func (f *fairSearch) fairEdgeExists(index int, fair fairnessCondition, component []int, members []bool) (bool, error) {
	s := f.search
	for _, node := range component {
		ok, err := s.fairEdgeFrom(index, fair, node, members)
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

func (s *search) fairEdgeFrom(index int, fair fairnessCondition, node int, members []bool) (bool, error) {
	for choice, edge := range s.edges[node] {
		if !members[edge.to] || edge.to == node && edge.next == nil {
			continue
		}
		ok, cached := s.cachedFairAction(index, node, choice)
		var err error
		if !cached {
			ok, err = s.model.holdsFair(fair, s.nodes[node].state, s.edgeNext(edge))
		}
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

func (m *model) holdsFair(fair fairnessCondition, current, next evaluator.State) (bool, error) {
	result, err := m.program.Evaluate(fairAngle(fair), evaluator.Context{Current: current, Next: next, Constants: m.constants, Locals: fair.locals, Enabled: m.enabledCallback})
	if err != nil {
		return false, err
	}
	ok, valid := result.Bool()
	if !valid {
		return false, fmt.Errorf("fairness action must be boolean, got %s", result)
	}
	return ok, nil
}
