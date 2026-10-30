package checker

import (
	"fmt"
	"strings"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

type possiblePredicate struct {
	name       string
	expression *parser.Expression
}

func (m *model) preparePossible() error {
	seen := map[string]bool{}
	for _, name := range m.cfg.Possible {
		if seen[name] {
			return fmt.Errorf("_POSSIBLE lists %s more than once", name)
		}
		seen[name] = true
		if err := m.addPossible(name); err != nil {
			return err
		}
	}
	return nil
}

func (m *model) addPossible(name string) error {
	definition, ok := m.defs[name]
	if !ok || definition.Kind != "Operator" {
		return fmt.Errorf("The possible %s specified in the configuration file\nis not defined in the specification.", name)
	}
	if len(definition.Params) != 0 {
		return fmt.Errorf("TLC requires possible not to take any argument, but one was given: %s", name)
	}
	expr := definition.Body
	if m.hasTemporal(expr, map[string]bool{}) {
		return fmt.Errorf("The possible of %s is equal to a temporal formula; only state- and action-level predicates are supported", name)
	}
	if !m.readsState(expr, map[string]bool{}) && !m.hasAction(expr, map[string]bool{}) {
		return fmt.Errorf("The possible %s cannot be a constant.", name)
	}
	p := possiblePredicate{name: name, expression: expr}
	if m.hasAction(expr, map[string]bool{}) {
		m.possibleActions = append(m.possibleActions, p)
	} else {
		m.possibleStates = append(m.possibleStates, p)
	}
	return nil
}

func (m *model) trackPossible(predicates []possiblePredicate, current, next evaluator.State) error {
	for _, p := range predicates {
		holds, err := m.holds(p.expression, current, next)
		if err != nil {
			return err
		}
		if m.tlcRuntime.possibleCounts == nil {
			m.tlcRuntime.possibleCounts = make(map[string]int)
		}
		count := m.tlcRuntime.possibleCounts[p.name]
		if holds {
			count++
		}
		m.tlcRuntime.possibleCounts[p.name] = count
	}
	return nil
}

func (m *model) unwitnessedPossible() *PossibleViolation {
	for _, name := range m.cfg.Possible {
		if m.tlcRuntime.possibleCounts[name] == 0 {
			return m.possibleFailure(name)
		}
	}
	return nil
}

func (m *model) possibleFailure(name string) *PossibleViolation {
	d := m.defs[name]
	r := d.BodyRange
	location := fmt.Sprintf(" at line %d, col %d to line %d, col %d of module %s", r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
	message := fmt.Sprintf("The _POSSIBLE predicate %s%s was never witnessed.\nOthers may be unwitnessed too; only witnessed predicates appear with a\nnon-zero count in the record printed above.\nSee https://explain.tlapl.us/possible-conditions for additional details.", name, location)
	return &PossibleViolation{Name: name, Message: message, Counts: m.possibleCountsText()}
}

func (m *model) possibleCountsText() string {
	if len(m.tlcRuntime.possibleCounts) == 0 {
		return "<<>>"
	}
	parts := make([]string, 0, len(m.tlcRuntime.possibleCounts))
	for _, d := range m.module.Definitions {
		if count, ok := m.tlcRuntime.possibleCounts[d.Name]; ok {
			parts = append(parts, fmt.Sprintf("%s |-> %d", d.Name, count))
		}
	}
	return value.PrettyPrint("[" + strings.Join(parts, ", ") + "]")
}

func possibleResult(result Result, failure *PossibleViolation) Result {
	if result.Status != Success {
		if result.Violation != nil {
			result.Violation.Unwitnessed = failure
		}
		return result
	}
	result.Status = PostconditionViolation
	result.Violation = &Violation{Name: failure.Name, Message: failure.Message, Counts: failure.Counts, Trace: []TraceStep{}}
	return result
}
