package pcal

func (x *exploder) explodeLabelIf(a *LabelIf, next string) (*seq, *seq) {
	thenStmts, thenRest := x.copyAndExplodeLastStmtWithGoto(a.unlabThen, firstLabelOr(a.labThen, next))
	elseStmts, elseRest := x.copyAndExplodeLastStmtWithGoto(a.unlabElse, firstLabelOr(a.labElse, next))
	n := &If{test: a.test, then: thenStmts, els: elseStmts}
	n.col, n.line, n.origin = a.col, a.line, a.origin
	if a.labThen.len() != 0 || thenRest.len() != 0 {
		n.source += brokenThen
	}
	if a.labElse.len() != 0 || elseRest.len() != 0 {
		n.source += brokenElse
	}
	rest := x.labeledStmtSeq(a.labThen, next)
	rest.add(x.labeledStmtSeq(a.labElse, next).items...)
	rest.add(thenRest.items...)
	rest.add(elseRest.items...)
	return newSeq(n), rest
}

func firstLabelOr(labeled *seq, next string) string {
	if labeled.len() > 0 {
		return labeled.at(0).(*LabeledStmt).label
	}
	return next
}

func (x *exploder) explodeLabelEither(a *LabelEither, next string) (*seq, *seq) {
	n := &Either{}
	n.col, n.line, n.origin = a.col, a.line, a.origin
	rest := newSeq()
	for _, c := range a.clauses {
		stmts, split := x.copyAndExplodeLastStmtWithGoto(c.unlabOr, firstLabelOr(c.labOr, next))
		if c.labOr.len() != 0 || split.len() != 0 {
			c.broken = true
		}
		n.ors = append(n.ors, stmts)
		rest.add(split.items...)
		rest.add(x.labeledStmtSeq(c.labOr, next).items...)
	}
	return newSeq(n), rest
}

// stmtAt returns an assignment and single assignment positioned at b.
func assignAt(b *nodeBase) (*Assign, *SingleAssign) {
	a := &Assign{}
	a.line, a.col = b.line, b.col
	sa := newSingleAssign()
	sa.line, sa.col = b.line, b.col
	return a, sa
}

func headOf(field string) *expr {
	return newTokenLines().add(identToken("Head"), builtInToken("("), addedToken("stack"),
		builtInToken(")"), builtInToken("."), identToken(field)).e.normalized()
}

func (e *expr) normalized() *expr {
	e.normalize()
	return e
}

// frameFields adds the saved variables of a stack frame record.
func frameFields(b *tokenLines, decls, params []*PVarDecl) {
	for _, d := range append(append([]*PVarDecl(nil), decls...), params...) {
		b.add(builtInToken(",")).newline().add(identToken(d.name), builtInToken("|->"), identToken(d.name))
	}
}

// pushFrame returns the expression that pushes a frame for a call of pe.
func pushFrame(to string, pc []*token, pe *procedureEntry, tail ...*token) *expr {
	b := newTokenLines().add(builtInToken("<<"), builtInToken("["), identToken("procedure"),
		builtInToken("|->"), stringToken(to), builtInToken(","))
	b.newline().add(identToken("pc"), builtInToken("|->")).add(pc...)
	frameFields(b, pe.decls, pe.params)
	b.add(builtInToken("]"), builtInToken(">>")).newline().add(builtInToken(`\o`)).add(tail...)
	makeNewStackTopExprPretty(b.e)
	return b.e.normalized()
}

func (x *exploder) callee(to string, at node) *procedureEntry {
	i := x.st.findProc(to)
	if i == len(x.st.procs) {
		failAt("Call of non-existent procedure "+to, at)
	}
	return x.st.procs[i]
}

// bindParams assigns the call arguments to the procedure parameters.
func bindParams(ass *Assign, b *nodeBase, pe *procedureEntry, args []*expr, name string, at node, origins bool) {
	if len(pe.params) != len(args) {
		failAt("Procedure "+name+" called with wrong number of arguments", at)
	}
	for i, d := range pe.params {
		_, sa := assignAt(b)
		if origins {
			sa.origin = d.origin
		}
		sa.lhs.name = d.name
		sa.lhs.sub = newExpr(nil)
		sa.rhs = args[i]
		ass.ass = append(ass.ass, sa)
	}
	if len(pe.params) > 0 {
		ass.origin = &region{pe.params[0].origin.beginLoc(), pe.params[len(pe.params)-1].origin.endLoc()}
	}
}

