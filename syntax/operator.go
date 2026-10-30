package syntax

import (
	"strings"
	"unicode/utf8"
)

func letter(c byte) bool     { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func digit(c byte) bool      { return c >= '0' && c <= '9' }
func identifier(c byte) bool { return letter(c) || digit(c) || c == '_' }

// MatchOperator recognizes a TLA+ operator at the start of source. The length
// returned is in bytes; aliases are normalized as in tla-ts.
func MatchOperator(source string) (text string, length int) {
	if source == "" {
		return "", 0
	}
	if source[0] == '\\' && len(source) > 1 && letter(source[1]) {
		return matchBackslash(source)
	}
	if source[0] >= utf8.RuneSelf {
		r, size := utf8.DecodeRuneInString(source)
		if op, ok := unicodeOperators[string(r)]; ok {
			return op, size
		}
	}
	return matchSymbol(source)
}

var operatorsByFirst = groupOperators()

func groupOperators() [256][]string {
	var groups [256][]string
	for _, op := range symbolOperators {
		groups[op[0]] = append(groups[op[0]], op)
	}
	return groups
}

func matchSymbol(source string) (string, int) {
	for _, op := range operatorsByFirst[source[0]] {
		if strings.HasPrefix(source, op) {
			if alias, ok := symbolAliases[op]; ok {
				return alias, len(op)
			}
			return op, len(op)
		}
	}
	if source[0] == '\\' {
		return "\\", 1
	}
	return "", 0
}

func matchBackslash(source string) (string, int) {
	end := 1
	for end < len(source) && letter(source[end]) {
		end++
	}
	for n := end; n > 1; n-- {
		if op, ok := backslashOperators[source[1:n]]; ok {
			return op, n
		}
	}
	return "\\", 1
}
