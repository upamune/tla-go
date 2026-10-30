package checker

import "github.com/upamune/tla-go/syntax/parser"

type expressionAnalysis uint8

const (
	actionAnalysis expressionAnalysis = iota
	temporalAnalysis
	stateAnalysis
)

func (m *model) matchesAnalysis(expr *parser.Expression, seen map[string]bool, analysis expressionAnalysis) bool {
	switch analysis {
	case actionAnalysis:
		return m.hasAction(expr, seen)
	case temporalAnalysis:
		return m.hasTemporal(expr, seen)
	case stateAnalysis:
		return m.readsState(expr, seen)
	default:
		panic("unknown expression analysis")
	}
}

// Walk the same children, in the same order, for the model's level analyses.
// LET definition bounds are intentionally excluded, matching their existing semantics.
func expressionChildrenMatch(m *model, expr *parser.Expression, seen map[string]bool, analysis expressionAnalysis) bool {
	return expressionListMatches(m, expr.Args, seen, analysis) || boundsMatch(m, expr.Bounds, seen, analysis) || fieldsMatch(m, expr.Fields, seen, analysis) || armsMatch(m, expr.Arms, seen, analysis) || definitionsMatch(m, expr.Definitions, seen, analysis) || updatesMatch(m, expr.Updates, seen, analysis)
}

func expressionListMatches(m *model, args []*parser.Expression, seen map[string]bool, analysis expressionAnalysis) bool {
	for _, arg := range args {
		if m.matchesAnalysis(arg, seen, analysis) {
			return true
		}
	}
	return false
}

func boundsMatch(m *model, bounds []parser.Bound, seen map[string]bool, analysis expressionAnalysis) bool {
	for _, bound := range bounds {
		if bound.Set != nil && m.matchesAnalysis(bound.Set, seen, analysis) {
			return true
		}
	}
	return false
}

func fieldsMatch(m *model, fields []parser.Field, seen map[string]bool, analysis expressionAnalysis) bool {
	for _, field := range fields {
		if m.matchesAnalysis(field.Value, seen, analysis) {
			return true
		}
	}
	return false
}

func armsMatch(m *model, arms []parser.CaseArm, seen map[string]bool, analysis expressionAnalysis) bool {
	for _, arm := range arms {
		if m.matchesAnalysis(arm.Condition, seen, analysis) || m.matchesAnalysis(arm.Body, seen, analysis) {
			return true
		}
	}
	return false
}

func definitionsMatch(m *model, definitions []parser.Definition, seen map[string]bool, analysis expressionAnalysis) bool {
	for _, definition := range definitions {
		if m.matchesAnalysis(definition.Body, seen, analysis) {
			return true
		}
	}
	return false
}

func updatesMatch(m *model, updates []parser.Update, seen map[string]bool, analysis expressionAnalysis) bool {
	for _, update := range updates {
		if m.matchesAnalysis(update.Value, seen, analysis) || pathMatches(m, update.Path, seen, analysis) {
			return true
		}
	}
	return false
}

func pathMatches(m *model, path []parser.PathStep, seen map[string]bool, analysis expressionAnalysis) bool {
	for _, step := range path {
		if expressionListMatches(m, step.Args, seen, analysis) {
			return true
		}
	}
	return false
}
