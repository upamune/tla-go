package pcal

import (
	"slices"
	"strconv"
)

// strList is a shared, mutable list of strings. Java's parser keeps label
// lists in static vectors that remain shared with the last procedure parsed.
type strList struct{ items []string }

func (l *strList) add(s string) { l.items = append(l.items, s) }

func (l *strList) contains(s string) bool {
	return slices.Contains(l.items, s)
}

// parser ports ParseAlgorithm's static state and methods.
type parser struct {
	params *Params
	log    *logger
	r      *charReader
	tz     *tokenizer

	algLine, algCol int

	currentProcedure jstring
	plusLabels       *strList
	minusLabels      *strList
	proceduresCalled *strList

	gotoUsed, gotoDoneUsed bool
	omitPC                 bool
	omitStutteringWhenDone bool
	hasDefaultInit         bool
	hasLabel               bool
	allLabels              map[string]bool
	nextLabelNum           int
	addedLabels            []string
	addedLabelsLocs        []string
	procedures             []*Procedure
	pSyntax, cSyntax       bool
	inGetMacro             bool
	labelLocation          *location

	lat         [10]string
	latSize     int
	curTokCol   [10]int
	curTokLine  [10]int
	lastTokCol  int
	lastTokLine int
	lastTokStr  string
}

func newParser(params *Params, log *logger, r *charReader, algLine, algCol int) *parser {
	p := &parser{
		params: params, log: log, r: r, tz: &tokenizer{r: r}, algLine: algLine, algCol: algCol,
		plusLabels: &strList{}, minusLabels: &strList{}, proceduresCalled: &strList{},
		omitPC: true, omitStutteringWhenDone: true, allLabels: map[string]bool{}, nextLabelNum: 1,
	}
	if params.inputVersion < version15 {
		p.omitPC = false
		p.omitStutteringWhenDone = false
	}
	return p
}

// parse returns the AST of the algorithm, as ParseAlgorithm.getAlgorithm.
func (p *parser) parse(fair bool) (result node, err *Error) {
	err = catch(func() { result = p.algorithm(fair) }, func() *Error {
		return &Error{Message: p.parsingErrorMessage("Unknown error at or before")}
	})
	return result, err
}

func (p *parser) parsingErrorMessage(msg string) string {
	return msg + "\n    line " + strconv.Itoa(p.lastTokLine) + ", column " + strconv.Itoa(p.lastTokCol)
}

func (p *parser) parsingError(msg string) { fail(p.parsingErrorMessage(msg)) }

func (p *parser) lastLocationStart() location {
	return location{p.lastTokLine - 1, p.lastTokCol - 1}
}

func (p *parser) lastLocationEnd() location {
	return location{p.lastTokLine - 1, p.lastTokCol - 1 + jlen(p.lastTokStr)}
}

// getAlgToken consumes and returns the next algorithm token.
func (p *parser) getAlgToken() string {
	if p.latSize == 0 {
		p.r.peek()
		p.lastTokCol = p.r.columnNumber() + 1
		p.lastTokLine = p.r.lineNumber() + 1
		p.lastTokStr = p.tz.algorithmToken()
		return p.lastTokStr
	}
	p.lastTokCol = p.curTokCol[0] + 1
	p.lastTokLine = p.curTokLine[0] + 1
	result := p.lat[0]
	copy(p.lat[:], p.lat[1:p.latSize])
	copy(p.curTokCol[:], p.curTokCol[1:p.latSize])
	copy(p.curTokLine[:], p.curTokLine[1:p.latSize])
	p.latSize--
	p.lastTokStr = result
	return result
}

