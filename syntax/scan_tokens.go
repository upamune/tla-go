package syntax

import (
	"strings"
	"unicode/utf8"
)

func (s *scanner) scanString() error {
	line, column := s.line, s.column()
	s.advance(1)
	var value strings.Builder
	for s.pos < len(s.source) && s.rest()[0] != '"' {
		if s.rest()[0] == '\n' {
			return s.fail("Unterminated string")
		}
		if s.rest()[0] == '\\' {
			if err := s.stringEscape(&value); err != nil {
				return err
			}
			continue
		}
		r, n := utf8.DecodeRuneInString(s.rest())
		value.WriteRune(r)
		s.advance(n)
	}
	if s.pos >= len(s.source) {
		return s.fail("Unterminated string")
	}
	s.advance(1)
	s.tokens = append(s.tokens, Token{"String", value.String(), line, column, s.column() - 1})
	return nil
}

func (s *scanner) stringEscape(value *strings.Builder) error {
	s.advance(1)
	if s.pos >= len(s.source) {
		return s.fail("Unterminated string")
	}
	r, n := utf8.DecodeRuneInString(s.rest())
	escapes := map[rune]rune{'n': '\n', 't': '\t', 'r': '\r', 'f': '\f'}
	if replacement, ok := escapes[r]; ok {
		r = replacement
	}
	value.WriteRune(r)
	s.advance(n)
	return nil
}

func baseDigit(c, base byte) bool {
	digits := "0123456789abcdefABCDEF"
	switch base {
	case 'b', 'B':
		digits = "01"
	case 'o', 'O':
		digits = "01234567"
	}
	return strings.ContainsRune(digits, rune(c))
}

func (s *scanner) number() bool {
	text := s.rest()
	if len(text) < 3 || text[0] != '\\' || !strings.ContainsRune("bBoOhH", rune(text[1])) || !baseDigit(text[2], text[1]) {
		return false
	}
	n := 2
	for n < len(text) && n < 64 && baseDigit(text[n], text[1]) {
		n++
	}
	s.push("Number", text[:n], n)
	return true
}

func (s *scanner) word() bool {
	text := s.rest()
	if !identifier(text[0]) {
		return false
	}
	n := 0
	for n < len(text) && identifier(text[n]) {
		n++
	}
	word := text[:n]
	if allDigits(word) {
		s.decimal(n)
		return true
	}
	if strings.HasPrefix(word, "WF_") || strings.HasPrefix(word, "SF_") {
		s.push("Keyword", word[:3], 3)
		return true
	}
	kind := "Identifier"
	if keywords[word] {
		kind = "Keyword"
	}
	s.push(kind, word, n)
	return true
}

func allDigits(word string) bool {
	for i := range len(word) {
		if !digit(word[i]) {
			return false
		}
	}
	return true
}

func (s *scanner) decimal(n int) {
	text := s.rest()
	kind := "Number"
	if n+1 < len(text) && text[n] == '.' && digit(text[n+1]) {
		n += 2
		for n < len(text) && digit(text[n]) {
			n++
		}
		kind = "Decimal"
	}
	s.push(kind, text[:n], n)
}

func stepLevel(text string) int {
	end := 1
	if end < len(text) && (text[end] == '+' || text[end] == '*') {
		return end + 1
	}
	for end < len(text) && digit(text[end]) {
		end++
	}
	return end
}

func isStepDash(text string, start, end int) bool {
	return end == start && end < len(text) && strings.ContainsRune("*-", rune(text[end]))
}

func stepName(text string, end int) int {
	start := end
	for end < len(text) && identifier(text[end]) {
		end++
	}
	if isStepDash(text, start, end) {
		end++
	}
	for end < len(text) && text[end] == '.' {
		end++
	}
	return end
}

func (s *scanner) step() bool {
	text := s.rest()
	if text[0] != '<' {
		return false
	}
	end := stepLevel(text)
	if end == 1 || end >= len(text) || text[end] != '>' {
		return false
	}
	end = stepName(text, end+1)
	s.push("Step", text[:end], end)
	return true
}

func (s *scanner) operator() bool {
	text, n := MatchOperator(s.rest())
	if n == 0 {
		return false
	}
	s.push("Operator", text, n)
	return true
}
