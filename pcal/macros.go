package pcal

import "slices"

// expandMacrosInStmtSeq replaces macro calls in stmts by the macro bodies.
func expandMacrosInStmtSeq(stmts *seq, macros []*Macro) {
	for i := 0; i < stmts.len(); i++ {
		call, ok := stmts.at(i).(*MacroCall)
		if !ok {
			expandNestedMacros(stmts.at(i), macros)
			continue
		}
		expansion := expandMacroCall(call, macros)
		stmts.items = slices.Replace(stmts.items, i, i+1, expansion.items...)
		i += expansion.len() - 1
	}
}

// expandNestedMacros expands the macro calls in the statement lists of a
// compound statement.
func expandNestedMacros(stmt node, macros []*Macro) {
	switch n := stmt.(type) {
	case *LabelIf:
		expandMacrosInStmtSeq(n.unlabThen, macros)
		expandMacrosInStmtSeq(n.unlabElse, macros)
	case *LabelEither:
		for _, c := range n.clauses {
			expandMacrosInStmtSeq(c.unlabOr, macros)
		}
	case *With:
		expandMacrosInStmtSeq(n.do, macros)
	case *While:
		expandMacrosInStmtSeq(n.unlabDo, macros)
	}
}

func expandMacroCall(call *MacroCall, macros []*Macro) *seq {
	var def *Macro
	for _, m := range macros {
		if m.name == call.name {
			def = m
		}
	}
	if def == nil {
		fail("Macro " + call.name + " undefined,\n    at " + call.location())
	}
	if len(def.params) != len(call.args) {
		fail("Macro " + call.name + " called with wrong number of arguments at \n    " + call.location())
	}
	sub := &macroSubst{args: call.args, params: def.params, macroLine: call.line, macroCol: call.col}
	result := sub.stmts(def.body)
	if result.len() > 0 {
		first := result.at(0).base()
		first.lbl = call.lbl
		first.lblLocation = call.lblLocation
		if call.origin != nil {
			b, e := call.origin.begin, call.origin.end
			first.macroBegin = &b
			result.last().base().macroEnd = &e
		}
	}
	return result
}

// macroSubst substitutes macro arguments for parameters in a macro body.
type macroSubst struct {
	args                []*expr
	params              []string
	macroLine, macroCol int
}

// macroStmt is implemented by the statements that may appear in a macro.
type macroStmt interface {
	substMacro(m *macroSubst) node
}

func (m *macroSubst) stmts(s *seq) *seq {
	result := newSeq()
	for _, stmt := range s.items {
		result.add(m.stmt(stmt))
	}
	return result
}

// stmt returns a copy of a macro-body statement with the arguments
// substituted. Errors lose their position, as in Java.
func (m *macroSubst) stmt(stmt node) (result node) {
	rethrow(func() {
		ms, ok := stmt.(macroStmt)
		if !ok {
			reportBug("Found unexpected statement type in macro at" + stmt.base().location())
		}
		result = ms.substMacro(m)
	})
	return result
}

func (m *macroSubst) labeledStmts(s *seq) *seq {
	inner := &macroSubst{args: m.args, params: m.params}
	result := newSeq()
	for _, stmt := range s.items {
		ls := stmt.(*LabeledStmt)
		n := &LabeledStmt{label: ls.label, stmts: inner.stmts(ls.stmts)}
		n.origin = ls.origin
		result.add(n)
	}
	return result
}

func (m *macroSubst) expr(e *expr) *expr {
	result := e.cloneAndNormalize()
	result.substituteForAll(m.args, m.params, true)
	return result
}

func (m *macroSubst) exprs(es []*expr) []*expr {
	result := make([]*expr, len(es))
	for i, e := range es {
		result[i] = m.expr(e)
	}
	return result
}

// position sets the positions of a substituted node from its original.
func (m *macroSubst) position(dst, src *nodeBase) {
	dst.copyPos(src)
	m.macroPosition(dst)
}

func (m *macroSubst) macroPosition(dst *nodeBase) {
	if m.macroLine > 0 {
		dst.macroLine, dst.macroCol = m.macroLine, m.macroCol
	}
}

func (s *Assign) substMacro(m *macroSubst) node {
	r := &Assign{}
	m.position(&r.nodeBase, &s.nodeBase)
	for _, a := range s.ass {
		r.ass = append(r.ass, m.singleAssign(a))
	}
	return r
}

func (s *If) substMacro(m *macroSubst) node {
	r := &If{}
	m.position(&r.nodeBase, &s.nodeBase)
	r.test = m.expr(s.test)
	r.then = m.stmts(s.then)
	r.els = m.stmts(s.els)
	return r
}

func (s *Either) substMacro(m *macroSubst) node {
	r := &Either{}
	m.position(&r.nodeBase, &s.nodeBase)
	for _, or := range s.ors {
		r.ors = append(r.ors, m.stmts(or))
	}
	return r
}

