package checker

import "fmt"

func (m *model) validateConfiguredConstants() error {
	for _, item := range []struct{ kind, name string }{{"initial predicate", m.cfg.Init}, {"next state action", m.cfg.Next}} {
		if _, exists := m.constants[item.name]; exists {
			return &ConfigurationError{Message: fmt.Sprintf("The %s %s cannot be a constant.", item.kind, item.name), Code: 2233}
		}
	}
	if err := m.validateConstantChecks("invariant", m.cfg.Invariants); err != nil {
		return err
	}
	return m.validateConstantChecks("property", m.cfg.Properties)
}

func (m *model) validateConstantChecks(kind string, names []string) error {
	for _, name := range names {
		v, exists := m.constants[name]
		if !exists {
			continue
		}
		b, valid := v.Bool()
		if !valid || !b {
			return &ConfigurationError{Message: fmt.Sprintf("The %s of %s is equal to %s", kind, name, v), Code: 2230}
		}
	}
	return nil
}

func (m *model) safetyViolationName(property temporalProperty) string {
	if property.predicate.Kind == "Identifier" && len(property.locals) == 0 {
		if _, exists := m.defs[property.predicate.Text]; exists {
			return property.predicate.Text
		}
	}
	return property.name
}
