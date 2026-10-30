package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

// SANY rejects Boolean connectives combining an action and a temporal formula.
func (m *model) temporalBooleanLevel(expr *parser.Expression) error {
	name := temporalBooleanName(expr)
	if name == "" {
		return nil
	}
	var action, temporal bool
	for _, arg := range expr.Args {
		isTemporal := m.hasTemporal(arg, map[string]bool{})
		temporal = temporal || isTemporal
		if !isTemporal && !action {
			action = m.hasAction(arg, map[string]bool{})
		}
	}
	if action && temporal {
		return fmt.Errorf("%s has both temporal formula and action as arguments", name)
	}
	return nil
}

func temporalBooleanName(expr *parser.Expression) string {
	switch expr.Kind {
	case "And":
		return `\land`
	case "Or":
		return `\lor`
	case "Apply":
		switch expr.Text {
		case "=>":
			return "=>"
		case "<=>":
			return `\equiv`
		}
	}
	return ""
}
