package checker

import "github.com/upamune/tla-go/syntax/parser"

// [](P => []Q) is a safety property: after P holds, Q must keep holding.
func (m *model) classifyGuardedAlways(p temporalProperty, expr *parser.Expression) (temporalProperty, bool) {
	if expr.Kind != "Always" || len(expr.Args) != 1 {
		return p, false
	}
	return m.classifyGuardedImplication(p, expr.Args[0])
}

func (m *model) classifyGuardedImplication(p temporalProperty, implication *parser.Expression) (temporalProperty, bool) {
	if implication.Kind != "Apply" || implication.Text != "=>" || len(implication.Args) != 2 {
		return p, false
	}
	always := implication.Args[1]
	if always.Kind != "Always" || len(always.Args) != 1 {
		return p, false
	}
	p.trigger, p.predicate, p.mode = implication.Args[0], always.Args[0], guardedAlways
	return p, m.isStateExpression(p.trigger) && m.isStateExpression(p.predicate)
}

func (m *model) isStateExpression(expr *parser.Expression) bool {
	return !m.hasTemporal(expr, map[string]bool{}) && !m.hasAction(expr, map[string]bool{})
}
