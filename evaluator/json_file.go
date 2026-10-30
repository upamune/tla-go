package evaluator

import (
	"fmt"
	"os"
	"strings"

	"github.com/upamune/tla-go/value"
)

func (e *evaluator) jsonFileValue(name, signature string, args []value.Value) (value.Value, error) {
	path, ok := args[0].Text()
	if !ok || args[0].Kind() != value.StringKind {
		return value.Value{}, jsonOverrideError(signature, "Cannot cast tlc2.value.impl."+jsonJavaClass(args[0])+" to tlc2.value.impl.StringValue")
	}
	if strings.HasSuffix(name, "Deserialize") {
		return e.jsonRead(name, signature, path)
	}
	return e.jsonWrite(name, signature, path, args[1])
}

func (e *evaluator) jsonRead(name, signature, path string) (value.Value, error) {
	if e.program.readFile == nil {
		return value.Value{}, jsonOverrideError(signature, path+" (No such file or directory)")
	}
	text, err := e.program.readFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return value.Value{}, jsonOverrideError(signature, path+" (No such file or directory)")
		}
		return value.Value{}, jsonOverrideError(signature, err.Error())
	}
	if name == "ndJsonDeserialize" {
		return e.jsonReadLines(text, signature)
	}
	return e.jsonDecode(text, signature)
}

func (e *evaluator) jsonReadLines(text, signature string) (value.Value, error) {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	items := make([]value.Value, 0)
	for line := range strings.SplitSeq(text, "\n") {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		item, err := e.jsonDecode(line, signature)
		if err != nil {
			return value.Value{}, err
		}
		items = append(items, item)
	}
	return value.Tuple(items)
}

func (e *evaluator) jsonWrite(name, signature, path string, v value.Value) (value.Value, error) {
	items, tupleError := v.TupleElements()
	if tupleError != nil && (name == "ndJsonSerialize" || !jsonRecord(v)) {
		kind := "sequence or record"
		if name == "ndJsonSerialize" {
			kind = "sequence"
		}
		return value.Value{}, &RuntimeError{Message: fmt.Sprintf("The second argument of %s should be a %s, but instead it is:\n%s", name, kind, v), Standard: true}
	}
	text, err := e.jsonWriteText(name, signature, v, items)
	if err != nil {
		return value.Value{}, err
	}
	if e.program.writeFile != nil {
		if err = e.program.writeFile(path, text); err != nil {
			return value.Value{}, jsonOverrideError(signature, err.Error())
		}
	}
	return value.Bool(true), nil
}

func (e *evaluator) jsonWriteText(name, signature string, v value.Value, items []value.Value) (string, error) {
	if name != "ndJsonSerialize" {
		return e.jsonEncode(v, signature)
	}
	var result strings.Builder
	for _, item := range items {
		text, err := e.jsonEncode(item, signature)
		if err != nil {
			return "", err
		}
		result.WriteString(text)
		result.WriteByte('\n')
	}
	return result.String(), nil
}
