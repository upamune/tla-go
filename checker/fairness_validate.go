package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func (m *model) prepareFairness() error {
	if err := m.validateFairness(); err != nil {
		return err
	}
	return m.expandFairness()
}

func (m *model) validateFairness() error {
	for _, fair := range m.fairness {
		if err := m.validateFairnessCondition(fair); err != nil {
			return err
		}
	}
	return nil
}

func (m *model) validateFairnessCondition(fair fairnessCondition) error {
	if !fair.recurrence && !m.validFairnessSubscript(fair.subscript) {
		return fmt.Errorf("fairness subscript must be a state expression")
	}
	if err := m.validateFairnessExpression(fair); err != nil {
		return err
	}
	for _, bound := range fair.bounds {
		if bound.Set == nil || m.hasAction(bound.Set, map[string]bool{}) || m.readsState(bound.Set, map[string]bool{}) {
			return fmt.Errorf("fairness quantifier domain must be a constant set")
		}
	}
	return nil
}

func (m *model) validateFairnessExpression(fair fairnessCondition) error {
	if m.hasTemporal(fair.action, map[string]bool{}) {
		return fmt.Errorf("fairness action is temporal")
	}
	if fair.recurrence && !fair.recurrenceAction && m.hasAction(fair.action, map[string]bool{}) {
		return fmt.Errorf("recurrence assumption must be a state predicate")
	}
	if fair.recurrenceAction && m.hasAction(fair.action.Args[1], map[string]bool{}) {
		return fmt.Errorf("recurrence action subscript must be a state expression")
	}
	return nil
}

func (m *model) validFairnessSubscript(expr *parser.Expression) bool {
	return !m.hasTemporal(expr, map[string]bool{}) && !m.hasAction(expr, map[string]bool{})
}
