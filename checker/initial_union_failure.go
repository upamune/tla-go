package checker

import (
	"errors"

	"github.com/upamune/tla-go/value"
)

// TLC tries to print an initial state after its fingerprint fails. A failure
// while rendering that state replaces the fingerprint error and statistics.
func (s *search) initialUnionFingerprintFailure(err error) (Result, bool) {
	candidate, ok := errors.AsType[*candidateFingerprintError](err)
	if !ok {
		return Result{}, false
	}
	if _, ok := errors.AsType[*value.UnionFirstElementError](err); !ok {
		return Result{}, false
	}
	if failure := s.model.initialUnionRenderFailure(candidate.state); failure != nil {
		return Result{Status: EvaluationFailure, Failure: failure}, true
	}
	result, _ := s.runtimeFailure(err, nil, 75)
	result.Failure.ToolCode = 2102
	return result, true
}

func (m *model) initialUnionRenderFailure(state map[string]value.Value) *Failure {
	for _, name := range m.symmetryVariableOrder() {
		v := state[name]
		var renderErr error
		if m.terseValues {
			_, renderErr = v.TerseString()
		} else {
			_, renderErr = v.StrictString()
		}
		if renderErr != nil {
			return &Failure{Message: unexpectedExceptionMessage(runtimeExceptionClass, renderErr.Error()), ToolCode: 1000, ExitCode: 255, SuppressStatistics: true}
		}
	}
	return nil
}
