package pcal

import "strconv"

// tokState values are the TLA-mode states of Java's Tokenize class.
type tokState int

const (
	stStart tokState = iota
	stID
	stNumOrID
	stBS
	stNum
	stNumOrBI
	stBSBuiltIn
	stBuiltIn
	stDash1
	stDash2
	stDash3
	stDashes
	stEq1
	stEq2
	stEq3
	stEqs
	stLeftParen
	stString
	stEscString
	stLineComment
	stLineComParen
	stLineComStar
	stComment
	stCommentStar
	stCommentParen
	stOrComment
	stOrCommentParen
	stOrCommentStar
	stDone
)

// Token kinds that are not expression tokens, from tla2tex.Token.
const (
	tokDashes    tokenType = 6
	tokEndModule tokenType = 7
)

// tokenizer ports Tokenize. Like the Java class, the delimiter fields and the
// comment and module depths persist across calls.
type tokenizer struct {
	r              *charReader
	delimiter      string
	delimiterLine  int
	delimiterCol   int
	vspec          [][]*token
	linev          []*token
	nextChar       uint16
	lineCorrection int
	tok            []uint16
	parenDepth     int
	inQuantifier   bool
	cdepth, mdepth int
	col, ncol      int
	state          tokState
	exprEnd        bool
	parseExpr      bool
	prevToken      string
}

var tokHandlers [stDone]func(*tokenizer)

func init() {
	tokHandlers = [stDone]func(*tokenizer){
		stStart: (*tokenizer).start, stID: (*tokenizer).ident, stNumOrID: (*tokenizer).numOrID,
		stBS: (*tokenizer).backslash, stNum: (*tokenizer).number, stNumOrBI: (*tokenizer).numOrBuiltIn,
		stBSBuiltIn: (*tokenizer).bsBuiltIn, stBuiltIn: (*tokenizer).builtIn,
		stDash1: dashOrBuiltIn('-', stDash2), stDash2: dashOrBuiltIn('-', stDash3),
		stDash3: repeatOrError('-', stDashes), stDashes: repeatOrOut('-', tokDashes),
		stEq1: dashOrBuiltIn('=', stEq2), stEq2: dashOrBuiltIn('=', stEq3),
		stEq3: repeatOrError('=', stEqs), stEqs: (*tokenizer).equals,
		stLeftParen: (*tokenizer).leftParen, stString: (*tokenizer).inString, stEscString: (*tokenizer).escString,
		stLineComment: (*tokenizer).lineComment, stLineComParen: lineCommentDelim('*', 1, "("),
		stLineComStar: lineCommentDelim(')', -1, "*"),
		stComment:     comment(stCommentStar, stCommentParen, "Input ended in the middle of a comment"),
		stCommentStar: commentStar(stComment), stCommentParen: commentParen(stComment),
		stOrComment:      comment(stOrCommentStar, stOrCommentParen, "Input ended in the middle of a multi-line comment"),
		stOrCommentStar:  commentStar(stOrComment),
		stOrCommentParen: commentParen(stOrComment),
	}
}

// tokenizeExpr reads an expression and leaves the following token in the
// delimiter fields.
func (t *tokenizer) tokenizeExpr() *expr { return t.run(true) }

// algorithmToken reads one token of the algorithm's own syntax.
func (t *tokenizer) algorithmToken() string {
	t.run(false)
	return t.delimiter
}

func (t *tokenizer) run(isExpr bool) *expr {
	t.ncol = t.r.columnNumber()
	t.col = t.ncol
	t.parseExpr = isExpr
	t.prevToken = " "
	t.vspec = nil
	t.linev = nil
	t.tok = nil
	t.readChar()
	t.parenDepth = 0
	t.inQuantifier = false
	t.state = stStart
	t.exprEnd = false
	for t.state != stDone && !t.exprEnd {
		tokHandlers[t.state](t)
	}
	if t.nextChar != '\n' {
		t.r.backspace()
	}
	return newExpr(t.vspec)
}

func (t *tokenizer) readChar() {
	t.nextChar = t.r.next()
	t.lineCorrection = 0
	if t.nextChar == '\n' {
		t.lineCorrection = -1
	}
}

func (t *tokenizer) skipNextChar() {
	t.ncol = t.r.columnNumber()
	t.readChar()
}

