package evaluator

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/upamune/tla-go/value"
)

type jsonMember struct {
	key   string
	value any
}

type jsonObject []jsonMember

func (e *evaluator) jsonDecode(text, signature string) (value.Value, error) {
	if strings.TrimSpace(text) == "" {
		return value.Value{}, javaOverrideError(signature, "Cannot convert value: unsupported JSON value null")
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	node, err := jsonReadNode(decoder)
	if err == nil {
		_, err = decoder.Token()
		if err == io.EOF {
			return e.jsonFromNode(node, signature)
		}
		if err == nil {
			err = fmt.Errorf("unexpected trailing JSON value")
		}
	}
	return value.Value{}, javaOverrideError(signature, err.Error())
}

func jsonReadNode(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := token.(json.Delim); ok {
		if delim == '[' {
			return jsonReadArray(decoder)
		}
		if delim == '{' {
			return jsonReadObject(decoder)
		}
	}
	return token, nil
}

func jsonReadArray(decoder *json.Decoder) ([]any, error) {
	items := make([]any, 0)
	for decoder.More() {
		item, err := jsonReadNode(decoder)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	_, err := decoder.Token()
	return items, err
}

func jsonReadObject(decoder *json.Decoder) (jsonObject, error) {
	var object jsonObject
	indices := map[string]int{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		item, err := jsonReadNode(decoder)
		if err != nil {
			return nil, err
		}
		name := key.(string)
		if index, ok := indices[name]; ok {
			object[index].value = item
		} else {
			indices[name] = len(object)
			object = append(object, jsonMember{name, item})
		}
	}
	_, err := decoder.Token()
	return object, err
}

func (e *evaluator) jsonFromNode(node any, signature string) (value.Value, error) {
	switch item := node.(type) {
	case bool:
		return value.Bool(item), nil
	case string:
		e.jsonIntern(item)
		return value.String(item), nil
	case json.Number:
		return jsonInteger(string(item))
	case []any:
		return e.jsonFromArray(item, signature)
	case jsonObject:
		return e.jsonFromObject(item, signature)
	default:
		return value.Value{}, javaOverrideError(signature, "Cannot convert value: unsupported JSON value null")
	}
}

func (e *evaluator) jsonIntern(text string) {
	if e.program.ordering != nil {
		e.program.ordering.Intern(text)
	}
}

func (e *evaluator) jsonFromArray(items []any, signature string) (value.Value, error) {
	result := make([]value.Value, len(items))
	for i, item := range items {
		v, err := e.jsonFromNode(item, signature)
		if err != nil {
			return value.Value{}, err
		}
		result[i] = v
	}
	return value.Tuple(result)
}

func (e *evaluator) jsonFromObject(object jsonObject, signature string) (value.Value, error) {
	slices.SortStableFunc(object, compareJSONKeys)
	pairs := make([]value.Pair, len(object))
	for i, item := range object {
		e.jsonIntern(item.key)
		v, err := e.jsonFromNode(item.value, signature)
		if err != nil {
			return value.Value{}, err
		}
		pairs[i] = value.Pair{Key: value.String(item.key), Value: v}
	}
	return e.orderedRecord(pairs)
}
