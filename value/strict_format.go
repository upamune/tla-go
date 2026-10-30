package value

import (
	"fmt"
	"strings"
)

// StrictString renders supported finite values without silently replacing a
// value that cannot be enumerated with a diagnostic placeholder.
func (v Value) StrictString() (string, error) {
	switch v.kind {
	case IntegerKind, BooleanKind, StringKind, ModelKind, IntervalKind, InfiniteSetKind:
		return v.String(), nil
	case SetKind:
		parts, err := strictValues(v.setIteration())
		return "{" + strings.Join(parts, ", ") + "}", err
	case FunctionKind, LazyFunctionKind:
		if v.hasLazyDescription() {
			return v.data.lazy.description, nil
		}
		f, err := v.materializedFunction()
		if err != nil {
			return "", err
		}
		return strictFunction(f)
	case SequenceSetKind:
		base, err := v.data.domain.StrictString()
		if err != nil {
			return "", err
		}
		return "Seq(" + base + ")", nil
	default:
		return v.strictLazySet()
	}
}

func strictValues(values []Value) ([]string, error) {
	parts := make([]string, len(values))
	for i, item := range values {
		var err error
		parts[i], err = item.StrictString()
		if err != nil {
			return nil, err
		}
	}
	return parts, nil
}

func strictFunction(v Value) (string, error) {
	pairs := v.displayPairs()
	parts := make([]string, len(pairs))
	tuple, record := tupleDomain(pairs), printableRecordDomain(pairs)
	for i, pair := range pairs {
		part, err := strictPair(pair, tuple, record)
		if err != nil {
			return "", err
		}
		parts[i] = part
	}
	if tuple {
		return "<<" + strings.Join(parts, ", ") + ">>", nil
	}
	if record {
		return "[" + strings.Join(parts, ", ") + "]", nil
	}
	return "(" + strings.Join(parts, " @@ ") + ")", nil
}

func strictPair(pair Pair, tuple, record bool) (string, error) {
	v, err := pair.Value.StrictString()
	if err != nil || tuple {
		return v, err
	}
	if record {
		return pair.Key.data.text + " |-> " + v, nil
	}
	key, err := pair.Key.StrictString()
	if err != nil {
		return "", err
	}
	return key + " :> " + v, nil
}

func (v Value) strictLazySet() (string, error) {
	if !isLazySet(v) {
		return "", fmt.Errorf("cannot render unsupported value")
	}
	expand, err := v.strictExpand()
	if err != nil {
		return "", err
	}
	if !expand {
		return v.strictSymbolicSet()
	}
	set, err := v.finiteSet()
	if err != nil {
		return "", err
	}
	return set.StrictString()
}
