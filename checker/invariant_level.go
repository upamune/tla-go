package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func (m *model) validateInvariantLevels() error {
	for _, name := range m.cfg.Invariants {
		expr, err := m.definition(name)
		if err != nil {
			return m.configuredOperatorError("invariant", name, err)
		}
		if m.hasAction(expr, map[string]bool{}) {
			return m.invalidInvariantLevel(name, expr)
		}
	}
	return nil
}

func (m *model) invalidInvariantLevel(name string, expr *parser.Expression) error {
	message := fmt.Sprintf("The invariant %s is not a state predicate (one with no primes or temporal operators).", name)
	if m.invariantLevelParameters(expr, map[string]bool{}) {
		message += "\nNote that a bug can cause TLC to incorrectly report this error.\n" +
			"If you believe your TLA+ or PlusCal specification to be correct,\n" +
			"please check if this bug described in LevelNode.java starting at line 590ff affects you."
	}
	return &ModelValidationError{Message: message, Code: 2146, ExitCode: 76}
}
