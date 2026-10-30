package checker

import "github.com/upamune/tla-go/syntax/parser"

func (p *specificationParts) collectAlways(expr *parser.Expression) bool {
	if isAlwaysBox(expr) {
		return p.nextAction(expr)
	}
	if isOftenAction(expr) {
		p.addRecurrence(expr.Args[0].Args[0], true)
		return true
	}
	predicate, mode := temporalPredicate(expr)
	if predicate == nil || mode != often {
		return false
	}
	p.addRecurrence(predicate, false)
	return true
}

func (p *specificationParts) addRecurrence(expr *parser.Expression, action bool) {
	p.fairness = append(p.fairness, fairnessCondition{
		recurrence: true, recurrenceAction: action, action: expr,
		bounds: append([]parser.Bound(nil), p.bounds...),
	})
}
