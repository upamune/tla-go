package checker

import "github.com/upamune/tla-go/syntax/parser"

func (m *model) prepareRandomConstants() {
	usesRandom := m.moduleUsesRandom()
	definitions := make(map[string]bool)
	expressions := make(map[*parser.Expression]bool)
	excluded := m.configSubstitutionCacheExclusions()
	for name, def := range m.defs {
		if def.Kind != "Operator" || len(def.Params) != 0 {
			continue
		}
		if m.prepareCacheCandidate(def.Body, expressions, usesRandom, excluded[name]) {
			definitions[name] = true
		}
	}
	m.program.SetConstantDefinitions(definitions)
	m.program.SetConstantTLCEvaluations(expressions)
}

func (m *model) prepareCacheCandidate(body *parser.Expression, expressions map[*parser.Expression]bool, usesRandom, excluded bool) bool {
	if excluded {
		if usesRandom {
			m.collectConstantTLCEval(body, expressions)
		}
		return false
	}
	return m.prepareEffectfulConstant(body, expressions, usesRandom) || m.staticConstantExpression(body, nil)
}

func (m *model) prepareEffectfulConstant(body *parser.Expression, expressions map[*parser.Expression]bool, usesRandom bool) bool {
	if usesRandom {
		m.collectConstantTLCEval(body, expressions)
	}
	return m.cacheableEffectfulDefinition(body)
}

func (m *model) moduleUsesRandom() bool {
	for _, def := range m.defs {
		if m.readsRandom(def.Body, map[string]bool{}) {
			return true
		}
	}
	return false
}

func (m *model) collectConstantTLCEval(expr *parser.Expression, result map[*parser.Expression]bool) {
	if expr == nil {
		return
	}
	if m.isConstantTLCEval(expr) {
		result[expr] = true
		return
	}
	if expr.Kind == "Let" || expr.Kind == "Lambda" || len(expr.Bounds) > 0 {
		return
	}
	walkChildren(expr, func(child *parser.Expression) { m.collectConstantTLCEval(child, result) })
}

func (m *model) isConstantTLCEval(expr *parser.Expression) bool {
	return expr.Kind == "Apply" && expr.Text == "TLCEval" && !m.readsState(expr, map[string]bool{}) && !m.hasAction(expr, map[string]bool{})
}

func (m *model) readsRandom(expr *parser.Expression, seen map[string]bool) bool {
	if expr == nil {
		return false
	}
	if expr.Kind == "Apply" && isRandomName(tlcOperatorName(expr)) {
		return true
	}
	if m.randomDefinition(expr, seen) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = m.readsRandom(child, seen)
		}
	})
	return found
}

func isRandomName(name string) bool {
	return name == "RandomElement" || name == "RandomSubset" || name == "RandomSetOfSubsets" || name == "TestRandomSetOfSubsets"
}

func (m *model) randomDefinition(expr *parser.Expression, seen map[string]bool) bool {
	if expr.Kind != "Identifier" && expr.Kind != "Apply" {
		return false
	}
	def, ok := m.defs[expr.Text]
	if !ok || seen[expr.Text] {
		return false
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return m.readsRandom(def.Body, seen)
}
