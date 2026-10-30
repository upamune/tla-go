package evaluator

import (
	"errors"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// RuntimeError is an evaluation failure that TLC reports as part of a model
// checking result. Standard identifies errors raised by a standard module.
type RuntimeError struct {
	Message     string
	Legacy      string
	Standard    bool
	ExitCode    int
	Unexpected  *value.UnexpectedError
	Fingerprint *value.FingerprintError
	Sources     []*parser.Expression
}

// AddUnexpectedSource records an evaluation frame unless a fingerprint stack
// already supplies the more specific value provenance.
func AddUnexpectedSource(err error, source *parser.Expression) {
	if missing, ok := errors.AsType[*UnboundVariable](err); ok {
		missing.Sources = append(missing.Sources, source)
		return
	}
	failure, ok := errors.AsType[*RuntimeError](err)
	if ok {
		if failure.Unexpected != nil && failure.Fingerprint == nil {
			failure.Unexpected.Sources = append(failure.Unexpected.Sources, source)
		}
		if len(failure.Sources) > 0 {
			failure.Sources = append(failure.Sources, source)
		}
		return
	}
	addValueErrorSource(err, source)
}

func addValueErrorSource(err error, source *parser.Expression) {
	if _, ok := errors.AsType[*value.FingerprintError](err); ok {
		return
	}
	if failure, ok := errors.AsType[*value.UnexpectedError](err); ok {
		failure.Sources = append(failure.Sources, source)
	}
	if failure, ok := errors.AsType[*value.TLCError](err); ok && failure.Standard {
		failure.Sources = append(failure.Sources, source)
	}
}

// ValueRuntimeError converts a TLC value failure without losing its source stack.
func ValueRuntimeError(err error) (*RuntimeError, bool) {
	unexpected, ok := errors.AsType[*value.UnexpectedError](err)
	if !ok {
		return nil, false
	}
	fingerprint, _ := errors.AsType[*value.FingerprintError](err)
	return &RuntimeError{Message: unexpected.Message, Unexpected: unexpected, Fingerprint: fingerprint}, true
}

func (e *RuntimeError) Error() string {
	if e.Legacy != "" {
		return e.Legacy
	}
	return e.Message
}
