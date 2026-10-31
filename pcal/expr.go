package pcal

import (
	"slices"
	"strings"
)

// tokenType values follow TLAToken.
type tokenType int

const (
	tokBuiltin tokenType = 1
	tokNumber  tokenType = 2
	tokString  tokenType = 3
	tokIdent   tokenType = 4
	tokAdded   tokenType = 5
)

// location and region mirror PCalLocation and Region. The translator only
// needs them to reproduce where Java dereferences a missing origin.
type location struct{ line, column int }

type region struct{ begin, end location }

func (r *region) beginLoc() location { return r.begin }

func (r *region) endLoc() location { return r.end }

// token ports TLAToken without its TLA-to-PlusCal mapping fields.
type token struct {
	str    string
	column int
	typ    tokenType
	source *region
}

func newToken(str string, col int, typ tokenType) *token {
	return &token{str: str, column: col, typ: typ}
}

func (t *token) width() int {
	if t.typ == tokString {
		return jlen(t.str) + 2
	}
	return jlen(t.str)
}

func (t *token) clone() *token {
	c := *t
	return &c
}

// expr ports TLAExpr: a sequence of token lines and the anchors used to keep
// their relative indentation.
type expr struct {
	tokens       [][]*token
	anchorTokens []*token
	anchorTokCol []int
	origin       *region
}

func newExpr(lines [][]*token) *expr {
	e := &expr{tokens: lines}
	e.normalize()
	return e
}

func (e *expr) addLine() { e.tokens = append(e.tokens, nil) }

func (e *expr) addToken(t *token) {
	n := len(e.tokens) - 1
	e.tokens[n] = append(e.tokens[n], t)
}

func (e *expr) addTokenOffset(t *token, offset int) {
	n := len(e.tokens) - 1
	col := offset
	if line := e.tokens[n]; len(line) > 0 {
		last := line[len(line)-1]
		col += last.column + jlen(last.str)
	}
	t.column = col
	e.tokens[n] = append(e.tokens[n], t)
}

func (e *expr) isEmpty() bool { return len(e.tokens) == 0 }

func (e *expr) isOneToken() bool { return len(e.tokens) == 1 && len(e.tokens[0]) == 1 }

func (e *expr) firstToken() *token { return e.tokens[0][0] }

func (e *expr) lastToken() *token {
	line := e.tokens[len(e.tokens)-1]
	return line[len(line)-1]
}

// allTokens returns the tokens of all lines in order.
func (e *expr) allTokens() []*token {
	var toks []*token
	for _, line := range e.tokens {
		toks = append(toks, line...)
	}
	return toks
}

func (e *expr) tokenAt(c coord) *token { return e.tokens[c.line][c.col] }

// normalize removes empty first and last lines, shifts the expression to
// column 0, and computes each line's anchor token.
func (e *expr) normalize() {
	for len(e.tokens) > 0 && len(e.tokens[0]) == 0 {
		e.tokens = e.tokens[1:]
	}
	for len(e.tokens) > 0 && len(e.tokens[len(e.tokens)-1]) == 0 {
		e.tokens = e.tokens[:len(e.tokens)-1]
	}
	e.shiftLeft(e.minColumn())
	e.anchorTokens = make([]*token, len(e.tokens))
	e.anchorTokCol = make([]int, len(e.tokens))
	for i := range e.tokens {
		e.setAnchor(i)
	}
}

// minColumn returns the leftmost column at which a line starts.
func (e *expr) minColumn() int {
	minCol := 999999
	for _, line := range e.tokens {
		if len(line) > 0 && line[0].column < minCol {
			minCol = line[0].column
		}
	}
	return minCol
}

func (e *expr) shiftLeft(n int) {
	for _, line := range e.tokens {
		for _, t := range line {
			t.column -= n
		}
	}
}

