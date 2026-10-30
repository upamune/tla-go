package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/value"
)

var coreOperatorArities = map[string]int{
	"~": 1, "DOMAIN": 1, "UNION": 1, "SUBSET": 1,
	"=": 2, "#": 2, "<=>": 2, "=>": 2, "/\\": 2, "\\/": 2,
	"\\in": 2, "\\notin": 2, "\\subseteq": 2, "\\cup": 2, "\\cap": 2, "\\": 2,
}

var integerOperatorArities = map[string]int{
	"+": 2, "-": 2, "-.": 1, "*": 2, "^": 2, "\\div": 2, "%": 2, "..": 2,
	"<": 2, "<=": 2, ">": 2, ">=": 2,
}

func (e *evaluator) supportsBuiltin(name string, arity int) bool {
	if e.randomization && matchesArity(randomArities, name, arity) {
		return true
	}
	return e.supportsNonRandomBuiltin(name, arity)
}

func (e *evaluator) supportsNonRandomBuiltin(name string, arity int) bool {
	if e.json && matchesArity(jsonArities, name, arity) {
		return true
	}
	if name == "SelectSeq" || name == "SortSeq" || name == "BagOfAll" {
		return false
	}
	if n, ok := coreOperatorArities[name]; ok {
		return n == arity
	}
	return e.supportsStandardBuiltin(name, arity)
}

func (e *evaluator) supportsStandardBuiltin(name string, arity int) bool {
	if name == "-." && !e.integers {
		return false
	}
	if e.naturals && matchesArity(integerOperatorArities, name, arity) {
		return true
	}
	if e.sequences && matchesArity(sequenceArities, name, arity) {
		return true
	}
	return e.supportsOtherBuiltin(name, arity)
}

func (e *evaluator) supportsOtherBuiltin(name string, arity int) bool {
	if e.supportsTLCExtValue(name, arity) {
		return true
	}
	if e.bags && matchesArity(bagArities, name, arity) {
		return true
	}
	if e.finiteSets && (name == "Cardinality" || name == "IsFiniteSet") {
		return arity == 1
	}
	return e.tlc && matchesArity(tlcArities, name, arity)
}

func matchesArity(operators map[string]int, name string, arity int) bool {
	n, ok := operators[name]
	return ok && n == arity
}

func (e *evaluator) invokeBuiltin(name string, args []value.Value) (value.Value, error) {
	if e.randomization && isRandomOperator(name) {
		return e.randomValue(name, args)
	}
	if e.bags && isBagOperator(name) {
		return bagValue(name, args)
	}
	if e.json && isJSONOperator(name) {
		return e.jsonValue(name, args)
	}
	return e.invokeOtherBuiltin(name, args)
}

func (e *evaluator) invokeOtherBuiltin(name string, args []value.Value) (value.Value, error) {
	if e.isTLCBuiltin(name) {
		return e.invokeTLCBuiltin(name, args)
	}
	if e.sequences && isSequenceOperator(name) {
		return sequenceValue(name, args)
	}
	if e.finiteSets && (name == "Cardinality" || name == "IsFiniteSet") {
		return finiteSetsValue(name, args[0])
	}
	return e.invokeCoreBuiltin(name, args)
}

func (e *evaluator) invokeCoreBuiltin(name string, args []value.Value) (value.Value, error) {
	if len(args) == 1 {
		return unary(name, args[0])
	}
	if len(args) == 2 {
		return operatorBinary(name, args[0], args[1], e.naturals && !e.integers)
	}
	return value.Value{}, fmt.Errorf("unsupported operator %s", name)
}

func operatorBinary(name string, left, right value.Value, naturalOnly bool) (value.Value, error) {
	if !booleanOperator(name) {
		return comparisonCheckedBinary(name, left, right, naturalOnly)
	}
	a, err := boolean(left)
	if err != nil {
		return value.Value{}, err
	}
	b, err := boolean(right)
	if err != nil {
		return value.Value{}, err
	}
	return booleanBinary(name, a, b), nil
}

func booleanBinary(name string, a, b bool) value.Value {
	switch name {
	case "=>":
		return value.Bool(!a || b)
	case "/\\":
		return value.Bool(a && b)
	default:
		return value.Bool(a || b)
	}
}

func booleanOperator(name string) bool {
	switch name {
	case "=>", "/\\", "\\/":
		return true
	default:
		return false
	}
}

func (e *evaluator) supportsTLCExtValue(name string, arity int) bool {
	return e.tlcext && name != "TLCDefer" && name != "AssertError" && name != "TLCGetAndSet" && matchesArity(tlcExtArities, name, arity)
}

func (e *evaluator) invokeTLCBuiltin(name string, args []value.Value) (value.Value, error) {
	if e.tlcext && isTLCExtOperator(name) {
		return e.tlcExtValue(name, args)
	}
	return e.callTLCValue(name, args)
}
