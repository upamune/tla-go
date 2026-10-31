package evaluator

import (
	"errors"
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// RuntimeError is an evaluation failure that TLC reports as part of a model
// checking result. Standard identifies errors raised by a standard module.
type RuntimeError struct {
	Message  string
	Legacy   string
	Standard bool
	ExitCode int
	// ToolCode identifies the corresponding Java TLC message when known.
	ToolCode       int
	NestedToolCode int
	NestedMessage  string
	TrackSource    bool
	// BooleanOperator identifies a TLC type error in a Boolean operator.
	BooleanOperator bool
	Unexpected      *value.UnexpectedError
	Fingerprint     *value.FingerprintError
	Sources         []*parser.Expression
	// WrongInvocation marks a failure that Java TLC raises as a
	// util.WrongInvocationException, without an evaluation stack.
	WrongInvocation bool
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
		if failure.TrackSource || len(failure.Sources) > 0 {
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

// javaOverrideError reports an exception raised by a Java TLC override.
func javaOverrideError(signature, message string) error {
	return &RuntimeError{Message: javaOverrideMessage(signature, message), Standard: true}
}

// javaOverrideMessage is the text with which TLC's MethodValue wraps an
// exception raised by a Java override.
func javaOverrideMessage(signature, message string) string {
	return fmt.Sprintf("Attempted to apply the operator overridden by the Java method\n%s,\nbut it produced the following error:\n%s", signature, message)
}

func (e *RuntimeError) Error() string {
	if e.Legacy != "" {
		return e.Legacy
	}
	return e.Message
}
