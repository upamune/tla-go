package pcal

// exploder ports PcalTranslate, which turns labeled statements into one
// statement sequence per atomic step with explicit pc updates.
type exploder struct {
	st               *symTab
	omitPC           bool
	currentProcedure jstring
}

func stringToken(s string) *token  { return &token{str: s, typ: tokString} }
func addedToken(s string) *token   { return &token{str: s, typ: tokAdded} }
func builtInToken(s string) *token { return &token{str: s, typ: tokBuiltin} }
func identToken(s string) *token   { return &token{str: s, typ: tokIdent} }

// tokensToExpr makes a one-line expression with the given gap between
// tokens.
func tokensToExpr(toks []*token, gap int) *expr {
	line := make([]*token, len(toks))
	col := 0
	for i, t := range toks {
		c := t.clone()
		c.column = col
		line[i] = c
		col += c.width() + gap
	}
	return newExpr([][]*token{line})
}

// tokenLines builds an expression from token lines without positions.
type tokenLines struct{ e *expr }

func newTokenLines() *tokenLines { return &tokenLines{e: &expr{tokens: [][]*token{nil}}} }

func (b *tokenLines) add(toks ...*token) *tokenLines {
	for _, t := range toks {
		b.e.addToken(t)
	}
	return b
}

func (b *tokenLines) newline() *tokenLines {
	b.e.addLine()
	return b
}

// makeNewStackTopExprPretty lays out a pushed stack frame record.
func makeNewStackTopExprPretty(e *expr) {
	for i, line := range e.tokens {
		col := 6
		if i == 0 || i == len(e.tokens)-1 {
			col = 1
		}
		for _, t := range line {
			t.column = col
			col = prettyColumn(t, col+t.width())
		}
	}
}

// prettyColumn adjusts the column of a builtin token and returns the column
// of the next token.
func prettyColumn(t *token, next int) int {
	if t.typ != tokBuiltin {
		return next
	}
	switch t.str {
	case "|->":
		t.column = max(t.column+1, 16)
		return t.column + 5
	case "[", "<<":
		return next + 1
	case "]", ">>":
		t.column++
		return next + 1
	case `\o`:
		t.column++
		return next + 2
	}
	return next
}

func makeAssign(id string, e *expr) *Assign {
	sa := newSingleAssign()
	sa.lhs.name = id
	sa.lhs.sub = newExpr(nil)
	sa.rhs = e
	return &Assign{ass: []*SingleAssign{sa}}
}

func isTRUE(e *expr) bool {
	if len(e.tokens) > 1 {
		return false
	}
	line := e.tokens[0]
	switch len(line) {
	case 1:
		return line[0].str == "TRUE"
	case 3:
		return line[0].str == "(" && line[1].str == "TRUE" && line[2].str == ")"
	}
	return false
}

func checkPC(label string) *When {
	return &When{exp: tokensToExpr([]*token{addedToken("pc"), builtInToken("="), stringToken(label)}, 1)}
}

func updatePC(next string) *Assign {
	return makeAssign("pc", newExpr([][]*token{{stringToken(next)}}))
}

func explode(ast node, st *symTab, omitPC bool) (result node, err *Error) {
	x := &exploder{st: st, omitPC: omitPC}
	err = catch(func() {
		switch a := ast.(type) {
		case *Uniprocess:
			result = x.uniprocess(a)
		case *Multiprocess:
			result = x.multiprocess(a)
		default:
			reportBug("Unexpected AST type.")
		}
	}, nil)
	return result, err
}

func (x *exploder) uniprocess(a *Uniprocess) *Uniprocess {
	n := &Uniprocess{name: a.name, decls: a.decls, defs: a.defs}
	n.pos, n.origin = a.pos, a.origin
	for _, p := range a.prcds {
		n.prcds = append(n.prcds, x.procedure(p))
	}
	n.body = x.labeledBody(a.body, "Done")
	return n
}

