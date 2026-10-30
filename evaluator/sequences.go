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
		return value.Value{}, err
	}
	return value.Integer(int64(len(items)))
}

func sequenceAppend(seq, item value.Value) (value.Value, error) {
	items, err := seq.TupleElements()
	if err != nil {
		return value.Value{}, err
	}
	return value.Tuple(append(items, item))
}

func sequenceHeadTail(name string, v value.Value) (value.Value, error) {
	if name == "Tail" && v.Kind() == value.StringKind {
		units := stringUnits(v)
		if len(units) == 0 {
			return value.Value{}, fmt.Errorf("Tail of empty sequence")
		}
		return decodeStringUnits(units[1:])
	}
	items, err := v.TupleElements()
	if err != nil {
		return value.Value{}, err
	}
	if len(items) == 0 {
		return value.Value{}, fmt.Errorf("%s of empty sequence", name)
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
	if left.Kind() == value.StringKind && right.Kind() == value.StringKind {
		a, _ := left.Text()
		b, _ := right.Text()
		return value.String(a + b), nil
	}
	a, err := left.TupleElements()
	if err != nil {
		return value.Value{}, err
	}
	b, err := right.TupleElements()
	if err != nil {
		return value.Value{}, err
	}
	return value.Tuple(append(a, b...))
}

func sequenceSubsequence(args []value.Value) (value.Value, error) {
	start, err := integer(args[1])
	if err != nil {
		return value.Value{}, err
	}
	end, err := integer(args[2])
	if err != nil {
		return value.Value{}, err
	}
	if args[0].Kind() == value.StringKind {
		return stringSubsequence(args[0], start, end)
	}
	return tupleSubsequence(args[0], start, end)
}

func stringSubsequence(v value.Value, start, end int64) (value.Value, error) {
	units := stringUnits(v)
	if start > end {
		return value.String(""), nil
	}
	if start < 1 || end > int64(len(units)) {
		return value.Value{}, fmt.Errorf("SubSeq index is outside the sequence")
	}
	return decodeStringUnits(units[start-1 : end])
}

func tupleSubsequence(v value.Value, start, end int64) (value.Value, error) {
	items, err := v.TupleElements()
	if err != nil {
		return value.Value{}, err
	}
	if start > end {
		return value.Tuple(nil)
	}
	if start < 1 || end > int64(len(items)) {
		return value.Value{}, fmt.Errorf("SubSeq index is outside the sequence")
	}
	return value.Tuple(items[start-1 : end])
}
