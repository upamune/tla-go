package value

import (
	"strconv"
	"strings"
	"unicode"
)

// String returns the TLC-style rendering for the supported value forms.
func (v Value) String() string {
	switch v.kind {
	case IntegerKind:
		return strconv.FormatInt(int64(v.number), 10)
	case BooleanKind:
		return formatBool(v.number != 0)
	case StringKind:
		return strconv.Quote(v.data.text)
	case ModelKind:
		return v.data.text
	case IntervalKind:
		return formatInterval(v.number, v.high)
	case SetKind:
		return formatSet(v.setIteration())
	case FunctionKind:
		return formatFunction(v.displayPairs())
	default:
		return v.formatSpecial()
	}
}

func formatInterval(low, high int32) string {
	if high < low {
		return "{}"
	}
	return strconv.FormatInt(int64(low), 10) + ".." + strconv.FormatInt(int64(high), 10)
}

func formatBool(b bool) string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}

func formatSet(elements []Value) string {
	parts := make([]string, len(elements))
	for i, v := range elements {
		parts[i] = v.String()
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func formatFunction(pairs []Pair) string {
	if tupleDomain(pairs) {
		parts := make([]string, len(pairs))
		for i, pair := range pairs {
			parts[i] = pair.Value.String()
		}
		return "<<" + strings.Join(parts, ", ") + ">>"
	}
	if printableRecordDomain(pairs) {
		parts := make([]string, len(pairs))
		for i, pair := range pairs {
			parts[i] = pair.Key.data.text + " |-> " + pair.Value.String()
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	parts := make([]string, len(pairs))
	for i, pair := range pairs {
		parts[i] = pair.Key.String() + " :> " + pair.Value.String()
	}
	return "(" + strings.Join(parts, " @@ ") + ")"
}

func printableRecordDomain(pairs []Pair) bool {
	if !recordDomain(pairs) {
		return false
	}
	for _, pair := range pairs {
		if !fieldName(pair.Key.data.text) {
			return false
		}
	}
	return true
}

func fieldName(name string) bool {
	if len(name) >= 4 && (strings.HasPrefix(name, "WF_") || strings.HasPrefix(name, "SF_")) {
		return false
	}
	hasLetter := false
	for _, r := range name {
		if unicode.IsLetter(r) {
			hasLetter = true
			continue
		}
		if !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return hasLetter
}

func tupleDomain(pairs []Pair) bool {
	for i, pair := range pairs {
		n, ok := pair.Key.Int()
		if !ok || int64(n) != int64(i)+1 {
			return false
		}
	}
	return true
}

func (v Value) formatSpecial() string {
	if v.symbolicSubset() {
		return v.unenumerableString()
	}
	if v.kind == LazyFunctionKind {
		return v.formatLazyFunction()
	}
	if v.kind == InfiniteSetKind {
		return v.infiniteName()
	}
	if v.kind == SequenceSetKind {
		return "Seq(" + v.data.domain.String() + ")"
	}
	if !isLazySet(v) {
		return "<invalid>"
	}
	set, err := v.finiteSet()
	if err != nil {
		return v.unenumerableString()
	}
	return set.String()
}

func (v Value) formatLazyFunction() string {
	if v.hasLazyDescription() {
		return v.data.lazy.description
	}
	f, err := v.materializedFunction()
	if err != nil {
		return "<non-enumerable function>"
	}
	return f.String()
}

func (v Value) symbolicSubset() bool {
	if v.kind != SubsetKind {
		return false
	}
	expand, err := v.strictExpand()
	return err != nil || !expand
}

func (v Value) unenumerableString() string {
	if v.kind == SubsetKind {
		return "SUBSET " + v.data.domain.String()
	}
	return "<non-enumerable set>"
}

func (v Value) displayPairs() []Pair {
	pairs := v.data.pairs
	if len(v.data.displayOrder) == 0 {
		return pairs
	}
	lookup := make(map[string]Pair, len(pairs))
	for _, pair := range pairs {
		lookup[pair.Key.data.text] = pair
	}
	ordered := make([]Pair, 0, len(pairs))
	for _, name := range v.data.displayOrder {
		pair, ok := lookup[name]
		if !ok {
			return pairs
		}
		ordered = append(ordered, pair)
		delete(lookup, name)
	}
	return ordered
}
