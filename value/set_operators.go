package value

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

// Subsets returns the powerset of base, without enumerating its members.
func Subsets(base Value) (Value, error) {
	return SubsetsWithSource(base, nil)
}

// TLC eagerly represents the union of enumerated sets as an enumerated set.
func (v Value) tlcUnionEnumerated() bool {
	if v.kind != UnionKind || v.data.domain.kind != SetKind {
		return false
	}
	for _, member := range v.data.domain.data.elements {
		if member.kind != SetKind {
			return false
		}
	}
	return true
}

// SubsetsWithSource retains the source of a deferred set for TLC diagnostics.
func SubsetsWithSource(base Value, source *parser.Expression) (Value, error) {
	if !isSet(base) {
		return Value{}, fmt.Errorf("SUBSET requires a set")
	}
	return Value{kind: SubsetKind, data: &payload{domain: base, source: source}}, nil
}

// SubsetOf checks inclusion without expanding pairs of power sets, as TLC does.
func (v Value) SubsetOf(other Value) (bool, error) {
	if !isSet(v) || !isSet(other) {
		return false, fmt.Errorf("subset comparison requires two sets")
	}
	if v.kind == SubsetKind && other.kind == SubsetKind && v.data.domain.IsEnumerable() {
		found, err := v.data.domain.SubsetOf(other.data.domain)
		return found, v.withFingerprintSource(err)
	}
	found := true
	err := v.Each(func(item Value) (bool, error) {
		var err error
		found, err = other.Contains(item)
		return found, err
	})
	if v.kind == SubsetKind {
		err = v.withFingerprintSource(err)
	}
	return found, err
}

// Union returns the union of the members of a set of sets.
func Union(base Value) (Value, error) {
	if !isSet(base) {
		return Value{}, fmt.Errorf("UNION requires a set")
	}
	return Value{kind: UnionKind, data: &payload{domain: base}}, nil
}

func (v Value) containsSetOperator(element Value) (bool, error) {
	if v.kind == SubsetKind {
		return v.containsSubset(element)
	}
	return v.containsUnion(element)
}

func (v Value) containsSubset(element Value) (bool, error) {
	if !element.tlcEnumerable() {
		message := fmt.Sprintf("Attempted to check if the non-enumerable value\n%s\nis element of\n%s", PrettyPrint(element.String()), PrettyPrint(v.String()))
		return false, &UnexpectedError{Message: message, TLCAssert: true}
	}
	found := true
	err := element.Each(func(item Value) (bool, error) {
		var err error
		found, err = v.data.domain.Contains(item)
		return found, err
	})
	return found, err
}

// TLC's Enumerable interface also includes sets that cannot be enumerated for
// every choice of their operands, such as SUBSET Nat.
func (v Value) tlcEnumerable() bool {
	switch v.kind {
	case SetKind, IntervalKind, SubsetKind, FilteredSetKind, FunctionSetKind, RecordSetKind, ProductSetKind, SetOperationKind, UnionKind:
		return true
	default:
		return false
	}
}

func (v Value) containsUnion(element Value) (bool, error) {
	found := false
	err := v.data.domain.Each(func(set Value) (bool, error) {
		var err error
		found, err = set.Contains(element)
		return !found, err
	})
	return found, err
}

func (v Value) eachSetOperator(visit func(Value) (bool, error)) error {
	if v.kind == SubsetKind {
		return v.eachSubset(visit)
	}
	return v.eachUnion(visit)
}

// The reference evaluator rejects subset enumeration above 30 base members.
func (v Value) subsetBase() ([]Value, error) {
	var base []Value
	err := v.data.domain.Each(func(item Value) (bool, error) {
		base = append(base, item)
		if len(base) > 30 {
			return false, v.subsetOverflow()
		}
		return true, nil
	})
	return base, err
}

func (v Value) subsetOverflow() error {
	base, err := v.data.domain.StrictString()
	if err != nil {
		return err
	}
	return &UnexpectedError{Message: "Overflow when computing the number of elements in:\nSUBSET " + base, TLCAssert: true, ToolCode: 2178}
}

func (v Value) eachSubset(visit func(Value) (bool, error)) error {
	base, err := v.subsetBase()
	if err != nil {
		return v.withFingerprintSource(err)
	}
	for size := 0; size <= len(base); size++ {
		indices := make([]int, size)
		for i := range indices {
			indices[i] = i
		}
		more, err := visitCombinations(base, indices, visit)
		if err != nil || !more {
			return err
		}
	}
	return nil
}

func visitCombinations(base []Value, indices []int, visit func(Value) (bool, error)) (bool, error) {
	for {
		members := make([]Value, len(indices))
		for i, index := range indices {
			members[i] = base[index]
		}
		set, err := Set(members)
		if err != nil {
			return false, err
		}
		more, err := visit(set)
		if err != nil || !more || !advanceCombination(indices, len(base)) {
			return more, err
		}
	}
}

func advanceCombination(indices []int, n int) bool {
	for i := len(indices) - 1; i >= 0; i-- {
		if indices[i] < n-len(indices)+i {
			indices[i]++
			for j := i + 1; j < len(indices); j++ {
				indices[j] = indices[j-1] + 1
			}
			return true
		}
	}
	return false
}

func (v Value) eachUnion(visit func(Value) (bool, error)) error {
	var elements []Value
	first := true
	err := v.data.domain.Each(func(set Value) (bool, error) {
		if first && !set.tlcEnumerable() {
			return false, &UnionFirstElementError{Cause: &UnexpectedError{Message: "Attempted to enumerate UNION(s), but some element of s is nonenumerable.", TLCAssert: true}}
		}
		first = false
		return true, set.Each(func(item Value) (bool, error) {
			elements = append(elements, item)
			return true, nil
		})
	})
	if err != nil {
		return err
	}
	result, err := Set(elements)
	if err != nil {
		return err
	}
	return result.Each(visit)
}
