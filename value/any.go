package value

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

// TLCError carries a value-level diagnostic that should be reported as an
// evaluation failure rather than as an unsupported model error.
type TLCError struct {
	Message  string
	Standard bool
	Sources  []*parser.Expression
}

func (e *TLCError) Error() string { return e.Message }

func isAny(v Value) bool { return v.kind == InfiniteSetKind && v.number == anySet }

func anyWithModel(a, b Value) bool {
	return isAny(a) && b.kind == ModelKind || isAny(b) && a.kind == ModelKind
}

func equalAny(a, b Value) (bool, error) {
	if anyWithModel(a, b) {
		return false, anyModelEqualityError(a, b)
	}
	return false, anyEqualityError(a, b)
}

func compareAnyValues(a, b Value) (int, error) {
	if anyWithModel(a, b) {
		return compareModel(a, b)
	}
	return 0, anyComparisonError(a, b)
}

func anyModelEqualityError(a, b Value) error {
	if b.kind == ModelKind {
		if modelType(b) == "" {
			return nil
		}
		return &TLCError{Message: fmt.Sprintf("Attempted to check equality of the typed model value %s and the non-model value\nANY", b)}
	}
	if modelType(a) == "" {
		return nil
	}
	return &TLCError{Message: fmt.Sprintf("Attempted to check equality of typed model value %s and non-model value\nANY", a)}
}

func anyComparisonError(a, b Value) error {
	if !isAny(a) {
		a, b = b, a
	}
	return &TLCError{Message: fmt.Sprintf("Attempted to compare ANY with the value\n%s", b)}
}

func anyEqualityError(a, b Value) error {
	if isAny(a) && isAny(b) {
		return anyComparisonError(a, b)
	}
	if isAny(a) {
		return anyLeftEqualityError(b)
	}
	return anyRightEqualityError(a)
}

func anyLeftEqualityError(other Value) error {
	if other.IsSet() && other.IsEnumerable() {
		return &TLCError{Message: fmt.Sprintf("Attempted to compare overridden value ANY with non-overridden value:\n%s", other)}
	}
	if other.kind == IntegerKind || other.kind == BooleanKind || other.kind == StringKind {
		return &TLCError{Message: fmt.Sprintf("Attempted to compare set ANY with %s %s", valueKindName(other), other)}
	}
	return anyComparisonError(Any(), other)
}

func anyRightEqualityError(other Value) error {
	if other.IsSet() && other.IsEnumerable() {
		items, err := other.Elements()
		if err != nil {
			return err
		}
		set, err := Set(items)
		if err != nil {
			return err
		}
		return &TLCError{Message: fmt.Sprintf("Attempted to check equality of the set %s with the value:\nANY", set)}
	}
	if other.kind == InfiniteSetKind {
		return &TLCError{Message: fmt.Sprintf("Attempted to compare %s with the value\nANY", other)}
	}
	return anyRightOtherError(other)
}

func anyRightOtherError(other Value) error {
	if other.kind == FunctionKind && len(other.data.pairs) > 0 && printableRecordDomain(other.data.pairs) {
		return &TLCError{Message: fmt.Sprintf("Attempted to check equality of record:\n%s\nwith non-record\nANY", other)}
	}
	if other.kind == FunctionKind || other.kind == LazyFunctionKind {
		return &TLCError{Message: fmt.Sprintf("Attempted to check equality of the function %s with the value:\nANY", other)}
	}
	kind := valueKindName(other)
	verb := "check"
	if other.kind == BooleanKind {
		verb = "compare"
	}
	return &TLCError{Message: fmt.Sprintf("Attempted to %s equality of %s %s with non-%s:\nANY", verb, kind, other, kind)}
}

func valueKindName(v Value) string {
	switch v.kind {
	case IntegerKind:
		return "integer"
	case StringKind:
		return "string"
	case BooleanKind:
		return "boolean"
	default:
		return "value"
	}
}
