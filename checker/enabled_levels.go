package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func (m *model) validateEnabledLevels() error {
	for _, def := range m.module.Definitions {
		if err := m.checkEnabledLevels(def.Body); err != nil {
			return err
		}
	}
	for _, assumption := range m.module.Assumptions {
		if err := m.checkEnabledLevels(assumption.Expression); err != nil {
			return err
		}
	}
	return nil
}

func (m *model) checkEnabledLevels(expr *parser.Expression) error {
	if expr.Kind == "Enabled" && m.hasTemporal(expr.Args[0], map[string]bool{}) {
		return fmt.Errorf("ENABLED cannot contain a temporal expression")
	}
	var failure error
	walkChildren(expr, func(child *parser.Expression) {
		if failure == nil {
			failure = m.checkEnabledLevels(child)
		}
	})
	return failure
}
