package parser

// SpecialOperatorKind identifies operators whose operands are evaluated in a
// different context or interpreted by the model checker.
func SpecialOperatorKind(name string) (string, bool) {
	switch name {
	case "'":
		return "Prime", true
	case "[]":
		return "Always", true
	case "<>":
		return "Eventually", true
	case "ENABLED":
		return "Enabled", true
	case "UNCHANGED":
		return "Unchanged", true
	default:
		return "", false
	}
}

// SpecialOperatorArity includes temporal operators that can be passed as arguments
// even when the receiving operator does not use them.
func SpecialOperatorArity(name string) (int, bool) {
	if _, ok := SpecialOperatorKind(name); ok {
		return 1, true
	}
	switch name {
	case "-+->", "~>", "\\cdot":
		return 2, true
	default:
		return 0, false
	}
}
