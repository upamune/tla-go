package value

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

// UnexpectedError is a Java TLC runtime exception, including its root cause.
type UnexpectedError struct {
	Message         string
	TLCAssert       bool
	WrongInvocation bool
	ToolCode        int
	Sources         []*parser.Expression
}

func (e *UnexpectedError) Error() string { return e.Message }

// UnionFirstElementError represents TLC's failure when the first outer element
// of a UNION iterator is not enumerable.
type UnionFirstElementError struct{ Cause *UnexpectedError }

func (e *UnionFirstElementError) Error() string { return e.Cause.Error() }
func (e *UnionFirstElementError) Unwrap() error { return e.Cause }

// FingerprintError records the source expressions of nested lazy values.
type FingerprintError struct {
	Cause   error
	Sources []*parser.Expression
}

func (e *FingerprintError) Error() string { return e.Cause.Error() }
func (e *FingerprintError) Unwrap() error { return e.Cause }

func (v Value) withFingerprintSource(err error) error {
	if err == nil || v.data == nil || v.data.source == nil {
		return err
	}
	if failure, ok := errors.AsType[*FingerprintError](err); ok {
		if len(failure.Sources) > 0 && failure.Sources[0] == v.data.source {
			return err
		}
		return &FingerprintError{Cause: failure.Cause, Sources: append([]*parser.Expression{v.data.source}, failure.Sources...)}
	}
	if _, ok := errors.AsType[*UnexpectedError](err); ok {
		return &FingerprintError{Cause: err, Sources: []*parser.Expression{v.data.source}}
	}
	return err
}

func (e *FingerprintError) Trace() string {
	var result strings.Builder
	for i, v := range slices.Backward(e.Sources) {
		r := v.Location.Range
		fmt.Fprintf(&result, "%d) line %d, col %d to line %d, col %d of module %s\n", i, r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
	}
	return result.String()
}
