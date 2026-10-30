package checker

import (
	"errors"
	"fmt"

	"github.com/upamune/tla-go/evaluator"
)

type ltlLasso struct {
	search    *search
	violation *Violation
	values    map[int][]bool
}

func (s *search) lassoViolatesGeneral(property temporalProperty, violation *Violation) (bool, error) {
	model := ltlLasso{search: s, violation: violation, values: map[int][]bool{}}
	values, err := model.evaluate(property.general)
	if err != nil {
		return false, err
	}
	return values[0], nil
}

func (l *ltlLasso) evaluate(formula *ltlFormula) ([]bool, error) {
	if values, ok := l.values[formula.id]; ok {
		return values, nil
	}
	var values []bool
	var err error
	switch formula.kind {
	case ltlTrue, ltlFalse:
		values = l.constant(formula.kind == ltlTrue)
	case ltlAtom, ltlAction:
		values, err = l.atom(formula)
	case ltlAnd, ltlOr:
		values, err = l.junction(formula)
	case ltlAlways, ltlEventually:
		values, err = l.temporal(formula)
	default:
		return nil, fmt.Errorf("invalid temporal formula")
	}
	if err == nil {
		l.values[formula.id] = values
	}
	return values, err
}

func (l *ltlLasso) constant(value bool) []bool {
	result := make([]bool, len(l.violation.Trace))
	if value {
		for i := range result {
			result[i] = true
		}
	}
	return result
}

func (l *ltlLasso) atom(formula *ltlFormula) ([]bool, error) {
	result := make([]bool, len(l.violation.Trace))
	for i, step := range l.violation.Trace {
		next := l.violation.Trace[*l.violation.LoopStart].State
		if i+1 < len(l.violation.Trace) {
			next = l.violation.Trace[i+1].State
		}
		value, err := l.search.model.holdsLTLAtom(formula, step.State, next, formula.locals)
		if err != nil {
			return nil, lassoAtomError(formula, err)
		}
		result[i] = value != formula.negative
	}
	return result, nil
}

func lassoAtomError(formula *ltlFormula, err error) error {
	if missing, ok := errors.AsType[*evaluator.UnboundVariable](err); ok {
		location := formula.atom.Location
		return fmt.Errorf("The variable %s is not assigned at line %d, column %d", missing.Name, location.Line, location.Column)
	}
	return err
}

func (l *ltlLasso) junction(formula *ltlFormula) ([]bool, error) {
	left, err := l.evaluate(formula.left)
	if err != nil {
		return nil, err
	}
	right, err := l.evaluate(formula.right)
	if err != nil {
		return nil, err
	}
	result := make([]bool, len(left))
	for i := range result {
		if formula.kind == ltlAnd {
			result[i] = left[i] && right[i]
		} else {
			result[i] = left[i] || right[i]
		}
	}
	return result, nil
}

func (l *ltlLasso) temporal(formula *ltlFormula) ([]bool, error) {
	body, err := l.evaluate(formula.left)
	if err != nil {
		return nil, err
	}
	result := make([]bool, len(body))
	if formula.kind == ltlAlways {
		for i := range result {
			result[i] = true
		}
	}
	for l.updateTemporal(formula.kind, body, result) {
	}
	return result, nil
}

func (l *ltlLasso) updateTemporal(kind ltlKind, body, result []bool) bool {
	changed := false
	for i := len(result) - 1; i >= 0; i-- {
		next := i + 1
		if next == len(result) {
			next = *l.violation.LoopStart
		}
		value := body[i] || result[next]
		if kind == ltlAlways {
			value = body[i] && result[next]
		}
		changed = changed || value != result[i]
		result[i] = value
	}
	return changed
}