func (t *tokenizer) addNextChar() {
	t.tok = append(t.tok, t.nextChar)
	t.skipNextChar()
}

func (t *tokenizer) gotoStart() {
	t.state = stStart
	t.col = t.ncol
}

func (t *tokenizer) text() string { return fromUnits(t.tok) }

func (t *tokenizer) setText(s string) { t.tok = toUnits(s) }

func (t *tokenizer) startNewLine() {
	t.vspec = append(t.vspec, t.linev)
	t.linev = nil
	t.col = 0
}

func (t *tokenizer) fail(msg string) {
	fail(msg + " `" + t.text() + "' found at\n    line " + strconv.Itoa(t.r.lineNumber()+1) +
		", column " + strconv.Itoa(t.col+1))
}

// out handles a complete token, as Tokenize.TokenOut does.
func (t *tokenizer) out(typ tokenType) {
	if !t.parseExpr {
		t.setDelimiter()
		t.tok = nil
		return
	}
	if typ < tokBuiltin || typ > tokIdent {
		t.fail("Illegal token in an expression")
	}
	if t.endsExpression(typ) {
		t.endExpression()
		return
	}
	t.addExprToken(typ)
}

func (t *tokenizer) setDelimiter() {
	t.exprEnd = true
	t.delimiter = t.text()
	t.delimiterCol = t.col
	t.delimiterLine = t.r.lineNumber()
	if t.nextChar == '\n' {
		t.delimiterLine--
	}
}

// endsExpression reports whether the current token follows the expression.
func (t *tokenizer) endsExpression(typ tokenType) bool {
	if typ == tokString {
		return false
	}
	s := t.text()
	if isDelimiter(s) && !(typ == tokIdent && selectorTokens[t.prevToken]) {
		return true
	}
	return t.parenDepth == 0 && t.closes(s)
}

// closes reports whether s closes the context of the expression.
func (t *tokenizer) closes(s string) bool {
	return s == ")" || s == "}" || (s == "," && !t.inQuantifier)
}

// selectorTokens can precede an identifier that is a record field, which
// may be a PlusCal keyword such as "end".
var selectorTokens = map[string]bool{".": true, "[": true, ",": true}

func (t *tokenizer) endExpression() {
	if t.parenDepth != 0 {
		t.fail("Expression with an unmatched (, [, {, or << followed by")
	}
	if t.inQuantifier {
		t.fail("Expression with \\A or \\E but no following `:` followed by")
	}
	t.setDelimiter()
	if len(t.linev) > 0 {
		t.startNewLine()
	}
}

func (t *tokenizer) addExprToken(typ tokenType) {
	s := t.text()
	if typ == tokBuiltin {
		t.trackParens(s)
	}
	t.prevToken = s
	if typ == tokString {
		t.prevToken = " "
	}
	t.trackQuantifier(s)
	if s != "" || typ == tokString {
		line := t.r.lineNumber() + t.lineCorrection
		tk := newToken(s, t.col, typ)
		tk.source = &region{location{line, t.col}, location{line, t.col + jlen(s)}}
		t.linev = append(t.linev, tk)
	}
	t.tok = nil
}

// trackQuantifier notes an unbounded or bounded quantifier whose ":" has
// not been seen, in which "," does not end the expression.
func (t *tokenizer) trackQuantifier(s string) {
	if t.parenDepth != 0 {
		return
	}
	if s == "\\A" || s == "\\E" {
		t.inQuantifier = true
	}
	if t.inQuantifier && s == ":" {
		t.inQuantifier = false
	}
}

func (t *tokenizer) trackParens(s string) {
	switch parenSymbols[s] {
	case leftParen:
		t.parenDepth++
	case rightParen:
		t.parenDepth--
		if t.parenDepth < 0 {
			t.fail("Extra (unmatching)")
		}
	}
}

var delimiters = map[string]bool{
	";": true, "do": true, "then": true, ":=": true, "begin": true, "variable": true,
	"variables": true, "||": true, "end": true, "else": true, "elsif": true, "if": true,
	"either": true, "or": true, "while": true, "with": true, "call": true, "return": true,
	"goto": true, "print": true, "assert": true, "skip": true, "procedure": true,
	"define": true, "process": true, "fair": true,
}

func isDelimiter(s string) bool { return delimiters[s] }
