package evaluator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/upamune/tla-go/value"
)

func (e *evaluator) jsonEncode(v value.Value, signature string) (string, error) {
	if text, ok := v.Text(); ok {
		return jsonQuote(text), nil
	}
	if n, ok := v.Int(); ok {
		return strconv.FormatInt(int64(n), 10), nil
	}
	if b, ok := v.Bool(); ok {
		return strconv.FormatBool(b), nil
	}
	if v.IsSet() {
		return e.jsonSet(v, signature)
	}
	if jsonFunction(v) {
		return e.jsonFunction(v, signature)
	}
	return "", jsonUnsupported(signature, v)
}

func (e *evaluator) jsonSet(v value.Value, signature string) (string, error) {
	if !v.IsEnumerable() {
		return "", jsonUnsupported(signature, v)
	}
	elements, err := v.Elements()
	if err != nil {
		return "", err
	}
	ordered, err := e.orderedSet(elements)
	if err != nil {
		return "", err
	}
	elements, err = ordered.Elements()
	if err != nil {
		return "", err
	}
	return e.jsonArray(elements, signature)
}

func (e *evaluator) jsonArray(elements []value.Value, signature string) (string, error) {
	result := make([]string, len(elements))
	for i, item := range elements {
		text, err := e.jsonEncode(item, signature)
		if err != nil {
			return "", err
		}
		result[i] = text
	}
	return "[" + strings.Join(result, ",") + "]", nil
}

func (e *evaluator) jsonFunction(v value.Value, signature string) (string, error) {
	if elements, err := v.TupleElements(); err == nil {
		return e.jsonArray(elements, signature)
	}
	pairs, err := e.orderedPairs(v)
	if err != nil {
		return "", err
	}
	result := make([]string, len(pairs))
	for i, pair := range pairs {
		key, err := jsonKey(pair.Key)
		if err != nil {
			return "", err
		}
		field, err := e.jsonEncode(pair.Value, signature)
		if err != nil {
			return "", err
		}
		result[i] = jsonQuote(key) + ":" + field
	}
	return "{" + strings.Join(result, ",") + "}", nil
}

func jsonKey(key value.Value) (string, error) {
	if text, ok := key.Text(); ok && key.Kind() == value.StringKind {
		return text, nil
	}
	return key.StrictString()
}

func jsonUnsupported(signature string, v value.Value) error {
	return jsonOverrideError(signature, fmt.Sprintf("Cannot convert value: unsupported value type tlc2.value.impl.%s", jsonJavaClass(v)))
}

func jsonOverrideError(signature, message string) error {
	return &RuntimeError{Message: fmt.Sprintf("Attempted to apply the operator overridden by the Java method\n%s,\nbut it produced the following error:\n%s", signature, message), Standard: true}
}

func jsonJavaClass(v value.Value) string {
	if class, ok := jsonSetClasses[v.Kind()]; ok {
		return class
	}
	switch v.Kind() {
	case value.IntegerKind:
		return "IntValue"
	case value.BooleanKind:
		return "BoolValue"
	case value.StringKind:
		return "StringValue"
	case value.ModelKind:
		return "ModelValue"
	case value.LazyFunctionKind:
		return "FcnLambdaValue"
	case value.FunctionKind:
		return jsonFunctionClass(v)
	default:
		return "Value"
	}
}

var jsonSetClasses = map[value.Kind]string{
	value.SetKind: "SetEnumValue", value.IntervalKind: "IntervalValue",
	value.InfiniteSetKind: "UserValue", value.SequenceSetKind: "UserValue",
	value.FunctionSetKind: "SetOfFcnsValue", value.RecordSetKind: "SetOfRcdsValue",
	value.ProductSetKind: "SetOfTuplesValue", value.SubsetKind: "SubsetValue",
	value.UnionKind: "UnionValue", value.FilteredSetKind: "SetPredValue",
}

func jsonFunctionClass(v value.Value) string {
	if _, err := v.TupleElements(); err == nil {
		return "TupleValue"
	}
	if jsonRecord(v) {
		return "RecordValue"
	}
	return "FcnRcdValue"
}
