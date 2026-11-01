package checker

// liveDNFWithinLimit bounds the number of terms that toDNF can allocate. It
// deliberately overestimates when TLC merges recurrent disjunctions.
func liveDNFWithinLimit(formula *liveNode, limit int) bool {
	return liveDNFTermBound(formula, false, limit) <= limit
}

func liveDNFTermBound(formula *liveNode, negated bool, limit int) int {
	if formula.kind == liveNeg {
		return liveDNFTermBound(formula.body, !negated, limit)
	}
	if formula.kind != liveConj && formula.kind != liveDisj {
		return 1
	}
	if (formula.kind == liveConj) != negated {
		return liveDNFProduct(formula.items, negated, limit)
	}
	return liveDNFSum(formula.items, negated, limit)
}

func liveDNFProduct(items []*liveNode, negated bool, limit int) int {
	count := 1
	for _, child := range items {
		terms := liveDNFTermBound(child, negated, limit)
		if terms == 0 {
			return 0
		}
		if terms > limit || count > limit/terms {
			return limit + 1
		}
		count *= terms
	}
	return count
}

func liveDNFSum(items []*liveNode, negated bool, limit int) int {
	count := 0
	for _, child := range items {
		terms := liveDNFTermBound(child, negated, limit)
		if terms > limit || count > limit-terms {
			return limit + 1
		}
		count += terms
	}
	return count
}