// labeledBody explodes a sequence of labeled statements; the last one
// continues at last.
func (x *exploder) labeledBody(body *seq, last string) *seq {
	result := newSeq()
	for i, s := range body.items {
		next := last
		if i+1 < body.len() {
			next = body.at(i + 1).(*LabeledStmt).label
		}
		result.add(x.labeledStmt(s.(*LabeledStmt), next).items...)
	}
	return result
}

func (x *exploder) multiprocess(a *Multiprocess) *Multiprocess {
	n := &Multiprocess{name: a.name, decls: a.decls, defs: a.defs}
	n.pos, n.origin = a.pos, a.origin
	for _, p := range a.prcds {
		n.prcds = append(n.prcds, x.procedure(p))
	}
	for _, p := range a.procs {
		n.procs = append(n.procs, x.process(p))
	}
	return n
}

func (x *exploder) procedure(a *Procedure) *Procedure {
	n := &Procedure{name: a.name, params: a.params, decls: a.decls}
	n.origin, n.col, n.line = a.origin, a.col, a.line
	x.currentProcedure = some(a.name)
	n.body = x.labeledBody(a.body, x.st.useThis(symLabel, "Error", ""))
	x.currentProcedure = jstring{}
	return n
}

func (x *exploder) process(a *Process) *Process {
	n := &Process{name: a.name, isEq: a.isEq, id: a.id, decls: a.decls}
	n.origin, n.col, n.line = a.origin, a.col, a.line
	n.body = x.labeledBody(a.body, "Done")
	return n
}

// exploded is the result of exploding a statement sequence: the statements
// of the current step, the labeled statements split off, and whether the
// step still needs a pc update.
type exploded struct {
	stmts     *seq
	rest      *seq
	needsGoto bool
}

// copyAndExplodeLastStmt explodes the last statement of stmts in place.
func (x *exploder) copyAndExplodeLastStmt(stmts *seq, next string) exploded {
	res := exploded{stmts: newSeq(), rest: newSeq(), needsGoto: true}
	if stmts != nil && stmts.len() > 0 {
		res.stmts = stmts
		res.needsGoto = x.explodeLast(stmts, next, res.rest)
	}
	if x.omitPC {
		res.needsGoto = false
	}
	return res
}

// explodeLast replaces the last statement of stmts, appending split-off
// statements to rest, and reports whether a pc update is still needed.
func (x *exploder) explodeLast(stmts *seq, next string, rest *seq) bool {
	last := stmts.last()
	switch s := last.(type) {
	case *If:
		return x.explodeIf(s, next, rest)
	case *Either:
		return x.explodeEither(s, next, rest)
	case *With:
		p := x.copyAndExplodeLastStmt(s.do, next)
		s.do = p.stmts
		rest.add(p.rest.items...)
		return p.needsGoto
	}
	replacement, split, ok := x.explodeTransfer(last, next)
	if !ok {
		return true
	}
	stmts.removeLast()
	stmts.add(replacement.items...)
	rest.add(split.items...)
	return false
}

// explodeTransfer explodes the statements that change control flow.
func (x *exploder) explodeTransfer(last node, next string) (replacement, split *seq, ok bool) {
	split = newSeq()
	switch s := last.(type) {
	case *LabelIf:
		replacement, split = x.explodeLabelIf(s, next)
	case *LabelEither:
		replacement, split = x.explodeLabelEither(s, next)
	case *Goto:
		replacement = newSeq(updatePC(s.to))
	case *Call:
		replacement = x.explodeCall(s, next)
	case *Return:
		replacement = x.explodeReturn(s)
	case *CallReturn:
		replacement = x.explodeCallReturn(s)
	case *CallGoto:
		replacement = x.explodeCallGoto(s)
	default:
		return nil, nil, false
	}
	return replacement, split, true
}

func (x *exploder) explodeIf(s *If, next string, rest *seq) bool {
	p1 := x.copyAndExplodeLastStmt(s.then, next)
	p2 := x.copyAndExplodeLastStmt(s.els, next)
	rest.add(p1.rest.items...)
	rest.add(p2.rest.items...)
	s.then, s.els = p1.stmts, p2.stmts
	if x.omitPC {
		return false
	}
	needsGoto := p1.needsGoto && p2.needsGoto
	if !needsGoto {
		x.addGoto(s.then, p1.needsGoto, next)
		x.addGoto(s.els, p2.needsGoto, next)
	}
	return needsGoto
}

