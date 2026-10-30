package checker

import (
	"maps"
	"slices"

	"github.com/upamune/tla-go/evaluator"
)

type ltlBranch struct {
	todo      []*ltlFormula
	seen      map[int]bool
	next      map[int]*ltlFormula
	fulfilled map[int]bool
}

type ltlAlternative struct {
	next      []*ltlFormula
	accepting []bool
}

type ltlExpander struct {
	model       *model
	state       evaluator.State
	nextState   evaluator.State
	eventuality []*ltlFormula
	values      map[int]bool
	known       map[int]bool
	cache       map[ltlValueKey]bool
	base        int
	choice      int
}

func (e *ltlExpander) alternatives(formulas []*ltlFormula) ([]ltlAlternative, error) {
	branch := ltlBranch{todo: append([]*ltlFormula(nil), formulas...), seen: map[int]bool{}, next: map[int]*ltlFormula{}, fulfilled: map[int]bool{}}
	var result []ltlAlternative
	err := e.expand(branch, &result)
	return result, err
}

func (e *ltlExpander) expand(branch ltlBranch, result *[]ltlAlternative) error {
	if err := e.model.ctx.Err(); err != nil {
		return err
	}
	for len(branch.todo) > 0 {
		last := len(branch.todo) - 1
		formula := branch.todo[last]
		branch.todo = branch.todo[:last]
		if branch.seen[formula.id] {
			continue
		}
		branch.seen[formula.id] = true
		if handled, err := e.expandSimple(formula, &branch); handled || err != nil {
			if err != nil {
				return err
			}
			continue
		}
		return e.expandChoice(formula, branch, result)
	}
	*result = append(*result, e.makeAlternative(branch))
	return nil
}

func (e *ltlExpander) expandSimple(formula *ltlFormula, branch *ltlBranch) (bool, error) {
	switch formula.kind {
	case ltlTrue:
		return true, nil
	case ltlFalse:
		return false, nil
	case ltlAtom, ltlAction:
		valid, err := e.atomHolds(formula)
		if err != nil {
			return true, err
		}
		if !valid {
			branch.todo = nil
			return false, nil
		}
	case ltlAnd:
		branch.todo = append(branch.todo, formula.left, formula.right)
	case ltlAlways:
		branch.todo = append(branch.todo, formula.left)
		branch.next[formula.id] = formula
	default:
		return false, nil
	}
	return true, nil
}

func (e *ltlExpander) atomHolds(formula *ltlFormula) (bool, error) {
	if !e.known[formula.id] {
		if value, ok := e.cachedAtom(formula); ok {
			e.values[formula.id], e.known[formula.id] = value, true
			return value, nil
		}
		value, err := e.holdsAtom(formula)
		if err != nil {
			return false, err
		}
		e.values[formula.id] = value != formula.negative
		e.known[formula.id] = true
	}
	return e.values[formula.id], nil
}

func (e *ltlExpander) holdsAtom(formula *ltlFormula) (bool, error) {
	old := e.model.tlcRuntime.useBaseLevel
	if formula.fairEnabled || formula.baseLevel {
		e.model.tlcRuntime.useBaseLevel = true
	}
	defer func() { e.model.tlcRuntime.useBaseLevel = old }()
	return e.model.holdsLTLAtom(formula, e.state, e.nextState, formula.locals)
}

func (e *ltlExpander) cachedAtom(formula *ltlFormula) (bool, bool) {
	choice := e.choice
	if formula.kind == ltlAtom {
		choice = -1
	}
	value, ok := e.cache[ltlValueKey{formula: formula, base: e.base, choice: choice}]
	return value, ok
}

func (e *ltlExpander) expandChoice(formula *ltlFormula, branch ltlBranch, result *[]ltlAlternative) error {
	if formula.kind == ltlAtom || formula.kind == ltlAction || formula.kind == ltlFalse {
		return nil
	}
	first := cloneLTLBranch(branch)
	first.todo = append(first.todo, formula.left)
	if formula.kind == ltlEventually {
		first.fulfilled[formula.id] = true
		branch.next[formula.id] = formula
	} else {
		branch.todo = append(branch.todo, formula.right)
	}
	if err := e.expand(first, result); err != nil {
		return err
	}
	return e.expand(branch, result)
}

func cloneLTLBranch(branch ltlBranch) ltlBranch {
	return ltlBranch{todo: append([]*ltlFormula(nil), branch.todo...), seen: maps.Clone(branch.seen), next: maps.Clone(branch.next), fulfilled: maps.Clone(branch.fulfilled)}
}

func (e *ltlExpander) makeAlternative(branch ltlBranch) ltlAlternative {
	alternative := ltlAlternative{accepting: make([]bool, len(e.eventuality))}
	for _, formula := range branch.next {
		alternative.next = append(alternative.next, formula)
	}
	slices.SortFunc(alternative.next, func(a, b *ltlFormula) int { return a.id - b.id })
	for i, formula := range e.eventuality {
		alternative.accepting[i] = !branch.seen[formula.id] || branch.fulfilled[formula.id]
	}
	return alternative
}
