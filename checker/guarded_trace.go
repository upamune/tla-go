package checker

func (s *search) guardedViolations(trace []TraceStep) ([]string, error) {
	seen := map[string]bool{}
	names := make([]string, 0, len(s.model.cfg.Properties))
	for _, property := range s.model.guarded {
		if seen[property.name] {
			continue
		}
		violated, err := s.guardedTraceViolates(property, trace)
		if err != nil {
			return nil, err
		}
		if violated {
			seen[property.name] = true
			names = append(names, property.name)
		}
	}
	return names, nil
}

func (s *search) guardedTraceViolates(property temporalProperty, trace []TraceStep) (bool, error) {
	return s.guardedBehaviorViolates(property, trace, -1)
}

func (s *search) guardedBehaviorViolates(property temporalProperty, trace []TraceStep, loop int) (bool, error) {
	armed := false
	violated, err := s.guardedTraceSegment(property, trace, &armed)
	if err != nil || violated || loop < 0 {
		return violated, err
	}
	return s.guardedTraceSegment(property, trace[loop:], &armed)
}

func (s *search) guardedTraceSegment(property temporalProperty, trace []TraceStep, armed *bool) (bool, error) {
	for _, step := range trace {
		trigger, err := s.model.holdsProperty(property.trigger, step.State, property.locals)
		if err != nil {
			return false, err
		}
		*armed = *armed || trigger
		if !*armed {
			continue
		}
		valid, err := s.model.holdsProperty(property.predicate, step.State, property.locals)
		if err != nil || !valid {
			return !valid, err
		}
	}
	return false, nil
}
