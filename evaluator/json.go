package evaluator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

var jsonArities = map[string]int{"ToJson": 1, "ToJsonArray": 1, "ToJsonObject": 1, "JsonSerialize": 2, "ndJsonSerialize": 2, "JsonDeserialize": 1, "ndJsonDeserialize": 1}

var jsonSignatures = map[string]string{
	"ToJson":            "public static tlc2.value.impl.StringValue tlc2.module.Json.toJson(tlc2.value.IValue) throws java.io.IOException",
	"ToJsonArray":       "public static tlc2.value.impl.StringValue tlc2.module.Json.toJsonArray(tlc2.value.IValue) throws java.io.IOException",
	"ToJsonObject":      "public static tlc2.value.impl.StringValue tlc2.module.Json.toJsonObject(tlc2.value.IValue) throws java.io.IOException",
	"JsonSerialize":     "public static synchronized tlc2.value.impl.BoolValue tlc2.module.Json.serialize(tlc2.value.impl.StringValue,tlc2.value.impl.Value) throws java.io.IOException",
	"ndJsonSerialize":   "public static synchronized tlc2.value.impl.BoolValue tlc2.module.Json.ndSerialize(tlc2.value.impl.StringValue,tlc2.value.impl.Value) throws java.io.IOException",
	"JsonDeserialize":   "public static tlc2.value.IValue tlc2.module.Json.deserialize(tlc2.value.impl.StringValue) throws java.io.IOException",
	"ndJsonDeserialize": "public static tlc2.value.IValue tlc2.module.Json.ndDeserialize(tlc2.value.impl.StringValue) throws java.io.IOException",
}

func isJSONOperator(name string) bool { _, ok := jsonArities[name]; return ok }

func (e *evaluator) jsonOperator(expr *parser.Expression) (value.Value, error) {
	if len(expr.Args) != jsonArities[expr.Text] {
		return value.Value{}, fmt.Errorf("%s expects %d arguments", expr.Text, jsonArities[expr.Text])
	}
	args, err := e.evalArgs(expr.Args)
	if err != nil {
		return value.Value{}, err
	}
	return e.jsonValue(expr.Text, args)
}

func (e *evaluator) jsonValue(name string, args []value.Value) (value.Value, error) {
	signature, ok := jsonSignatures[name]
	if !ok {
		return value.Value{}, fmt.Errorf("%s is not yet supported", name)
	}
	if strings.Contains(name, "Serialize") || strings.Contains(name, "Deserialize") {
		return e.jsonFileValue(name, signature, args)
	}
	v := args[0]
	if !validJSONArgument(name, v) {
		return value.Value{}, jsonUnsupported(signature, v)
	}
	result, err := e.jsonEncode(v, signature)
	if err != nil {
		return value.Value{}, err
	}
	return value.String(result), nil
}

func validJSONArgument(name string, v value.Value) bool {
	if name == "ToJsonArray" {
		return v.IsSet() || jsonFunction(v) && !jsonRecord(v)
	}
	return name != "ToJsonObject" || jsonFunction(v)
}

func jsonFunction(v value.Value) bool {
	return v.Kind() == value.FunctionKind || v.Kind() == value.LazyFunctionKind
}

func jsonRecord(v value.Value) bool {
	pairs, err := v.Pairs()
	if err != nil || len(pairs) == 0 {
		return false
	}
	for _, pair := range pairs {
		if pair.Key.Kind() != value.StringKind {
			return false
		}
	}
	return true
}

func jsonQuote(text string) string {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(text)
	return strings.TrimSuffix(output.String(), "\n")
}
