package pcal

import "slices"

// coord is a <line, token> position in an expression, like IntPair.
type coord struct{ line, col int }

func (c coord) before(d coord) bool {
	return c.line < d.line || (c.line == d.line && c.col < d.col)
}

// stepCoord advances c by incr tokens, skipping empty lines. It returns
// false when it moves past the last token.
func (e *expr) stepCoord(c coord, incr int) (coord, bool) {
	if len(e.tokens) <= c.line {
		reportBug("TLAExpr.StepCoord called with line too big")
	}
	line := e.tokens[c.line]
	if len(line) <= c.col {
		reportBug("TLAExpr.StepCoord called with col too big")
	}
	for range incr {
		c.col++
		if len(line) != c.col {
			continue
		}
		c.col = 0
		c.line++
		for c.line < len(e.tokens) && len(e.tokens[c.line]) == 0 {
			c.line++
		}
		if c.line == len(e.tokens) {
			return c, false
		}
		line = e.tokens[c.line]
	}
	return c, true
}

// findNextInstanceIn returns the first occurrence of id at or after start
// that is not a string, a record field, or a field selector.
func (e *expr) findNextInstanceIn(id string, start coord) (coord, bool) {
	if e.isEmpty() {
		return coord{}, false
	}
	c, ok := start, true
	for ok {
		t := e.tokenAt(c)
		skip := e.skipLength(c)
		if skip == 1 && t.typ != tokString && t.str == id {
			return c, true
		}
		c, ok = e.stepCoord(c, skip)
	}
	return coord{}, false
}

// skipLength returns how many tokens findNextInstanceIn steps over at c.
func (e *expr) skipLength(c coord) int {
	t := e.tokenAt(c)
	switch {
	case t.typ == tokString:
		return 1
	case t.str == ".":
		return 2
	case e.isFieldPosition(c):
		return 3
	}
	return 1
}

// isFieldPosition reports whether c holds "[" or "," that begins a record
// field such as "[f |-> e" or ", f : S".
func (e *expr) isFieldPosition(c coord) bool {
	t := e.tokenAt(c)
	if t.typ == tokString || (t.str != "[" && t.str != ",") {
		return false
	}
	after, ok := e.stepCoord(c, 2)
	if !ok {
		return false
	}
	next := e.tokenAt(after)
	return next.typ != tokString && (next.str == ":" || next.str == "|->")
}

// substituteFor replaces every instance of id with x.
func (e *expr) substituteFor(x *expr, id string, parenthesize bool) {
	next, ok := coord{}, true
	for ok {
		next, ok = e.findNextInstanceIn(id, next)
		if ok {
			next, ok = e.substituteAt(x, next, parenthesize)
		}
	}
}

// substituteForAll simultaneously replaces each strs[i] with exprs[i].
func (e *expr) substituteForAll(exprs []*expr, strs []string, parenthesize bool) {
	next := make([]*coord, len(exprs))
	for i := range next {
		next[i] = e.findInstance(strs[i], coord{})
	}
	for {
		idx := earliest(next)
		if idx < 0 {
			return
		}
		at := *next[idx]
		after, _ := e.stepCoord(at, 1)
		newPos, ok := e.substituteAt(exprs[idx], at, parenthesize)
		next[idx] = nil
		if ok {
			next[idx] = e.findInstance(strs[idx], newPos)
		}
		shiftPending(next, idx, at, after, newPos, ok)
	}
}

func (e *expr) findInstance(id string, start coord) *coord {
	c, ok := e.findNextInstanceIn(id, start)
	if !ok {
		return nil
	}
	return &c
}

func earliest(next []*coord) int {
	idx := -1
	for i, c := range next {
		if c != nil && (idx < 0 || c.before(*next[idx])) {
			idx = i
		}
	}
	return idx
}

