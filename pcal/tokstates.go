package pcal

import "strings"

// startRule describes how the START state handles a class of characters.
type startRule struct {
	match func(uint16) bool
	add   bool
	next  tokState
}

func is(c uint16) func(uint16) bool { return func(d uint16) bool { return c == d } }

var startRules = []startRule{
	{isSpace, false, stStart},
	{isLetter, true, stID},
	{isDigit, true, stNumOrID},
	{is('\\'), true, stBS},
	{is('-'), true, stDash1},
	{is('='), true, stEq1},
	{is('('), false, stLeftParen},
	{is('"'), false, stString},
	{is('\n'), false, stStart},
	{func(c uint16) bool { return isBuiltInPrefix(fromUnits([]uint16{c})) }, true, stBuiltIn},
}

func (t *tokenizer) start() {
	c := t.nextChar
	for _, rule := range startRules {
		if rule.match(c) {
			t.applyStart(rule)
			return
		}
	}
	if c == endOfInput {
		t.state = stDone
		return
	}
	t.addNextChar()
	t.fail("Illegal lexeme")
}

func (t *tokenizer) applyStart(rule startRule) {
	newline := t.nextChar == '\n'
	if rule.add {
		t.addNextChar()
	} else {
		t.skipNextChar()
	}
	if newline {
		t.startNewLine()
	}
	if rule.next == stStart {
		t.gotoStart()
		return
	}
	t.state = rule.next
}

// emit outputs the current token and returns to the START state.
func (t *tokenizer) emit(typ tokenType) {
	t.out(typ)
	t.gotoStart()
}

func (t *tokenizer) ident() {
	s := t.text()
	switch {
	case s == "WF_" || s == "SF_":
		t.emit(tokBuiltin)
	case isLetter(t.nextChar) || isDigit(t.nextChar):
		t.addNextChar()
	case isBuiltInSymbol(s):
		if s == "MODULE" {
			t.mdepth++
		}
		t.emit(tokBuiltin)
	default:
		t.emit(tokIdent)
	}
}

func (t *tokenizer) numOrID() {
	switch {
	case isDigit(t.nextChar):
		t.addNextChar()
	case isLetter(t.nextChar):
		t.addNextChar()
		t.state = stID
	default:
		t.emit(tokNumber)
	}
}

func (t *tokenizer) backslash() {
	c := t.nextChar
	switch {
	case strings.ContainsRune("bBoOhH", rune(c)):
		t.addNextChar()
		t.state = stNumOrBI
	case isLetter(c):
		t.state = stBSBuiltIn
	case c == '*':
		t.skipNextChar()
		t.tok = nil
		t.state = stLineComment
	default:
		t.state = stBuiltIn
	}
}

func (t *tokenizer) numOrBuiltIn() {
	t.state = stBSBuiltIn
	if isDigit(t.nextChar) {
		t.state = stNum
	}
}

func (t *tokenizer) number() {
	switch {
	case isDigit(t.nextChar):
		t.addNextChar()
	case isLetter(t.nextChar):
		t.addNextChar()
		if t.tok[0] == '\\' {
			t.fail("Illegal lexeme")
		}
		t.state = stID
	default:
		t.emit(tokNumber)
	}
}

func (t *tokenizer) bsBuiltIn() {
	switch {
	case isLetter(t.nextChar) && t.nextChar != '_':
		t.addNextChar()
	case isBuiltInSymbol(t.text()):
		t.emit(tokBuiltin)
	default:
		t.fail("Illegal lexeme ")
	}
}

func (t *tokenizer) builtIn() {
	if isBuiltInPrefix(t.text() + fromUnits([]uint16{t.nextChar})) {
		t.addNextChar()
		return
	}
	if !isBuiltInSymbol(t.text()) {
		t.backUpToSymbol()
	}
	t.emit(tokBuiltin)
}

// backUpToSymbol shortens the token to its longest prefix that is a symbol,
// returning the removed characters to the reader.
func (t *tokenizer) backUpToSymbol() {
	t.r.backspace()
	for !isBuiltInSymbol(t.text()) {
		t.r.backspace()
		if len(t.tok) == 0 {
			t.fail("Illegal lexeme")
		}
		t.tok = t.tok[:len(t.tok)-1]
	}
	t.skipNextChar()
}

