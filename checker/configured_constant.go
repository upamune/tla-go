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

func (m *model) validateConfiguredStateOperators() error {
	for _, item := range []struct{ kind, name string }{{"initial predicate", m.cfg.Init}, {"next state action", m.cfg.Next}} {
		if item.name == "" {
			continue
		}
		if _, err := m.definition(item.name); err != nil {
			return m.configuredOperatorError(item.kind, item.name, err)
		}
	}
	return nil
}

func (m *model) configuredOperatorError(kind, name string, cause error) error {
	if definition, ok := m.defs[name]; ok && len(definition.Params) > 0 {
		if kind == "property" {
			return &ConfigurationError{Message: fmt.Sprintf("TLC requires %s not to take any argument.", name), Code: 2228, Err: cause}
		}
		return &ConfigurationError{Message: fmt.Sprintf("TLC requires %s not to take any argument, but one was given: %s", kind, name), Code: 2228, Err: cause}
	}
	return &ConfigurationError{Message: fmt.Sprintf("The %s %s specified in the configuration file\nis not defined in the specification.", kind, name), Code: 2229, Err: cause}
}

func (m *model) validateConfiguredOperators(kind string, names []string) error {
	for _, name := range names {
		if _, err := m.definition(name); err != nil {
			return m.configuredOperatorError(kind, name, err)
		}
	}
	return nil
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