func (x *exploder) addGoto(stmts *seq, needed bool, next string) {
	if needed {
		stmts.add(updatePC(next))
	}
}

func (x *exploder) explodeEither(s *Either, next string, rest *seq) bool {
	needsGoto := true
	needs := make([]bool, len(s.ors))
	for i, or := range s.ors {
		p := x.copyAndExplodeLastStmt(or, next)
		s.ors[i] = p.stmts
		rest.add(p.rest.items...)
		needsGoto = needsGoto && p.needsGoto
		needs[i] = p.needsGoto
	}
	if !x.omitPC && !needsGoto {
		for i, or := range s.ors {
			x.addGoto(or, needs[i], next)
		}
	}
	return needsGoto
}

func (x *exploder) copyAndExplodeLastStmtWithGoto(stmts *seq, next string) (*seq, *seq) {
	res := x.copyAndExplodeLastStmt(stmts, next)
	if res.needsGoto {
		res.stmts.add(updatePC(next))
	}
	return res.stmts, res.rest
}

func (x *exploder) labeledStmt(a *LabeledStmt, next string) *seq {
	if a.stmts.len() > 0 {
		if _, ok := a.stmts.at(0).(*While); ok {
			return x.explodeWhile(a, next)
		}
	}
	n := &LabeledStmt{label: a.label}
	stmts, rest := x.copyAndExplodeLastStmtWithGoto(a.stmts.clone(), next)
	result := newSeq()
	n.origin, n.col, n.line = a.origin, a.col, a.line
	n.stmts = stmts
	if !x.omitPC {
		n.stmts.items = append([]node{checkPC(n.label)}, n.stmts.items...)
		result.add(n)
	}
	result.add(rest.items...)
	return result
}

func (x *exploder) labeledStmtSeq(stmts *seq, next string) *seq {
	result := newSeq()
	for i, s := range stmts.items {
		nxt := next
		if i < stmts.len()-1 {
			nxt = stmts.at(i + 1).(*LabeledStmt).label
		}
		result.add(x.labeledStmt(s.(*LabeledStmt), nxt).items...)
	}
	return result
}

func (x *exploder) explodeWhile(a *LabeledStmt, next string) *seq {
	w := a.stmts.at(0).(*While)
	n := &LabeledStmt{label: a.label, stmts: newSeq()}
	n.origin = &region{a.origin.beginLoc(), whileEnd(a, w)}
	whileBegin := w.origin.beginLoc()
	n.col, n.line = a.col, a.line
	if !x.omitPC {
		n.stmts.add(checkPC(a.label))
	}
	unlabDoNext := a.label
	if w.labDo.len() > 0 {
		unlabDoNext = w.labDo.at(0).(*LabeledStmt).label
	}
	doStmts, doRest := x.copyAndExplodeLastStmtWithGoto(w.unlabDo.clone(), unlabDoNext)
	after := a.stmts.clone()
	after.items = after.items[1:]
	afterStmts, afterRest := x.copyAndExplodeLastStmtWithGoto(after, next)
	if isTRUE(w.test) {
		n.stmts.add(doStmts.items...)
	} else {
		ifS := &If{test: w.test, then: doStmts, els: afterStmts}
		ifS.origin = &region{whileBegin, n.origin.end}
		n.stmts.add(ifS)
	}
	result := newSeq(n)
	result.add(x.labeledStmtSeq(w.labDo, a.label).items...)
	result.add(doRest.items...)
	if !isTRUE(w.test) {
		result.add(afterRest.items...)
	}
	return result
}

// whileEnd returns the end of the region of an exploded while statement.
func whileEnd(a *LabeledStmt, w *While) location {
	if a.stmts.len() != 1 {
		return a.origin.endLoc()
	}
	if w.unlabDo.len() != 0 {
		return w.unlabDo.last().base().origin.endLoc()
	}
	return w.test.origin.endLoc()
}