// peekAt returns the n-th next token without consuming it.
func (p *parser) peekAt(n int) string {
	for p.latSize < n {
		if p.r.peek() == "\t" {
			fail("Premature end of file, perhaps because of unclosed comment, near\n    line " +
				strconv.Itoa(p.curTokLine[p.latSize]+1) + ", column " + strconv.Itoa(p.curTokCol[p.latSize]+1))
		}
		p.curTokCol[p.latSize] = p.r.columnNumber()
		p.curTokLine[p.latSize] = p.r.lineNumber()
		p.lat[p.latSize] = p.tz.algorithmToken()
		p.latSize++
	}
	return p.lat[n-1]
}

// peekPast reads n tokens of lookahead and returns the rest of the line
// after them.
func (p *parser) peekPast(n int) string {
	if n < p.latSize {
		reportBug("ParseAlgorithm.PeekPastAlgToken called to peek after a token\n    it has already peeked at")
	}
	for n > p.latSize {
		p.lat[p.latSize] = p.tz.algorithmToken()
		p.latSize++
	}
	return p.r.peek()
}

func (p *parser) peekIs(n int, toks ...string) bool {
	tok := p.peekAt(n)
	return slices.Contains(toks, tok)
}

// getExpr reads an expression; the token after it becomes the lookahead.
func (p *parser) getExpr() *expr {
	if p.latSize != 0 {
		reportBug("ParseAlgorithm: GetExpr called after lookahead")
	}
	result := p.tz.tokenizeExpr()
	p.lat[p.latSize] = p.tz.delimiter
	p.curTokCol[p.latSize] = p.tz.delimiterCol
	p.curTokLine[p.latSize] = p.tz.delimiterLine
	p.latSize++
	if len(result.tokens) == 0 {
		return result
	}
	lastLine := result.tokens[len(result.tokens)-1]
	if len(lastLine) == 0 {
		reportBug("Unexpected Event in ParseAlgorithm.GetExpr")
	}
	result.origin = &region{result.tokens[0][0].source.begin, lastLine[len(lastLine)-1].source.end}
	return result
}

var optionalSemicolonBefore = map[string]bool{
	"end": true, "begin": true, "else": true, "elsif": true, "or": true, "do": true, "macro": true,
	"procedure": true, "process": true, "fair": true, "define": true, "}": true, "{": true,
}

var statementStarts = map[string]bool{
	"if": true, "either": true, "while": true, "with": true, "call": true, "return": true,
	"goto": true, "print": true, "assert": true, "skip": true,
}

// gobble consumes the token str, which may be an omitted semicolon.
func (p *parser) gobble(str string) {
	if str == ";" {
		nxt := p.peekAt(1)
		if optionalSemicolonBefore[nxt] {
			return
		}
		if statementStarts[nxt] {
			fail("Missing `;' before line " + strconv.Itoa(p.curTokLine[0]+1) + ", column " + strconv.Itoa(p.curTokCol[0]+1))
		}
	}
	p.mustGobble(str)
}

func (p *parser) mustGobble(str string) {
	if tok := p.getAlgToken(); tok != str {
		p.parsingError("Expected \"" + str + "\" but found \"" + tok + "\"")
	}
}

func (p *parser) gobbleEqualOrIn() bool {
	tok := p.getAlgToken()
	switch tok {
	case "=":
		return true
	case "\\in":
		return false
	}
	p.parsingError("Expected \"=\" or \"\\in\"  but found \"" + tok + "\"")
	return false
}

func (p *parser) gobbleCommaOrSemicolon() {
	if p.peekAt(1) == "," {
		p.gobble(",")
		return
	}
	p.gobble(";")
}

func (p *parser) gobbleBegin() {
	switch {
	case p.pSyntax:
		p.gobble("begin")
	case p.cSyntax:
		p.gobble("{")
	default:
		reportBug("Syntax not initialized.")
	}
}

func (p *parser) gobbleEnd(str string) {
	switch {
	case p.pSyntax:
		p.gobble("end")
		p.gobble(str)
	case p.cSyntax:
		p.gobble("}")
	default:
		reportBug("Bad call of GobbleEndRightBrace")
	}
}
