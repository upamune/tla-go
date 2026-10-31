package pcal

import (
	"slices"
	"strconv"
)

// fixer ports PcalFixIDs, which renames identifiers to their disambiguated
// names.
type fixer struct {
	st  *symTab
	log *logger
}

func fixIDs(ast node, st *symTab, log *logger) {
	f := &fixer{st: st, log: log}
	switch a := ast.(type) {
	case *Uniprocess:
		f.uniprocess(a)
	case *Multiprocess:
		f.multiprocess(a)
	default:
		reportBug("Unexpected AST type" + astString(ast, false, false))
	}
	if st.iPC.ok {
		st.iPC = some(st.useThis(symLabel, st.iPC.val, ""))
	}
}

func (f *fixer) uniprocess(a *Uniprocess) {
	for _, d := range a.decls {
		f.varDecl(d, "")
	}
	for _, p := range a.prcds {
		f.procedure(p)
	}
	f.labeledStmts(a.body, "")
}

func (f *fixer) multiprocess(a *Multiprocess) {
	for _, d := range a.decls {
		f.varDecl(d, "")
	}
	names := make([]string, len(a.prcds))
	called := make([][]string, len(a.prcds))
	for i, p := range a.prcds {
		f.procedure(p)
		names[i] = p.name
		called[i] = p.proceduresCalled.items
	}
	path := callClosure(names, called)
	for i, p := range a.prcds {
		p.proceduresCalled = &strList{items: reachable(names, path[i])}
	}
	for _, proc := range a.procs {
		f.process(proc)
		if !f.addCallees(proc, names, path) {
			return
		}
	}
}

func reachable(names []string, path []bool) []string {
	var result []string
	for j, name := range names {
		if path[j] {
			result = append(result, name)
		}
	}
	return result
}

// callClosure returns the transitive closure of the procedure call graph.
func callClosure(names []string, called [][]string) [][]bool {
	path := make([][]bool, len(names))
	for i := range path {
		path[i] = make([]bool, len(names))
		for j, name := range names {
			path[i][j] = slices.Contains(called[i], name)
		}
	}
	for k := range path {
		closeThrough(path, k)
	}
	return path
}

func closeThrough(path [][]bool, k int) {
	for i := range path {
		for j := range path {
			path[i][j] = path[i][j] || (path[i][k] && path[k][j])
		}
	}
}

// addCallees adds the procedures reachable from those a process calls.
func (f *fixer) addCallees(proc *Process, names []string, path [][]bool) bool {
	for j := 0; j < len(proc.proceduresCalled.items); j++ {
		pNum := slices.Index(names, proc.proceduresCalled.items[j])
		if pNum == -1 {
			f.log.error("Could not find procedure name `" + proc.proceduresCalled.items[j] + "'")
			return false
		}
		for k, callee := range names {
			if path[pNum][k] && !proc.proceduresCalled.contains(callee) {
				proc.proceduresCalled.add(callee)
			}
		}
	}
	return true
}

func (f *fixer) renameLabels(labels *strList, context string) {
	for i, lbl := range labels.items {
		labels.items[i] = f.st.useThis(symLabel, lbl, context)
	}
}

func (f *fixer) procedure(p *Procedure) {
	for _, d := range p.decls {
		f.pVarDecl(d, p.name)
	}
	for _, d := range p.params {
		f.pVarDecl(d, p.name)
	}
	f.labeledStmts(p.body, p.name)
	pe := f.st.procs[f.st.findProc(p.name)]
	f.renameLabels(p.plusLabels, p.name)
	f.renameLabels(p.minusLabels, p.name)
	pe.iPC = f.useThisLabel(pe.iPC, p.name)
	p.name = f.st.useThis(symProcedure, p.name, "")
	pe.name = p.name
}

func (f *fixer) process(p *Process) {
	for _, d := range p.decls {
		f.varDecl(d, p.name)
	}
	f.labeledStmts(p.body, p.name)
	pe := f.st.processes[f.st.findProcess(p.name)]
	f.renameLabels(p.plusLabels, p.name)
	f.renameLabels(p.minusLabels, p.name)
	pe.iPC = f.useThisLabel(pe.iPC, p.name)
	p.name = f.st.useThis(symProcess, p.name, "")
	pe.name = p.name
}

func (f *fixer) useThisLabel(lbl jstring, context string) jstring {
	if !lbl.ok {
		return lbl
	}
	return some(f.st.useThis(symLabel, lbl.val, context))
}

func (f *fixer) useThisProc(name jstring, context string) jstring {
	if !name.ok {
		return name
	}
	return some(f.st.useThis(symProcedure, name.val, context))
}