// dashOrBuiltIn handles the states that read the start of "----" or "====".
func dashOrBuiltIn(c uint16, next tokState) func(*tokenizer) {
	return func(t *tokenizer) {
		if t.nextChar != c {
			t.state = stBuiltIn
			return
		}
		t.addNextChar()
		t.state = next
	}
}

func repeatOrError(c uint16, next tokState) func(*tokenizer) {
	return func(t *tokenizer) {
		if t.nextChar != c {
			t.fail("Illegal lexeme")
		}
		t.addNextChar()
		t.state = next
	}
}

func repeatOrOut(c uint16, typ tokenType) func(*tokenizer) {
	return func(t *tokenizer) {
		if t.nextChar == c {
			t.addNextChar()
			return
		}
		t.emit(typ)
	}
}

func (t *tokenizer) equals() {
	if t.nextChar == '=' {
		t.addNextChar()
		return
	}
	t.mdepth--
	t.emit(tokEndModule)
}

func (t *tokenizer) leftParen() {
	if t.nextChar == '*' {
		t.skipNextChar()
		t.cdepth = 1
		t.state = stComment
		return
	}
	t.setText("(")
	t.state = stBuiltIn
}

func (t *tokenizer) inString() {
	switch c := t.nextChar; {
	case c == '\\':
		t.addNextChar()
		t.state = stEscString
	case c == '"':
		t.skipNextChar()
		t.emit(tokString)
	case isStringChar(c):
		t.addNextChar()
	default:
		t.addNextChar()
		t.fail("Illegal character in string")
	}
}

func (t *tokenizer) escString() {
	ok := strings.ContainsRune("\"\\tnfr", rune(t.nextChar))
	t.addNextChar()
	if !ok {
		t.fail("Illegal character following \\ in string")
	}
	t.state = stString
}

func (t *tokenizer) lineComment() {
	switch c := t.nextChar; {
	case c == '(':
		t.skipNextChar()
		t.state = stLineComParen
	case c == '*' && t.cdepth > 0:
		t.skipNextChar()
		t.state = stLineComStar
	case c == '\n' || c == endOfInput:
		t.tok = nil
		t.cdepth = 0
		t.gotoStart()
	case t.cdepth == 0:
		t.addNextChar()
	default:
		t.skipNextChar()
	}
}

// lineCommentDelim handles the character after "(" or "*" in a line comment.
func lineCommentDelim(c uint16, delta int, literal string) func(*tokenizer) {
	return func(t *tokenizer) {
		t.state = stLineComment
		if t.nextChar == c {
			t.skipNextChar()
			t.cdepth += delta
			return
		}
		if t.cdepth == 0 {
			t.tok = append(t.tok, toUnits(literal)...)
		}
	}
}

// comment handles the body of a (* *) comment.
func comment(star, paren tokState, eofMsg string) func(*tokenizer) {
	return func(t *tokenizer) {
		switch t.nextChar {
		case '*':
			t.skipNextChar()
			t.state = star
		case '(':
			t.skipNextChar()
			t.state = paren
		case '\n':
			t.tok = nil
			t.skipNextChar()
			t.startNewLine()
			t.state = stOrComment
		case endOfInput:
			fail(eofMsg)
		default:
			t.commentChar()
		}
	}
}

func (t *tokenizer) commentChar() {
	if t.cdepth == 1 {
		t.addNextChar()
		return
	}
	t.skipNextChar()
}

func commentStar(back tokState) func(*tokenizer) {
	return func(t *tokenizer) {
		t.state = back
		if t.nextChar != ')' {
			t.appendAtTopLevel("*")
			return
		}
		t.skipNextChar()
		t.cdepth--
		if t.cdepth == 0 {
			t.tok = nil
			t.gotoStart()
		}
	}
}

func commentParen(back tokState) func(*tokenizer) {
	return func(t *tokenizer) {
		t.state = back
		if t.nextChar != '*' {
			t.appendAtTopLevel("(")
			return
		}
		t.skipNextChar()
		t.cdepth++
	}
}

func (t *tokenizer) appendAtTopLevel(s string) {
	if t.cdepth == 1 {
		t.tok = append(t.tok, toUnits(s)...)
	}
}
