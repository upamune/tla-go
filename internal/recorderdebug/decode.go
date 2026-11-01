package recorderdebug

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Decode accepts only the supported partial protocol. Unknown event types and
// Java array/value types are errors, not silently converted observations.
func Decode(r io.Reader) ([]Event, error) {
	data, err := readStream(r)
	if err != nil {
		return nil, err
	}
	return decodeStream(data)
}

func readStream(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, 1<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("recorder stream exceeds size limit")
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("invalid recorder UTF-8")
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return nil, fmt.Errorf("incomplete recorder stream")
	}
	return data, nil
}

func decodeStream(data []byte) ([]Event, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var events []Event
	for scanner.Scan() {
		if len(events) >= 10000 {
			return nil, fmt.Errorf("too many recorder events")
		}
		event, err := decodeLine(scanner.Bytes())
		if err != nil {
			return nil, err
		}
		if err := validate(event, len(events)); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("missing recorder header")
	}
	return events[1:], nil
}

func decodeLine(line []byte) (Event, error) {
	keys, err := uniqueKeys(line)
	if err != nil {
		return Event{}, err
	}
	var raw struct {
		Kind     string            `json:"kind"`
		Version  int               `json:"version"`
		Complete *bool             `json:"complete"`
		Code     int               `json:"code"`
		Array    string            `json:"array"`
		Values   []json.RawMessage `json:"values"`
		Reason   string            `json:"reason"`
	}
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return Event{}, fmt.Errorf("decode recorder event: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Event{}, fmt.Errorf("trailing recorder data")
	}
	if raw.Complete == nil {
		return Event{}, fmt.Errorf("missing recorder completeness marker")
	}
	if err := validateFieldSet(keys, raw.Kind); err != nil {
		return Event{}, err
	}
	if raw.Kind == "record" && raw.Values == nil {
		return Event{}, fmt.Errorf("recorder values must be an array")
	}
	event := Event{Kind: raw.Kind, Version: raw.Version, Complete: *raw.Complete, Code: raw.Code, Array: raw.Array, Reason: raw.Reason}
	values, err := decodeValues(raw.Values)
	event.Values = values
	return event, err
}

func decodeValues(raw []json.RawMessage) ([]string, error) {
	var values []string
	for _, value := range raw {
		if len(value) == 0 || value[0] != '"' {
			return nil, fmt.Errorf("unsupported non-string recorder value")
		}
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return nil, err
		}
		values = append(values, text)
	}
	return values, nil
}

func uniqueKeys(line []byte) (map[string]bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(line))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("recorder event must be an object")
	}
	seen := make(map[string]bool)
	for decoder.More() {
		if err := readRecorderField(decoder, seen); err != nil {
			return nil, err
		}
	}
	_, err = decoder.Token()
	return seen, err
}

func readRecorderField(decoder *json.Decoder, seen map[string]bool) error {
	key, err := decoder.Token()
	if err != nil {
		return err
	}
	name, ok := key.(string)
	if !ok {
		return fmt.Errorf("invalid recorder field")
	}
	if err := addRecorderKey(seen, name); err != nil {
		return err
	}
	return decoder.Decode(new(json.RawMessage))
}

func validateFieldSet(keys map[string]bool, kind string) error {
	var required []string
	switch kind {
	case "header":
		required = []string{"kind", "version", "complete"}
	case "record":
		required = []string{"kind", "complete", "code", "array", "values"}
	case "omission":
		required = []string{"kind", "complete", "code", "reason"}
	default:
		return fmt.Errorf("unsupported recorder kind %q", kind)
	}
	if len(keys) != len(required) {
		return fmt.Errorf("unexpected recorder fields")
	}
	for _, key := range required {
		if !keys[key] {
			return fmt.Errorf("missing recorder field %q", key)
		}
	}
	return nil
}

func addRecorderKey(seen map[string]bool, name string) error {
	if seen[name] || !allowedRecorderKey(name) {
		return fmt.Errorf("duplicate or unsupported recorder field %q", name)
	}
	seen[name] = true
	return nil
}

func allowedRecorderKey(name string) bool {
	switch name {
	case "kind", "version", "complete", "code", "array", "values", "reason":
		return true
	default:
		return false
	}
}

func validate(event Event, index int) error {
	if index == 0 {
		return validateHeader(event)
	}
	if event.Kind == "omission" {
		return validateOmission(event)
	}
	if event.Kind != "record" || event.Version != 0 || event.Complete || event.Array != "strings" {
		return fmt.Errorf("unsupported recorder event")
	}
	return validateRecordPayload(event)
}

func validateOmission(event Event) error {
	if event.Version == 0 && !event.Complete && event.Code == 2193 && event.Reason == SuccessProbabilityOmission && event.Array == "" && len(event.Values) == 0 {
		return nil
	}
	return fmt.Errorf("unsupported recorder omission")
}

func validateRecordPayload(event Event) error {
	if n := recordArity(event.Code); n >= 0 && len(event.Values) == n && validPayloadValues(event) {
		return nil
	}
	return fmt.Errorf("unsupported recorder code or arguments: %d", event.Code)
}

func validPayloadValues(event Event) bool {
	if event.Code == 2193 {
		return validProbability(event.Values[0])
	}
	return validRecordValues(event)
}

func validRecordValues(event Event) bool {
	if event.Code == 2190 || event.Code == 2191 {
		return validInitialValues(event.Values)
	}
	for _, value := range event.Values {
		if event.Code == 2186 {
			var ok bool
			value, ok = strings.CutSuffix(value, "ms")
			if !ok {
				return false
			}
		}
		if !decimal(value) {
			return false
		}
	}
	return true
}

var probabilityPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?(E-?[0-9]+)?$`)

func validProbability(value string) bool {
	value, ok := strings.CutPrefix(value, "val = ")
	probability, err := strconv.ParseFloat(value, 64)
	return ok && probabilityPattern.MatchString(value) && err == nil && !math.IsInf(probability, 0)
}

func validInitialValues(values []string) bool {
	if !decimal(values[0]) {
		return false
	}
	plural := "s"
	if values[0] == "1" {
		plural = ""
	}
	if values[1] != plural {
		return false
	}
	return len(values) == 2 || decimal(values[2])
}

func decimal(s string) bool {
	if s == "" || len(s) > 1 && s[0] == '0' {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func recordArity(code int) int {
	switch code {
	case 2185, 2189, 2219, 2220:
		return 0
	case 2190:
		return 2
	case 2191, 2199:
		return 3
	case 2193, 2194, 2269, 2186:
		return 1
	default:
		return -1
	}
}

func validateHeader(event Event) error {
	if event.Kind != "header" || event.Version != 1 || event.Complete || event.Code != 0 || event.Array != "" || len(event.Values) != 0 {
		return fmt.Errorf("invalid or complete recorder header")
	}
	return nil
}
