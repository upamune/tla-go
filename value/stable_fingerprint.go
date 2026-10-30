package value

// StableFingerprint reports whether fingerprinting can read this value without
// invoking a deferred TLA+ expression or materializing a symbolic set.
func (v Value) StableFingerprint() bool {
	switch v.kind {
	case IntegerKind, BooleanKind, StringKind, ModelKind:
		return true
	case SetKind:
		return stableFingerprintValues(v.setIteration())
	case FunctionKind:
		return stableFingerprintPairs(v.data.pairs)
	default:
		return false
	}
}

func stableFingerprintValues(values []Value) bool {
	for _, item := range values {
		if !item.StableFingerprint() {
			return false
		}
	}
	return true
}

func stableFingerprintPairs(pairs []Pair) bool {
	for _, pair := range pairs {
		if !pair.Key.StableFingerprint() || !pair.Value.StableFingerprint() {
			return false
		}
	}
	return true
}