func (s *With) substMacro(m *macroSubst) node {
	r := &With{name: s.name, isEq: s.isEq}
	m.position(&r.nodeBase, &s.nodeBase)
	r.exp = m.expr(s.exp)
	r.do = m.stmts(s.do)
	return r
}

func (s *When) substMacro(m *macroSubst) node {
	r := &When{}
	m.position(&r.nodeBase, &s.nodeBase)
	r.exp = m.expr(s.exp)
	return r
}

func (s *Assert) substMacro(m *macroSubst) node {
	r := &Assert{}
	m.position(&r.nodeBase, &s.nodeBase)
	r.exp = m.expr(s.exp)
	return r
}

func (s *Skip) substMacro(m *macroSubst) node {
	r := &Skip{}
	r.line, r.col, r.origin = s.line, s.col, s.origin
	m.macroPosition(&r.nodeBase)
	return r
}

func (s *PrintS) substMacro(m *macroSubst) node {
	r := &PrintS{}
	m.position(&r.nodeBase, &s.nodeBase)
	r.exp = m.expr(s.exp)
	return r
}

func (s *While) substMacro(m *macroSubst) node {
	r := &While{}
	m.position(&r.nodeBase, &s.nodeBase)
	r.test = m.expr(s.test)
	r.unlabDo = m.stmts(s.unlabDo)
	r.labDo = m.labeledStmts(s.labDo)
	return r
}

func (s *LabelIf) substMacro(m *macroSubst) node {
	r := &LabelIf{}
	m.position(&r.nodeBase, &s.nodeBase)
	r.test = m.expr(s.test)
	r.unlabThen = m.stmts(s.unlabThen)
	r.labThen = m.labeledStmts(s.labThen)
	r.unlabElse = m.stmts(s.unlabElse)
	r.labElse = m.labeledStmts(s.labElse)
	return r
}

func (s *LabelEither) substMacro(m *macroSubst) node {
	r := &LabelEither{}
	m.position(&r.nodeBase, &s.nodeBase)
	for _, old := range s.clauses {
		c := &Clause{}
		c.origin = old.origin
		c.labOr = m.labeledStmts(old.labOr)
		c.unlabOr = m.stmts(old.unlabOr)
		r.clauses = append(r.clauses, c)
	}
	return r
}

func (s *Call) substMacro(m *macroSubst) node {
	r := &Call{to: s.to, returnTo: s.returnTo}
	m.position(&r.nodeBase, &s.nodeBase)
	r.args = m.exprs(s.args)
	return r
}

func (s *Return) substMacro(m *macroSubst) node {
	r := &Return{from: s.from}
	m.position(&r.nodeBase, &s.nodeBase)
	return r
}

func (s *CallReturn) substMacro(m *macroSubst) node {
	r := &CallReturn{to: s.to, from: s.from}
	m.position(&r.nodeBase, &s.nodeBase)
	r.args = m.exprs(s.args)
	return r
}

func (s *CallGoto) substMacro(m *macroSubst) node {
	r := &CallGoto{to: s.to, after: s.after}
	m.position(&r.nodeBase, &s.nodeBase)
	r.args = m.exprs(s.args)
	return r
}

func (s *Goto) substMacro(m *macroSubst) node {
	r := &Goto{to: s.to}
	m.position(&r.nodeBase, &s.nodeBase)
	return r
}

// singleAssign substitutes into one assignment. A parameter used as the
// assigned variable is replaced by the argument's variable, and the rest of
// the argument becomes a subscript.
func (m *macroSubst) singleAssign(a *SingleAssign) *SingleAssign {
	r := newSingleAssign()
	r.origin = a.origin
	r.pos = a.pos
	m.macroPosition(&r.nodeBase)
	r.rhs = m.expr(a.rhs)
	r.lhs.origin = a.origin
	r.lhs.sub = a.lhs.sub
	if len(a.lhs.sub.tokens) != 0 {
		r.lhs.sub = m.expr(a.lhs.sub)
	}
	r.lhs.name = a.lhs.name
	if i := slices.Index(m.params, r.lhs.name); i >= 0 {
		m.substituteVariable(r, m.args[i])
	}
	return r
}

func (m *macroSubst) substituteVariable(r *SingleAssign, arg *expr) {
	varTok := arg.tokenAt(coord{})
	if varTok.typ != tokIdent {
		fail("Macro expansion substitutes `" + varTok.str + "' for assignment variable\n    at " + r.location())
	}
	r.lhs.name = varTok.str
	if _, ok := arg.stepCoord(coord{}, 1); !ok {
		return
	}
	rest := arg.cloneAndNormalize()
	rest.tokens[0] = rest.tokens[0][1:]
	rest.normalize()
	if r.lhs.sub == nil || len(r.lhs.sub.tokens) == 0 {
		r.lhs.sub = rest
		return
	}
	r.lhs.sub.prepend(rest, 0)
}
