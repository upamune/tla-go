package checker

import "slices"

// TLC evaluates extracted recurrence predicates using the source state's level.
// Other tableau predicates retain the incoming transition's level.
func (c *ltlCompiler) markRecurrenceLevels(formula *ltlFormula) *ltlFormula {
	if isFairnessRecurrence(formula) {
		body := c.markRecurrenceBody(formula.left.left)
		inner := c.copyLTLFormula(formula.left, body, nil, false)
		return c.copyLTLFormula(formula, inner, nil, false)
	}
	if formula.kind == ltlAnd || formula.kind == ltlOr {
		left := c.markRecurrenceLevels(formula.left)
		right := c.markRecurrenceLevels(formula.right)
		if left != formula.left || right != formula.right {
			return c.copyLTLFormula(formula, left, right, false)
		}
	}
	return formula
}

func (c *ltlCompiler) markRecurrenceBody(formula *ltlFormula) *ltlFormula {
	if formula.kind == ltlAtom {
		return c.copyLTLFormula(formula, nil, nil, true)
	}
	if formula.kind == ltlAnd || formula.kind == ltlOr {
		left := c.markRecurrenceBody(formula.left)
		right := c.markRecurrenceBody(formula.right)
		if left != formula.left || right != formula.right {
			return c.copyLTLFormula(formula, left, right, false)
		}
	}
	return formula
}

func (c *ltlCompiler) copyLTLFormula(original, left, right *ltlFormula, baseLevel bool) *ltlFormula {
	c.nextID++
	copy := *original
	copy.id, copy.left, copy.right, copy.baseLevel = c.nextID, left, right, baseLevel
	if copy.kind == ltlEventually {
		c.eventuality = append(c.eventuality, &copy)
	}
	return &copy
}

func (c *ltlCompiler) retainEventualities(formula *ltlFormula) {
	reachable := map[int]bool{}
	markLTLReachable(formula, reachable)
	c.eventuality = slices.DeleteFunc(c.eventuality, func(eventuality *ltlFormula) bool {
		return !reachable[eventuality.id]
	})
}

func markLTLReachable(formula *ltlFormula, seen map[int]bool) {
	if formula == nil || seen[formula.id] {
		return
	}
	seen[formula.id] = true
	markLTLReachable(formula.left, seen)
	markLTLReachable(formula.right, seen)
}
