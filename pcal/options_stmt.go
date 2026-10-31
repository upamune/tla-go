package pcal

import (
	"strings"
	"unicode/utf8"
)

// loc is a <line, column> position in the input lines, like IntPair.
type loc struct{ line, col int }

// gotoNextNonSpace moves l to the next non-space character and returns its
// line, or "" at the end of the input.
func gotoNextNonSpace(lines []string, l *loc) string {
	for l.line < len(lines) {
		line := lines[l.line]
		for l.col < len(line) && line[l.col] == ' ' {
			l.col++
		}
		if l.col < len(line) {
			return line
		}
		l.line++
		l.col = 0
	}
	return ""
}

// isIDRune reports whether r continues an identifier, as NextNonIdChar
// decides for its first char.
func isIDRune(r rune) bool { return isJavaLetter(r) || isJavaDigit(r) || r == '_' }

// findToken moves l past the next occurrence of token that is not followed
// by a letter or underscore, and reports whether it was found. Like Java, it
// looks only at the first char of the following character.
func findToken(token string, lines []string, l *loc) bool {
	for l.line < len(lines) {
		line := gotoNextNonSpace(lines, l)
		rest := line[l.col:]
		if strings.HasPrefix(rest, token) {
			r, _ := utf8.DecodeRuneInString(rest[len(token):])
			if len(rest) == len(token) || !(isJavaLetter(r) || r == '_') {
				l.col += len(token)
				l.col = nextIndexAny(line, l.col, " ")
				return true
			}
		}
		l.col = nextIndexAny(line, l.col, " ")
	}
	return false
}

// nextIndexAny returns the first index at or after col of a character in
// chars, or len(s).
func nextIndexAny(s string, col int, chars string) int {
	if i := strings.IndexAny(s[col:], chars); i >= 0 {
		return col + i
	}
	return len(s)
}

// startsIdentifier reports whether s begins with word as a whole
// identifier.
func startsIdentifier(s, word string) bool {
	if !strings.HasPrefix(s, word) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s[len(word):])
	return len(s) == len(word) || !isIDRune(r)
}

// processOptionsStatement finds "PlusCal options (...)" and applies the
// options it lists, as trans.performTranslation does.
func (t *translation) processOptionsStatement(lines []string) {
	l := &loc{}
	for findToken("PlusCal", lines, l) {
		line := gotoNextNonSpace(lines, l)
		if startsIdentifier(line[min(l.col, len(line)):], "options") {
			t.p.optionsInFile = true
			t.processOptions(lines, l)
			return
		}
	}
}

// processOptions ports ParseAlgorithm.ProcessOptions. Like Java, the caller
// ignores the result of processing the options.
func (t *translation) processOptions(lines []string, l *loc) {
	l.col += len("options")
	line := gotoNextNonSpace(lines, l)
	if !strings.HasPrefix(line[l.col:], "(") {
		t.log.error("`PlusCal options' not followed by '('")
		return
	}
	l.col++
	line = gotoNextNonSpace(lines, l)
	var args []string
	for l.line < len(lines) && line[l.col] != ')' {
		if line[l.col] == ',' {
			l.col++
		} else {
			end := nextIndexAny(line, l.col, " ,)")
			args = append(args, line[l.col:end])
			l.col = end
		}
		line = gotoNextNonSpace(lines, l)
	}
	if l.line >= len(lines) {
		t.log.error("No closing ')' found in options statement")
		return
	}
	t.p.parseArgs(append(args, ""), t.log)
}