// shiftPending adjusts the other pending positions after a substitution at
// at, which moved the token following it from after to newPos.
func shiftPending(next []*coord, idx int, at, after, newPos coord, ok bool) {
	for i, c := range next {
		if i != idx && c != nil {
			assertBug(ok, "Doing substitutions in wrong order.")
			shiftCoord(c, at, after, newPos)
		}
	}
}

func shiftCoord(c *coord, at, after, newPos coord) {
	sameLine := c.line == at.line
	moved := after.line != newPos.line
	if moved {
		c.line += newPos.line - after.line
	}
	if !sameLine {
		return
	}
	if moved {
		assertBug(c.col > at.col, "Wrong substitution order.")
	}
	c.col += newPos.col - after.col
	if !moved {
		assertBug(c.col > at.col, "Wrong substitution order.")
	}
}

// substituteAt replaces the token at c with x and returns the position of
// the token that followed it.
func (e *expr) substituteAt(x *expr, c coord, par bool) (coord, bool) {
	tok := e.tokenAt(c)
	if e.isOneToken() {
		cloned := x.cloneAndNormalize()
		e.tokens, e.anchorTokens, e.anchorTokCol = cloned.tokens, cloned.anchorTokens, cloned.anchorTokCol
		return coord{}, false
	}
	tokLine := e.tokens[c.line]
	gap := 0
	if c.col+1 < len(tokLine) {
		gap = tokLine[c.col+1].column - (tok.column + tok.width())
	}
	rest := slices.Clone(tokLine[c.col+1:])
	e.tokens[c.line] = tokLine[:c.col+1]
	cur := c.line
	if x.isOneToken() {
		xt := x.tokens[0][0]
		tok.str, tok.typ, tok.source = xt.str, xt.typ, xt.source
	} else {
		cur = e.insertExpr(x, tok, cur, par)
	}
	line := e.tokens[cur]
	result := coord{cur, len(line) - 1}
	if len(rest) > 0 {
		prev := line[len(line)-1]
		indent := prev.column + prev.width() + gap - rest[0].column
		for _, t := range rest {
			t.column += indent
		}
		e.tokens[cur] = append(line, rest...)
	}
	e.renormalize()
	return e.stepCoord(result, 1)
}

// insertExpr replaces tok, on line cur, by the tokens of a multi-token
// expression and returns the line holding its last token.
func (e *expr) insertExpr(x *expr, tok *token, cur int, par bool) int {
	ins := &inserter{e: e, tok: tok, cur: cur, indent: tok.column, doInsert: !par}
	if par {
		ins.indent++
		tok.str, tok.typ, tok.source = "(", tokBuiltin, nil
	}
	nx := x.cloneAndNormalize()
	for i, eline := range nx.tokens {
		ins.addLine(eline)
		if i+1 < len(nx.tokens) {
			ins.indent = 0
			ins.cur++
			e.insertLine(ins.cur, nx.anchorTokens[i+1], nx.anchorTokCol[i+1])
		}
	}
	if par {
		line := e.tokens[ins.cur]
		last := line[len(line)-1]
		e.tokens[ins.cur] = append(line, newToken(")", last.column+last.width(), tokBuiltin))
	}
	return ins.cur
}

// inserter copies the lines of an expression into another expression.
type inserter struct {
	e        *expr
	tok      *token
	cur      int
	indent   int
	doInsert bool
}

func (ins *inserter) addLine(line []*token) {
	for _, t := range line {
		t.column += ins.indent
		if ins.doInsert {
			ins.tok.str, ins.tok.typ = t.str, t.typ
			ins.doInsert = false
			continue
		}
		ins.e.tokens[ins.cur] = append(ins.e.tokens[ins.cur], t)
	}
}

// insertLine inserts an empty token line with the given anchor at index at.
func (e *expr) insertLine(at int, anchor *token, anchorCol int) {
	e.tokens = slices.Insert(e.tokens, at, []*token(nil))
	e.anchorTokens = slices.Insert(e.anchorTokens, at, anchor)
	e.anchorTokCol = slices.Insert(e.anchorTokCol, at, anchorCol)
}
