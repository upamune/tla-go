package checker

import (
	"maps"
	"math/bits"
	"slices"

	"github.com/upamune/tla-go/evaluator"
)

type ltlBranch struct {
	todo      []*ltlFormula
	seen      ltlIDSet
	// A formula is expanded once per branch, so next contains no repeated IDs.
	next      []*ltlFormula
	fulfilled ltlIDSet
}

// Most temporal formulas have fewer than 64 nodes. Keep their branch-local
// membership inline so a choice doesn't allocate two copied maps. IDs beyond
// that range retain the same semantics through the overflow map.
type ltlIDSet struct {
	small uint64
	large map[int]bool
}

func (s ltlIDSet) has(id int) bool {
	if id >= 0 && id < 64 {
		return s.small&(uint64(1)<<uint(id)) != 0
	}
	return s.large[id]
}

func (s *ltlIDSet) add(id int) {
	if id >= 0 && id < 64 {
		s.small |= uint64(1) << uint(id)
		return
	}
	if s.large == nil {
		s.large = make(map[int]bool)
	}
	s.large[id] = true
}

func (s ltlIDSet) clone() ltlIDSet {
	s.large = maps.Clone(s.large)
	return s
}

type ltlAlternative struct {
	next      []*ltlFormula
	accepting []bool
}

type ltlExpander struct {
	model             *model
	state             evaluator.State
	nextState         evaluator.State
	eventuality       []*ltlFormula
	values            map[int]bool
	known             map[int]bool
	cache             map[ltlValueKey]bool
	base              int
	choice            int
	tableau           bool
	tableauAtoms      map[int]*ltlFormula
	tableauConditions [][]*ltlFormula
}

func (e *ltlExpander) alternatives(formulas []*ltlFormula) ([]ltlAlternative, error) {
	branch := ltlBranch{todo: append([]*ltlFormula(nil), formulas...)}
	if e.tableau {
		if e.tableauAtoms == nil {
			e.tableauAtoms = map[int]*ltlFormula{}
		}
		e.tableauConditions = nil
	}
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
		if branch.seen.has(formula.id) {
			continue
		}
		branch.seen.add(formula.id)
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
		return e.expandAtom(formula, branch)
	case ltlAnd:
		branch.todo = append(branch.todo, formula.left, formula.right)
	case ltlAlways:
		branch.todo = append(branch.todo, formula.left)
		branch.next = append(branch.next, formula)
	default:
		return false, nil
	}
	return true, nil
}

func (e *ltlExpander) expandAtom(formula *ltlFormula, branch *ltlBranch) (bool, error) {
	if e.tableau {
		e.tableauAtoms[formula.id] = formula
		return true, nil
	}
	valid, err := e.atomHolds(formula)
	if err != nil {
		return true, err
	}
	if !valid {
		branch.todo = nil
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
		first.fulfilled.add(formula.id)
		branch.next = append(branch.next, formula)
	} else {
		branch.todo = append(branch.todo, formula.right)
	}
	if err := e.expand(first, result); err != nil {
		return err
	}
	return e.expand(branch, result)
}

func cloneLTLBranch(branch ltlBranch) ltlBranch {
	return ltlBranch{todo: append([]*ltlFormula(nil), branch.todo...), seen: branch.seen.clone(), next: append([]*ltlFormula(nil), branch.next...), fulfilled: branch.fulfilled.clone()}
}

func (e *ltlExpander) makeAlternative(branch ltlBranch) ltlAlternative {
	alternative := ltlAlternative{accepting: make([]bool, len(e.eventuality))}
	alternative.next = branch.next
	if e.tableau {
		e.tableauConditions = append(e.tableauConditions, e.conditionsFor(branch.seen))
	}
	slices.SortFunc(alternative.next, func(a, b *ltlFormula) int { return a.id - b.id })
	for i, formula := range e.eventuality {
		alternative.accepting[i] = !branch.seen.has(formula.id) || branch.fulfilled.has(formula.id)
	}
	return alternative
}

func (e *ltlExpander) conditionsFor(seen ltlIDSet) []*ltlFormula {
	var conditions []*ltlFormula
	for remaining := seen.small; remaining != 0; remaining &= remaining - 1 {
		conditions = e.appendCondition(conditions, bits.TrailingZeros64(remaining))
	}
	for id := range seen.large {
		conditions = e.appendCondition(conditions, id)
	}
	slices.SortFunc(conditions, func(a, b *ltlFormula) int { return a.id - b.id })
	return conditions
}

func (e *ltlExpander) appendCondition(conditions []*ltlFormula, id int) []*ltlFormula {
	if formula := e.tableauAtoms[id]; formula != nil {
		return append(conditions, formula)
	}
	return conditions
}
