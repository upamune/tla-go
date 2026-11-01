package upstreamjunit

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The Java recorder keeps the runtime type of the varargs array itself: a
// String[] is observably different from an Object[] containing strings.
type recorderValue struct {
	kind string
	text string
}

type recorderRecord struct {
	code, array string
	values      []recorderValue
}

func encodeBridge(records []bridgeRecord) (string, error) {
	var typed []recorderRecord
	for _, record := range records {
		item := recorderRecord{code: record.code, array: "strings"}
		for _, v := range record.values {
			item.values = append(item.values, recorderValue{"s", v})
		}
		typed = append(typed, item)
	}
	return encodeRecorder(typed)
}

// Unsupported values fail closed; no lossy String() conversion is allowed.
func encodeRecorder(records []recorderRecord) (string, error) {
	var b strings.Builder
	for _, record := range records {
		if !recorderCodeSupported(record.code) || record.array != "strings" && record.array != "objects" {
			return "", fmt.Errorf("unsupported recorder code %q or array %q", record.code, record.array)
		}
		b.WriteString(record.code + "\t" + record.array)
		for _, value := range record.values {
			encoded, err := encodeRecorderValue(value, record.array)
			if err != nil {
				return "", err
			}
			b.WriteByte('\t')
			b.WriteString(encoded)
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func recorderCodeSupported(code string) bool {
	switch code {
	case "1000", "2186", "2190", "2194", "2199", "2221", "2772", "2773", "2774", "2775":
		return true
	}
	return false
}

func encodeRecorderValue(value recorderValue, array string) (string, error) {
	if array == "strings" && value.kind != "s" {
		return "", fmt.Errorf("non-string recorder value in String[]")
	}
	switch value.kind {
	case "s":
		return encodeRecorderString(value.text)
	case "i", "l":
		return encodeRecorderInteger(value)
	case "b", "n":
		return encodeRecorderPrimitive(value)
	}
	return "", fmt.Errorf("unsupported recorder value %q", value.kind)
}

func encodeRecorderPrimitive(value recorderValue) (string, error) {
	if value.kind == "b" && (value.text == "true" || value.text == "false") {
		return "b:" + value.text, nil
	}
	if value.kind == "n" && value.text == "" {
		return "n:", nil
	}
	return "", fmt.Errorf("unsupported recorder value %q", value.kind)
}

func encodeRecorderString(value string) (string, error) {
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("unsupported recorder string encoding")
	}
	return "s:" + base64.StdEncoding.EncodeToString([]byte(value)), nil
}

func encodeRecorderInteger(value recorderValue) (string, error) {
	bits := 64
	if value.kind == "i" {
		bits = 32
	}
	n, err := strconv.ParseInt(value.text, 10, bits)
	if err != nil || strconv.FormatInt(n, 10) != value.text {
		return "", fmt.Errorf("unsupported recorder integer %q", value.text)
	}
	return value.kind + ":" + value.text, nil
}
