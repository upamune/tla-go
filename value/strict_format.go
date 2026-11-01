package value

import (
	"fmt"
	"strings"
)

// StrictString renders supported finite values without silently replacing a
// value that cannot be enumerated with a diagnostic placeholder.
func (v Value) StrictString() (string, error) { return v.render(false) }

// TerseString renders v as Java TLC does with -terse: SUBSET, function-set,
// record-set, and Cartesian-product values are printed as formulas rather
// than enumerated, at any depth. Other values render as with StrictString.
func (v Value) TerseString() (string, error) { return v.render(true) }

func (v Value) render(terse bool) (string, error) {
	switch v.kind {
	case IntegerKind, BooleanKind, StringKind, ModelKind, IntervalKind, InfiniteSetKind:
		return v.String(), nil
	case SetKind:
		parts, err := strictValues(v.setIteration(), terse)
		return "{" + strings.Join(parts, ", ") + "}", err
	case FunctionKind, LazyFunctionKind:
		if v.hasLazyDescription() {
			return v.data.lazy.description, nil
		}
		f, err := v.materializedFunction()
		if err != nil {
			return "", err
		}
		return strictFunction(f, terse)
	case SequenceSetKind:
		base, err := v.data.domain.render(terse)
		if err != nil {
			return "", err
		}
		return "Seq(" + base + ")", nil
	default:
		return v.strictLazySet(terse)
	}
}

func strictValues(values []Value, terse bool) ([]string, error) {
	parts := make([]string, len(values))
	for i, item := range values {
		var err error
		parts[i], err = item.render(terse)
		if err != nil {
			return nil, err
		}
	}
	return parts, nil
}

func strictFunction(v Value, terse bool) (string, error) {
	if v.emptyFunctionRecord() {
		return emptyFunctionRecordText, nil
	}
	pairs := v.displayPairs()
	parts := make([]string, len(pairs))
	tuple, record := tupleDomain(pairs), printableRecordDomain(pairs)
	for i, pair := range pairs {
		part, err := strictPair(pair, tuple, record, terse)
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

func strictPair(pair Pair, tuple, record, terse bool) (string, error) {
	v, err := pair.Value.render(terse)
	if err != nil || tuple {
		return v, err
	}
	if record {
		return pair.Key.data.text + " |-> " + v, nil
	}
	key, err := pair.Key.render(terse)
	if err != nil {
		return "", err
	}
	return key + " :> " + v, nil
}

func (v Value) strictLazySet(terse bool) (string, error) {
	if !isLazySet(v) {
		return "", fmt.Errorf("cannot render unsupported value")
	}
	if terse && v.symbolicSetKind() {
		return v.strictSymbolicSet(terse)
	}
	expand, err := v.strictExpand()
	if err != nil {
		return "", err
	}
	if !expand {
		return v.strictSymbolicSet(terse)
	}
	set, err := v.finiteSet()
	if err != nil {
		return "", err
	}
	return set.render(terse)
}

// symbolicSetKind reports whether Java TLC keeps v as a lazy set value whose
// unexpanded form is a formula.
func (v Value) symbolicSetKind() bool {
	switch v.kind {
	case SubsetKind, FunctionSetKind, RecordSetKind, ProductSetKind:
		return true
	}
	return false
}
