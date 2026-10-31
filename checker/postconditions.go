package checker

import (
	"errors"
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

func (m *model) checkPostconditions(result Result) (Result, error) {
	m.tlcRuntime.level = 0
	m.tlcRuntime.counterExample = result.Violation
	if unwitnessed := m.unwitnessedPossible(); unwitnessed != nil {
		return possibleResult(result, unwitnessed), nil
	}
	for _, name := range m.cfg.Postconditions {
		expr := &parser.Expression{Kind: "Identifier", Text: name}
		holds, err := m.holds(expr, nil, nil)
		if err != nil {
			failure, ok := errors.AsType[*evaluator.RuntimeError](err)
			if !ok {
				return result, err
			}
			return postconditionResult(result, name, failure.Message), nil
		}
		if !holds {
			return postconditionResult(result, name, fmt.Sprintf("Postcondition %s is false.", name)), nil
		}
	}
	return result, nil
}

func postconditionResult(result Result, name, message string) Result {
	if result.Status != Success {
		return result
	}
	result.Status = PostconditionViolation
	result.Violation = &Violation{Name: name, Message: message, Trace: []TraceStep{}}
	return result
}
