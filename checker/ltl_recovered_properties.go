package checker

// Property names must describe the concrete trace recovered from reduced
// fingerprints, which may differ from the states used during exploration.
func (m *model) updateRecoveredTableauProperties(result *Result) error {
	s := m.tlcRuntime.search
	if result.Status != LivenessViolation || s == nil || !s.reducedTableauTrace {
		return nil
	}
	names, err := s.completedSearchLassoNames(result.Violation)
	if err != nil {
		return err
	}
	result.Violation.Properties = names
	result.Violation.Name = ""
	if len(names) > 0 {
		result.Violation.Name = names[0]
	}
	return nil
}