// initDecls assigns the initial values of the procedure variables.
func initDecls(result *seq, b *nodeBase, pe *procedureEntry) {
	for _, d := range pe.decls {
		a, sa := assignAt(b)
		sa.origin = d.origin
		sa.lhs.name = d.name
		sa.lhs.sub = newExpr(nil)
		sa.rhs = d.val
		a.origin = d.origin
		a.ass = []*SingleAssign{sa}
		result.add(a)
	}
}

func (x *exploder) explodeCall(c *Call, next string) *seq {
	pe := x.callee(c.to, c)
	ass, sa := assignAt(&c.nodeBase)
	sa.lhs.name = "stack"
	sa.lhs.sub = newExpr(nil)
	sa.rhs = pushFrame(c.to, []*token{stringToken(next)}, pe, addedToken("stack"))
	ass.ass = []*SingleAssign{sa}
	bindParams(ass, &c.nodeBase, pe, c.args, c.to, c, true)
	result := newSeq(ass)
	initDecls(result, &c.nodeBase, pe)
	result.add(updatePC(pe.iPC.String()))
	return result
}

// restoreAssign assigns name := Head(stack).name.
func restoreAssign(r *Return, name string, origin bool) *Assign {
	a, sa := assignAt(&r.nodeBase)
	sa.lhs.name = name
	sa.lhs.sub = &expr{}
	sa.rhs = headOf(name)
	if origin {
		sa.origin, a.origin = r.origin, r.origin
	}
	a.ass = []*SingleAssign{sa}
	return a
}

func (x *exploder) explodeReturn(r *Return) *seq {
	if !r.from.ok {
		r.from = x.currentProcedure
	}
	if !r.from.ok {
		failAt("`return' statement not in procedure", r)
	}
	from := x.st.findProc(r.from.val)
	if from >= len(x.st.procs) {
		failAt("Error in procedure (perhaps name used elsewhere)", r)
	}
	pe := x.st.procs[from]
	result := newSeq(restoreAssign(r, "pc", false))
	for _, d := range append(append([]*PVarDecl(nil), pe.decls...), pe.params...) {
		result.add(restoreAssign(r, d.name, true))
	}
	a, sa := assignAt(&r.nodeBase)
	sa.origin, a.origin = r.origin, r.origin
	sa.lhs.name = "stack"
	sa.lhs.sub = &expr{}
	sa.rhs = newTokenLines().add(identToken("Tail"), builtInToken("("), addedToken("stack"), builtInToken(")")).e.normalized()
	a.ass = []*SingleAssign{sa}
	result.add(a)
	return result
}

func (x *exploder) explodeCallReturn(c *CallReturn) *seq {
	if !c.from.ok {
		fail("`return' statement following `call' at " + c.location() + " not in a procedure")
	}
	peFrom := x.st.procs[x.st.findProc(c.from.val)]
	peTo := x.callee(c.to, c)
	ass := &Assign{}
	ass.line, ass.col = c.line, c.col
	if c.from.val != c.to {
		ass.ass = x.replaceFrame(c, peFrom, peTo)
	}
	bindParams(ass, &c.nodeBase, peTo, c.args, c.to, c, false)
	result := newSeq(ass)
	initDecls(result, &c.nodeBase, peTo)
	result.add(updatePC(peTo.iPC.String()))
	return result
}

// replaceFrame restores the caller's variables and replaces the top stack
// frame by one for the called procedure.
func (x *exploder) replaceFrame(c *CallReturn, peFrom, peTo *procedureEntry) []*SingleAssign {
	var result []*SingleAssign
	for _, d := range peFrom.decls {
		_, sa := assignAt(&c.nodeBase)
		sa.lhs.name = d.name
		sa.lhs.sub = &expr{}
		sa.rhs = headOf(d.name)
		result = append(result, sa)
	}
	_, sa := assignAt(&c.nodeBase)
	sa.lhs.name = "stack"
	sa.lhs.sub = newExpr(nil)
	pc := []*token{identToken("Head"), builtInToken("("), addedToken("stack"), builtInToken(")"), builtInToken("."), identToken("pc")}
	sa.rhs = pushFrame(c.to, pc, peTo, identToken("Tail"), builtInToken("("), addedToken("stack"), builtInToken(")"))
	return append(result, sa)
}

func (x *exploder) explodeCallGoto(c *CallGoto) *seq {
	call := &Call{to: c.to, args: c.args}
	call.line, call.col, call.origin = c.line, c.col, c.origin
	return x.explodeCall(call, c.after)
}
