package checker

// TLC validates invariants and temporal properties before exploring the model,
// and requires a next-state predicate when either is configured.
func (m *model) validateNextRequiredForChecks() error {
	if m.cfg.Next != "" {
		return nil
	}
	if len(m.cfg.Invariants) > 0 || m.hasTemporalProperty() {
		return &ConfigurationError{Code: 2232, Message: "The configuration file did not specify the next state predicate."}
	}
	return nil
}

func (m *model) hasTemporalProperty() bool {
	for _, name := range m.cfg.Properties {
		expr, err := m.definition(name)
		if err == nil && m.hasTemporal(expr, map[string]bool{}) {
			return true
		}
	}
	return false
}
