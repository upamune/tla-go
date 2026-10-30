// Package syntax implements the TLA+ lexical and syntactic front end.
// Tokenization follows tla-ts; see UPSTREAM-LICENSE.
package syntax

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Token positions are one-based, with tabs expanded to eight columns. Columns
// match tla-ts, which measures Unicode characters in UTF-16 code units.
type Token struct {
	Kind      string `json:"kind"`
	Text      string `json:"text"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndColumn int    `json:"endColumn"`
}
type SyntaxError struct {
	Reason       string
	Line, Column int
	IsEnd        bool
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("%s at line %d, column %d", e.Reason, e.Line, e.Column)
}

var moduleHeader = regexp.MustCompile(`-{4,}[\t ]*MODULE\b`)

type scanner struct {
	source                  string
	pos, line, width, depth int
	tokens                  []Token
}

func (s *scanner) rest() string { return s.source[s.pos:] }
func (s *scanner) column() int {
	if s.pos < len(s.source) && s.source[s.pos] == '\t' {
		return s.width + 8 - s.width%8
	}
	return s.width + 1
}

func (s *scanner) advance(n int) {
	end := s.pos + n
	for s.pos < end {
		r, size := utf8.DecodeRuneInString(s.source[s.pos:end])
		s.pos += size
		s.advanceRune(r)
	}
}

func (s *scanner) advanceRune(r rune) {
	switch {
	case r == '\n':
		s.line++
		s.width = 0
	case r == '\t':
		s.width += 8 - s.width%8
	case r > 0xffff:
		s.width += 2
	default:
		s.width++
	}
}

func units(text string) int {
	n := 0
	for _, r := range text {
		n++
		if r > 0xffff {
			n++
		}
	}
	return n
}

func (s *scanner) push(kind, text string, n int) {
	c := s.column()
	s.tokens = append(s.tokens, Token{kind, text, s.line, c, c + units(s.rest()[:n]) - 1})
	s.advance(n)
}

func (s *scanner) fail(reason string) error {
	return &SyntaxError{reason, s.line, s.column(), s.pos >= len(s.source)}
}

// Tokenize skips leading text before the module header and stops at the end of
// the outermost module, as the upstream lexer does.
func Tokenize(source string) ([]Token, error) {
	loc := moduleHeader.FindStringIndex(source)
	if loc == nil {
		return nil, fmt.Errorf("Expected a module header such as ---- MODULE Name ----")
	}
	s := scanner{source: source, line: 1}
	s.advance(loc[0])
	for s.pos < len(source) {
		stop, err := s.scan()
		if err != nil {
			return nil, err
		}
		if stop {
			break
		}
	}
	c := s.column()
	s.tokens = append(s.tokens, Token{"End", "", s.line, c, c})
	return s.tokens, nil
}

func (s *scanner) scan() (bool, error) {
	if s.whitespace() {
		return false, nil
	}
	if ok, err := s.comment(); ok {
		return false, err
	}
	if s.separator() {
		return s.depth <= 0, nil
	}
	if s.rest()[0] == '"' {
		return false, s.scanString()
	}
	if s.scanToken() {
		return false, nil
	}
	r, _ := utf8.DecodeRuneInString(s.rest())
	return false, s.fail(fmt.Sprintf("Unexpected character %q", string(r)))
}

func (s *scanner) scanToken() bool { return s.number() || s.word() || s.step() || s.operator() }
func (s *scanner) whitespace() bool {
	if !strings.ContainsRune(" \t\r\n", rune(s.rest()[0])) {
		return false
	}
	s.advance(1)
	return true
}

func (s *scanner) comment() (bool, error) {
	if strings.HasPrefix(s.rest(), "\\*") {
		n := strings.IndexByte(s.rest(), '\n')
		if n < 0 {
			n = len(s.rest())
		}
		s.advance(n)
		return true, nil
	}
	if !strings.HasPrefix(s.rest(), "(*") {
		return false, nil
	}
	return true, s.blockComment()
}

func (s *scanner) blockComment() error {
	depth := 0
	for s.pos < len(s.source) {
		switch {
		case strings.HasPrefix(s.rest(), "(*"):
			depth++
			s.advance(2)
		case strings.HasPrefix(s.rest(), "*)"):
			depth--
			s.advance(2)
			if depth == 0 {
				return nil
			}
		default:
			_, n := utf8.DecodeRuneInString(s.rest())
			s.advance(n)
		}
	}
	return s.fail("Unterminated comment")
}

func runLength(source string, c byte) int {
	i := 0
	for i < len(source) && source[i] == c {
		i++
	}
	return i
}

func (s *scanner) separator() bool {
	if strings.HasPrefix(s.rest(), "----") {
		n := runLength(s.rest(), '-')
		if strings.HasPrefix(strings.TrimLeft(s.rest()[n:], " \t\r\n"), "MODULE") {
			s.depth++
		}
		s.push("Separator", "----", n)
		return true
	}
	if strings.HasPrefix(s.rest(), "====") {
		n := runLength(s.rest(), '=')
		s.push("ModuleEnd", "====", n)
		s.depth--
		return true
	}
	return false
}
