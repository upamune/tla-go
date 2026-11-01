package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/value"
)

// DefaultSetBound is Java TLC's default -maxSetSize.
const DefaultSetBound = 1000000

// javaValueVecCapacity is the initial capacity of Java TLC's ValueVec, which
// checks the bound only when it grows.
const javaValueVecCapacity = 10

// Signatures of the Java overrides that build a ValueVec.
const (
	cardinalitySignature = "public static tlc2.value.impl.IntValue tlc2.module.FiniteSets.Cardinality(tlc2.value.impl.Value)"
	selectSeqSignature   = "public static tlc2.value.impl.Value tlc2.module.Sequences.SelectSeq(tlc2.value.impl.Value,tlc2.value.impl.Value)"
	combineFcnSignature  = "public static tlc2.value.impl.Value tlc2.module.TLC.CombineFcn(tlc2.value.impl.Value,tlc2.value.impl.Value)"
	bagUnionSignature    = "public static tlc2.value.impl.Value tlc2.module.Bags.BagUnion(tlc2.value.impl.Value)"
	bagOfAllSignature    = "public static tlc2.value.impl.Value tlc2.module.Bags.BagOfAll(tlc2.value.impl.Value,tlc2.value.impl.Value)"
)

// SetBound limits the values that Java TLC builds element by element in a
// ValueVec: set maps and filters over enumerated sets or intervals, the set
// operations and UNIONs that TLC reduces eagerly, SelectSeq, @@, BagUnion,
// BagOfAll and Cardinality of a deferred set. A nonpositive bound selects
// DefaultSetBound.
func (p *Program) SetBound(bound int) {
	p.setBound = bound
}

func (p *Program) bound() int {
	if p.setBound <= 0 {
		return DefaultSetBound
	}
	return p.setBound
}

// setLimit returns the largest number of elements Java TLC's ValueVec accepts
// before its growth check fails.
func (p *Program) setLimit() int {
	return max(p.bound(), javaValueVecCapacity)
}

func (p *Program) checkSetSize(count int) error {
	return p.checkSetSize64(int64(count))
}

func (p *Program) checkSetSize64(count int64) error {
	if count <= int64(p.setLimit()) {
		return nil
	}
	message := fmt.Sprintf("Attempted to construct a set with too many elements (>%d).", p.bound())
	return &RuntimeError{Message: message, WrongInvocation: true}
}

// checkMethodSetSize reports a bound failure inside a Java override as
// message 2154, the assertion with which TLC's MethodValue wraps it.
func (p *Program) checkMethodSetSize(signature string, count int) error {
	err := p.checkSetSize(count)
	if err == nil {
		return nil
	}
	return &RuntimeError{Message: javaOverrideMessage(signature, err.Error()), ToolCode: 2154, TrackSource: true}
}

// checkEnumeratedSize counts the elements of set as Java TLC adds them to a
// ValueVec. upper bounds the count, so small sets need no enumeration.
func (p *Program) checkEnumeratedSize(set value.Value, upper int64) error {
	if upper <= int64(p.setLimit()) {
		return nil
	}
	count := 0
	return set.Each(func(value.Value) (bool, error) {
		count++
		return true, p.checkSetSize(count)
	})
}

// checkSetOperation bounds the set operations that Java TLC's reducible
// values, intervals and enumerated sets, compute eagerly: S \cup T when both
// are reducible and S is nonempty, S \cap T when either is, and S \ T when S
// is. Other operations stay lazy in TLC as well.
func (p *Program) checkSetOperation(operator string, left, right, result value.Value) error {
	upper, ok := reducedOperationSize(operator, left, right)
	if !ok || upper <= int64(p.setLimit()) {
		return nil
	}
	if operator == "\\cup" {
		return p.checkReducedUnion(left, right)
	}
	return p.checkEnumeratedSize(result, upper)
}

// checkReducedUnion counts S \cup T in the order of Java TLC's Reducible.cup:
// the elements of S, then each element of T that is not in S.
func (p *Program) checkReducedUnion(left, right value.Value) error {
	size, _ := reducibleSize(left)
	count := int(size)
	if err := p.checkSetSize(count); err != nil {
		return err
	}
	return right.Each(func(item value.Value) (bool, error) {
		found, err := left.Contains(item)
		if err != nil || found {
			return err == nil, err
		}
		count++
		return true, p.checkSetSize(count)
	})
}

func reducedOperationSize(operator string, left, right value.Value) (int64, bool) {
	switch operator {
	case "\\cup":
		l, leftReducible := reducibleSize(left)
		r, rightReducible := reducibleSize(right)
		return l + r, leftReducible && rightReducible && l > 0
	case "\\cap":
		if size, ok := reducibleSize(left); ok {
			return size, true
		}
		return reducibleSize(right)
	case "\\":
		return reducibleSize(left)
	default:
		return 0, false
	}
}

func reducibleSize(v value.Value) (int64, bool) {
	if low, high, ok := v.Range(); ok {
		return max(int64(high)-int64(low)+1, 0), true
	}
	if v.Kind() != value.SetKind {
		return 0, false
	}
	size, err := v.Cardinality()
	return size, err == nil
}

// checkUnion bounds UNION S, which Java TLC computes eagerly when S and all of
// its members are enumerated sets.
func (p *Program) checkUnion(base, result value.Value) error {
	if base.Kind() != value.SetKind {
		return nil
	}
	var upper int64
	enumerated := true
	_ = base.Each(func(member value.Value) (bool, error) {
		size, ok := reducibleSize(member)
		enumerated = ok && member.Kind() == value.SetKind
		upper += size
		return enumerated, nil
	})
	if !enumerated {
		return nil
	}
	return p.checkEnumeratedSize(result, upper)
}

// checkDeferredCardinality bounds Cardinality of the deferred set operations,
// unions and filters, whose size Java TLC computes by enumerating them.
func (p *Program) checkDeferredCardinality(set value.Value, size int64) error {
	switch set.Kind() {
	case value.SetOperationKind, value.UnionKind, value.FilteredSetKind:
		return p.checkMethodSetSize(cardinalitySignature, int(min(size, int64(p.setLimit())+1)))
	default:
		return nil
	}
}

// checkBuiltinSetSize bounds a set operator applied as an operator argument.
func (p *Program) checkBuiltinSetSize(name string, args []value.Value, result value.Value) error {
	if name == "UNION" {
		return p.checkUnion(args[0], result)
	}
	if len(args) == 2 {
		return p.checkSetOperation(name, args[0], args[1], result)
	}
	return nil
}

// methodBound applies the bound inside a Java override. Its zero value
// imposes no bound.
type methodBound struct {
	program   *Program
	signature string
}

func (b methodBound) check(count int) error {
	if b.program == nil {
		return nil
	}
	return b.program.checkMethodSetSize(b.signature, count)
}
