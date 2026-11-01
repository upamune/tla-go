package evaluator

import (
	"fmt"
	"unicode/utf16"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

var sequenceArities = map[string]int{"Seq": 1, "Len": 1, "Append": 2, "Head": 1, "Tail": 1, "SubSeq": 3, "\\o": 2, "SelectSeq": 2}

func isSequenceOperator(name string) bool { _, ok := sequenceArities[name]; return ok }

func (e *evaluator) sequenceOperator(expr *parser.Expression) (value.Value, error) {
	if len(expr.Args) != sequenceArities[expr.Text] {
		return value.Value{}, fmt.Errorf("%s expects %d arguments", expr.Text, sequenceArities[expr.Text])
	}
	if expr.Text == "SelectSeq" {
		return e.selectSequence(expr.Args)
	}
	args, err := e.evalArgs(expr.Args)
	if err != nil {
		return value.Value{}, err
	}
	return sequenceValue(expr.Text, args)
}

func sequenceValue(name string, args []value.Value) (value.Value, error) {
	switch name {
	case "Seq":
		return value.Sequences(args[0]), nil
	case "Len":
		return sequenceLength(args[0])
	case "Append":
		return sequenceAppend(args[0], args[1])
	case "Head", "Tail":
		return sequenceHeadTail(name, args[0])
	case "SubSeq":
		return sequenceSubsequence(args)
	default:
		return sequenceConcat(args[0], args[1])
	}
}

func sequenceLength(v value.Value) (value.Value, error) {
	if s, ok := v.Text(); ok && v.Kind() == value.StringKind {
		return value.Integer(int64(len(utf16.Encode([]rune(s)))))
	}
	items, err := v.TupleElements()
	if err != nil {
		return value.Value{}, sequenceUnaryError("Len", v)
	}
	return value.Integer(int64(len(items)))
}

func sequenceAppend(seq, item value.Value) (value.Value, error) {
	if seq.Kind() == value.StringKind {
		if item.Kind() != value.StringKind {
			return value.Value{}, sequenceEvaluatingError("t \\o s", "string", item)
		}
		a, _ := seq.Text()
		b, _ := item.Text()
		return value.String(a + b), nil
	}
	items, err := seq.TupleElements()
	if err != nil {
		return value.Value{}, sequenceEvaluatingError("Append(v, s)", "sequence", seq)
	}
	return value.Tuple(append(items, item))
}

func sequenceHeadTail(name string, v value.Value) (value.Value, error) {
	if v.Kind() == value.StringKind {
		units := stringUnits(v)
		if len(units) == 0 {
			return value.Value{}, sequenceEmptyError(name)
		}
		if name == "Head" {
			return decodeStringUnits(units[:1])
		}
		return decodeStringUnits(units[1:])
	}
	items, err := v.TupleElements()
	if err != nil {
		return value.Value{}, sequenceUnaryError(name, v)
	}
	if len(items) == 0 {
		return value.Value{}, sequenceEmptyError(name)
	}
	if name == "Head" {
		return items[0], nil
	}
	return value.Tuple(items[1:])
}

func stringUnits(v value.Value) []uint16 {
	s, _ := v.Text()
	return utf16.Encode([]rune(s))
}

// Go strings cannot preserve an unpaired UTF-16 surrogate. Reject such slices
// until the value representation can represent them without replacement.
func decodeStringUnits(units []uint16) (value.Value, error) {
	for i := 0; i < len(units); i++ {
		if isLowSurrogate(units[i]) {
			return value.Value{}, fmt.Errorf("string slice contains an unpaired UTF-16 surrogate")
		}
		if isHighSurrogate(units[i]) {
			if i+1 >= len(units) || !isLowSurrogate(units[i+1]) {
				return value.Value{}, fmt.Errorf("string slice contains an unpaired UTF-16 surrogate")
			}
			i++
		}
	}
	return value.String(string(utf16.Decode(units))), nil
}

func isLowSurrogate(u uint16) bool  { return u >= 0xdc00 && u <= 0xdfff }
func isHighSurrogate(u uint16) bool { return u >= 0xd800 && u <= 0xdbff }

func sequenceConcat(left, right value.Value) (value.Value, error) {
	if left.Kind() == value.StringKind {
		if right.Kind() != value.StringKind {
			return value.Value{}, sequenceEvaluatingError("t \\o s", "string", right)
		}
		a, _ := left.Text()
		b, _ := right.Text()
		return value.String(a + b), nil
	}
	a, err := left.TupleElements()
	if err != nil {
		return value.Value{}, sequenceEvaluatingError("s \\o t", "sequence", left)
	}
	b, err := sequenceTupleElements(right)
	if err != nil {
		if right.Kind() == value.StringKind {
			return value.Value{}, err
		}
		return value.Value{}, sequenceEvaluatingError("t \\o s", "sequence", right)
	}
	return value.Tuple(append(a, b...))
}

// Java's StringValue.toTuple converts each UTF-16 unit into a string value.
func sequenceTupleElements(v value.Value) ([]value.Value, error) {
	if v.Kind() != value.StringKind {
		return v.TupleElements()
	}
	units := stringUnits(v)
	items := make([]value.Value, len(units))
	for i := range units {
		item, err := decodeStringUnits(units[i : i+1])
		if err != nil {
			return nil, err
		}
		items[i] = item
	}
	return items, nil
}

func sequenceSubsequence(args []value.Value) (value.Value, error) {
	isString := args[0].Kind() == value.StringKind
	items, units, err := subsequenceInput(args[0], isString)
	if err != nil {
		return value.Value{}, err
	}
	start, end, err := subsequenceIndexes(args)
	if err != nil {
		return value.Value{}, err
	}
	if start > end {
		if isString {
			return value.String(""), nil
		}
		return value.Tuple(nil)
	}
	n := int64(len(items))
	if isString {
		n = int64(len(units))
	}
	if err := subsequenceDomain(args, start, end, n); err != nil {
		return value.Value{}, err
	}
	if isString {
		return decodeStringUnits(units[start-1 : end])
	}
	return value.Tuple(items[start-1 : end])
}

func subsequenceInput(v value.Value, isString bool) ([]value.Value, []uint16, error) {
	if isString {
		return nil, stringUnits(v), nil
	}
	items, err := v.TupleElements()
	if err != nil {
		return nil, nil, sequenceArgumentError("first", "SubSeq", "sequence", v)
	}
	return items, nil, nil
}

func subsequenceDomain(args []value.Value, start, end, n int64) error {
	if start < 1 || start > n {
		return sequenceDomainError("second", args[0], args[1])
	}
	if end < 1 || end > n {
		return sequenceDomainError("third", args[0], args[2])
	}
	return nil
}

func subsequenceIndexes(args []value.Value) (int64, int64, error) {
	start, err := integer(args[1])
	if err != nil {
		return 0, 0, sequenceArgumentError("second", "SubSeq", "natural number", args[1])
	}
	end, err := integer(args[2])
	if err != nil {
		return 0, 0, sequenceArgumentError("third", "SubSeq", "natural number", args[2])
	}
	return start, end, nil
}

func sequenceError(code int, message string) error {
	return &RuntimeError{Message: message, Standard: true, ToolCode: code, TrackSource: true}
}

func sequenceEmptyError(name string) error {
	return sequenceError(2184, fmt.Sprintf("Attempted to apply %s to the empty sequence.", name))
}

func sequenceUnaryError(name string, v value.Value) error {
	return sequenceError(2283, fmt.Sprintf("The argument of %s should be a sequence, but instead it is:\n%s", name, v))
}

func sequenceArgumentError(position, name, expected string, v value.Value) error {
	return sequenceError(2169, fmt.Sprintf("The %s argument of %s should be a %s, but instead it is:\n%s", position, name, expected, v))
}

func sequenceEvaluatingError(form, expected string, v value.Value) error {
	return sequenceError(2182, fmt.Sprintf("Evaluating an expression of the form %s when s is not a %s:\n%s", form, expected, v))
}

func sequenceDomainError(position string, seq, index value.Value) error {
	return sequenceError(2183, fmt.Sprintf("The %s argument of SubSeq must be in the domain of its first argument:\n%s\n, but instead it is\n%s", position, seq, index))
}