// setAnchor finds the token of an earlier line under which line i starts.
func (e *expr) setAnchor(i int) {
	line := e.tokens[i]
	if len(line) == 0 {
		return
	}
	first := line[0].column
	for j := i - 1; j >= 0; j-- {
		anc := e.tokens[j]
		if len(anc) == 0 || anc[0].column > first {
			continue
		}
		k := 0
		for k+1 < len(anc) && anc[k+1].column <= first {
			k++
		}
		e.anchorTokens[i] = anc[k]
		e.anchorTokCol[i] = anc[k].column
		return
	}
}

// renormalize moves each line right by the distance its anchor moved.
func (e *expr) renormalize() {
	for i, anc := range e.anchorTokens {
		if anc == nil {
			continue
		}
		k := anc.column - e.anchorTokCol[i]
		e.anchorTokCol[i] = anc.column
		if k < 0 {
			fail("TLAExpr.renormalize() found anchor has moved to left.")
		}
		for _, t := range e.tokens[i] {
			t.column += k
		}
	}
}

// lines returns the expression as text lines, like toStringVector.
func (e *expr) lines() []string {
	result := make([]string, 0, len(e.tokens))
	for i, line := range e.tokens {
		var sb strings.Builder
		if anc := e.anchorTokens[i]; anc != nil {
			sb.WriteString(spaces(anc.column - e.anchorTokCol[i]))
		}
		for j, t := range line {
			if j == 0 {
				sb.WriteString(spaces(t.column))
			} else {
				prev := line[j-1]
				sb.WriteString(spaces(t.column - prev.column - prev.width()))
			}
			sb.WriteString(t.text())
		}
		result = append(result, sb.String())
	}
	return result
}

func (t *token) text() string {
	if t.typ == tokString {
		return `"` + t.str + `"`
	}
	return t.str
}

// String returns the TLA+ sequence-of-strings form used in AST.toString.
func (e *expr) String() string {
	var sb strings.Builder
	sb.WriteString("<< ")
	nonempty := false
	for i, line := range e.tokens {
		if i > 0 {
			sb.WriteString("\n")
		}
		for _, t := range line {
			if nonempty {
				sb.WriteString(", ")
			}
			nonempty = true
			sb.WriteString(t.quoted())
		}
	}
	sb.WriteString(" >>")
	return sb.String()
}

func (t *token) quoted() string {
	switch {
	case t.typ == tokString:
		return `"\"", "` + t.str + `", "\""`
	case t.str[0] == '\\':
		return `"\` + t.str + `"`
	case t.str == `/\`:
		return `"/\\"`
	}
	return `"` + t.str + `"`
}

func (e *expr) cloneAndNormalize() *expr {
	lines := make([][]*token, len(e.tokens))
	for i, line := range e.tokens {
		lines[i] = make([]*token, len(line))
		for j, t := range line {
			lines[i][j] = t.clone()
		}
	}
	result := &expr{tokens: lines, origin: e.origin}
	result.normalize()
	return result
}

// prepend puts x in front of e, with the given number of spaces between the
// last token of x and the first token of e.
func (e *expr) prepend(x *expr, gap int) {
	n := len(x.tokens)
	oldAnchors, oldCols := e.anchorTokens, e.anchorTokCol
	e.tokens = slices.Insert(e.tokens, 0, x.tokens[:n-1]...)
	xLine, eLine := x.tokens[n-1], e.tokens[n-1]
	last := xLine[len(xLine)-1]
	incr := last.column + last.width() + gap
	for _, t := range eLine {
		t.column += incr
	}
	e.tokens[n-1] = append(slices.Clone(xLine), eLine...)
	e.anchorTokens = make([]*token, len(e.tokens))
	e.anchorTokCol = make([]int, len(e.tokens))
	copy(e.anchorTokens, x.anchorTokens[:n])
	copy(e.anchorTokCol, x.anchorTokCol[:n])
	for j := n; j < len(e.tokens); j++ {
		e.anchorTokens[j] = oldAnchors[j-n+1]
		e.anchorTokCol[j] = oldCols[j-n+1]
	}
	e.renormalize()
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

// exprOf builds a normalized one-line expression from tokens whose columns
// are already set.
func exprOf(toks ...*token) *expr {
	return newExpr([][]*token{toks})
}