func (f *fixer) varDecl(d *VarDecl, context string) {
	d.name = f.st.useThisVar(d.name, context)
	f.expr(d.val, context)
}

func (f *fixer) pVarDecl(d *PVarDecl, context string) {
	d.name = f.st.useThisVar(d.name, context)
	f.expr(d.val, context)
}

func (f *fixer) labeledStmts(stmts *seq, context string) {
	for _, s := range stmts.items {
		ls := s.(*LabeledStmt)
		ls.label = f.st.useThis(symLabel, ls.label, context)
		f.stmts(ls.stmts, context)
	}
}

func (f *fixer) stmts(stmts *seq, context string) {
	for _, s := range stmts.items {
		f.stmt(s, context)
	}
}

func (f *fixer) stmt(ast node, context string) {
	switch a := ast.(type) {
	case *While:
		f.stmts(a.unlabDo, context)
		f.labeledStmts(a.labDo, context)
		f.expr(a.test, context)
	case *Assign:
		for _, sa := range a.ass {
			f.singleAssign(sa, context)
		}
	case *If:
		f.stmts(a.then, context)
		f.stmts(a.els, context)
		f.expr(a.test, context)
	case *With:
		f.expr(a.exp, context)
		f.stmts(a.do, context)
	default:
		f.branchStmt(ast, context)
	}
}

func (f *fixer) branchStmt(ast node, context string) {
	switch a := ast.(type) {
	case *LabelIf:
		f.labelIf(a, context)
	case *Either:
		for _, or := range a.ors {
			f.stmts(or, context)
		}
	case *LabelEither:
		f.labelEither(a, context)
	default:
		f.simpleStmt(ast, context)
	}
}

func (f *fixer) singleAssign(sa *SingleAssign, context string) {
	sa.lhs.name = f.st.useThisVar(sa.lhs.name, context)
	f.expr(sa.lhs.sub, context)
	f.expr(sa.rhs, context)
}

func (f *fixer) labelIf(a *LabelIf, context string) {
	f.expr(a.test, context)
	f.stmts(a.unlabThen, context)
	f.labeledStmts(a.labThen, context)
	f.stmts(a.unlabElse, context)
	f.labeledStmts(a.labElse, context)
}

func (f *fixer) labelEither(a *LabelEither, context string) {
	for _, c := range a.clauses {
		f.stmts(c.unlabOr, context)
		f.labeledStmts(c.labOr, context)
	}
}

func (f *fixer) simpleStmt(ast node, context string) {
	switch a := ast.(type) {
	case *When:
		f.expr(a.exp, context)
	case *PrintS:
		f.expr(a.exp, context)
	case *Assert:
		f.expr(a.exp, context)
	case *Skip:
	case *Goto:
		f.gotoStmt(a, context)
	default:
		f.callStmt(ast, context)
	}
}

func (f *fixer) gotoStmt(a *Goto, context string) {
	if f.st.find(symLabel, a.to, context) == len(f.st.syms) && a.to != "Done" {
		fail("goto to non-existent label `" + a.to + "' at line " + strconv.Itoa(a.line) + ", column " + strconv.Itoa(a.col))
	}
	a.to = f.st.useThis(symLabel, a.to, context)
}

func (f *fixer) callStmt(ast node, context string) {
	switch a := ast.(type) {
	case *Call:
		a.returnTo = f.st.useThis(symProcedure, a.returnTo, context)
		a.to = f.st.useThis(symProcedure, a.to, context)
		f.exprs(a.args, context)
	case *Return:
		a.from = f.useThisProc(a.from, context)
	case *CallReturn:
		a.from = f.useThisProc(a.from, context)
		a.to = f.st.useThis(symProcedure, a.to, context)
		f.exprs(a.args, context)
	case *CallGoto:
		a.after = f.st.useThis(symProcedure, a.after, context)
		a.to = f.st.useThis(symProcedure, a.to, context)
		f.exprs(a.args, context)
	default:
		reportBug("Unexpected AST type" + astString(ast, false, false))
	}
}

func (f *fixer) exprs(es []*expr, context string) {
	for _, e := range es {
		f.expr(e, context)
	}
}

// expr renames the variables of an expression in place.
func (f *fixer) expr(e *expr, context string) {
	var exprs []*expr
	var names []string
	for _, t := range e.allTokens() {
		if t.typ != tokIdent || slices.Contains(names, t.str) {
			continue
		}
		if useMe := f.st.useThisVar(t.str, context); useMe != t.str {
			names = append(names, t.str)
			exprs = append(exprs, exprOf(newToken(useMe, 0, tokIdent)))
		}
	}
	if len(exprs) > 0 {
		e.substituteForAll(exprs, names, false)
	}
}
