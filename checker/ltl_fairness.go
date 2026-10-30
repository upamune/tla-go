package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

// Fairness is translated to its temporal definition, so it can occur inside
// arbitrary properties as well as in the specification's fairness assumptions.
func (c *ltlCompiler) compileFairness(expr *parser.Expression, negative bool) (*ltlFormula, error) {
	if len(expr.Args) != 2 || c.model.hasTemporal(expr.Args[1], map[string]bool{}) || c.model.hasAction(expr.Args[0], map[string]bool{}) {
		return nil, fmt.Errorf("invalid fairness action or subscript")
	}
	step := &parser.Expression{Kind: "AngleAction", Args: []*parser.Expression{expr.Args[1], expr.Args[0]}, Location: expr.Args[0].Location}
	enabled := &parser.Expression{Kind: "Enabled", Args: []*parser.Expression{step}, Location: expr.Args[0].Location}
	if expr.Kind == "WeakFairness" {
		return c.weakFairness(enabled, step, negative), nil
	}
	return c.strongFairness(enabled, step, negative), nil
}

func (c *ltlCompiler) weakFairness(enabled, step *parser.Expression, negative bool) *ltlFormula {
	state := c.intern(ltlAtom, nil, nil, enabled, !negative)
	state.fairEnabled = true
	action := c.intern(ltlAction, nil, nil, step, negative)
	action.fairness = true
	junction := ltlOr
	if negative {
		junction = ltlAnd
	}
	body := c.intern(junction, state, action, nil, false)
	return c.fairnessRecurrence(body, !negative)
}

func (c *ltlCompiler) strongFairness(enabled, step *parser.Expression, negative bool) *ltlFormula {
	state := c.intern(ltlAtom, nil, nil, enabled, !negative)
	state.fairEnabled = true
	action := c.intern(ltlAction, nil, nil, step, negative)
	action.fairness = true
	state = c.fairnessRecurrence(state, negative)
	action = c.fairnessRecurrence(action, !negative)
	junction := ltlOr
	if negative {
		junction = ltlAnd
	}
	return c.intern(junction, state, action, nil, false)
}

func (c *ltlCompiler) fairnessRecurrence(body *ltlFormula, often bool) *ltlFormula {
	inner, outer := ltlAlways, ltlEventually
	if often {
		inner, outer = ltlEventually, ltlAlways
	}
	body = c.intern(inner, body, nil, nil, false)
	return c.intern(outer, body, nil, nil, false)
}

// TLC can extract action checks only from recurrence conjuncts after the
// property has been negated and its outer boolean junctions distributed.
func validateFairnessActions(formula *ltlFormula) error {
	if !hasFairnessAction(formula) {
		return nil
	}
	if formula.kind == ltlAnd || formula.kind == ltlOr {
		if err := validateFairnessActions(formula.left); err != nil {
			return err
		}
		return validateFairnessActions(formula.right)
	}
	if isFairnessRecurrence(formula) {
		return nil
	}
	return fmt.Errorf("TLC cannot handle the temporal formula: actions may only appear as []<>A or <>[]A")
}

func hasFairnessAction(formula *ltlFormula) bool {
	return formula != nil && (formula.fairness || hasFairnessAction(formula.left) || hasFairnessAction(formula.right))
}

func isFairnessRecurrence(formula *ltlFormula) bool {
	if formula.left == nil || formula.kind != ltlAlways && formula.kind != ltlEventually {
		return false
	}
	inner := formula.left
	if inner.kind == formula.kind || inner.kind != ltlAlways && inner.kind != ltlEventually {
		return false
	}
	return !hasTemporalFormula(inner.left)
}

func hasTemporalFormula(formula *ltlFormula) bool {
	return formula != nil && (formula.kind == ltlAlways || formula.kind == ltlEventually || hasTemporalFormula(formula.left) || hasTemporalFormula(formula.right))
}
