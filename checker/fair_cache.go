package checker

func (s *search) hasCachedFairActions() bool {
	for _, check := range s.fairChecks {
		if check.action != nil {
			return true
		}
	}
	return false
}

func (s *search) remapFairActions(source *search, node, oldCount int, choices []int) {
	if !s.hasCachedFairActions() {
		return
	}
	for _, check := range s.fairChecks {
		for choice := range oldCount {
			delete(s.ltlValues, ltlValueKey{formula: check.action, base: node, choice: choice})
		}
	}
	for choice, original := range choices {
		copyFairActionValues(source, s, node, choice, node, original)
	}
}

func copyFairStateValues(source, target *search, node, base int) {
	for _, check := range target.fairChecks {
		copyFairValue(source, target, check.enabled, node, -1, base, -1)
	}
}

func copyFairActionValues(source, target *search, node, choice, base, baseChoice int) {
	for _, check := range target.fairChecks {
		copyFairValue(source, target, check.action, node, choice, base, baseChoice)
	}
}

func copyFairValue(source, target *search, formula *ltlFormula, node, choice, base, baseChoice int) {
	if formula == nil {
		return
	}
	if value, ok := source.ltlValues[ltlValueKey{formula: formula, base: base, choice: baseChoice}]; ok {
		if target.ltlValues == nil {
			target.ltlValues = map[ltlValueKey]bool{}
		}
		target.ltlValues[ltlValueKey{formula: formula, base: node, choice: choice}] = value
	}
}

func (s *search) cachedFairEnabled(index, node int) (bool, bool) {
	if index >= len(s.fairChecks) {
		return false, false
	}
	return s.cachedFairValue(s.fairChecks[index].enabled, node, -1)
}

func (s *search) cachedFairAction(index, node, choice int) (bool, bool) {
	if index >= len(s.fairChecks) {
		return false, false
	}
	return s.cachedFairValue(s.fairChecks[index].action, node, choice)
}

func (s *search) cachedFairValue(formula *ltlFormula, node, choice int) (bool, bool) {
	if formula == nil {
		return false, false
	}
	result, ok := s.ltlValues[ltlValueKey{formula: formula, base: node, choice: choice}]
	return result != formula.negative, ok
}
