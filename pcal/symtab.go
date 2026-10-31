package pcal

import "strconv"

// Symbol kinds, as in PcalSymTab.
const (
	symGlobal = iota
	symLabel
	symProcedure
	symProcess
	symProcessVar
	symProcedureVar
	symParameter
	numSymKinds
)

var symKindName = []string{
	"Global variable", "Label", "Procedure", "Process",
	"Process variable", "Procedure variable", "Parameter",
}

type symEntry struct {
	kind      int
	id        string
	context   string
	cType     string
	line, col int
	useThis   string
}

type procedureEntry struct {
	name   string
	params []*PVarDecl
	decls  []*PVarDecl
	iPC    jstring
	ast    *Procedure
}

type processEntry struct {
	name  string
	isEq  bool
	id    *expr
	decls []*VarDecl
	iPC   jstring
	ast   *Process
}

// symTab ports PcalSymTab.
type symTab struct {
	syms               []*symEntry
	procs              []*procedureEntry
	processes          []*processEntry
	disambiguateReport []string
	errorReport        string
	iPC                jstring
}

func firstLabel(body *seq) jstring {
	if body.len() == 0 {
		return jstring{}
	}
	return some(body.at(0).(*LabeledStmt).label)
}

func newSymTab(ast node) (*symTab, *Error) {
	st := &symTab{}
	st.insert(symGlobal, "pc", "", "", 0, 0)
	st.extract(ast)
	if st.errorReport != "" {
		return nil, &Error{Message: st.errorReport}
	}
	return st, nil
}

func isVariableKind(kind int) bool {
	return kind == symGlobal || kind == symProcessVar || kind == symProcedureVar || kind == symParameter
}

func (st *symTab) insert(kind int, id, context, cType string, line, col int) bool {
	if kind == symProcedureVar || kind == symProcessVar || kind == symParameter {
		if st.find(symGlobal, id, "") < len(st.syms) || st.findAny(id, context) < len(st.syms) {
			return false
		}
	} else if st.find(kind, id, context) < len(st.syms) {
		return false
	}
	st.syms = append(st.syms, &symEntry{kind: kind, id: id, context: context, cType: cType, line: line, col: col, useThis: id})
	return true
}

func (st *symTab) find(kind int, id, context string) int {
	for i, se := range st.syms {
		if se.id == id && se.context == context && se.kind == kind {
			return i
		}
	}
	return len(st.syms)
}

func (st *symTab) findAny(id, context string) int {
	for i, se := range st.syms {
		if se.id == id && se.context == context {
			return i
		}
	}
	return len(st.syms)
}

func (st *symTab) findProc(name string) int {
	for i, pe := range st.procs {
		if pe.name == name {
			return i
		}
	}
	return len(st.procs)
}

func (st *symTab) findProcess(name string) int {
	for i, pe := range st.processes {
		if pe.name == name {
			return i
		}
	}
	return len(st.processes)
}

// useThis returns the disambiguated name of a symbol.
func (st *symTab) useThis(kind int, id, context string) string {
	i := st.find(kind, id, context)
	if i == len(st.syms) {
		return id
	}
	return st.syms[i].useThis
}

// useThisVar returns the disambiguated name of a variable.
func (st *symTab) useThisVar(id, context string) string {
	i := st.findAny(id, context)
	if i == len(st.syms) {
		return id
	}
	if se := st.syms[i]; isVariableKind(se.kind) {
		return se.useThis
	}
	return id
}

func (st *symTab) ambiguous(id string) bool {
	found := false
	for _, se := range st.syms {
		if se.useThis != id {
			continue
		}
		if found {
			return true
		}
		found = true
	}
	return false
}

// disambiguate renames symbols that would otherwise share a TLA+ name.
func (st *symTab) disambiguate() {
	for kind := 0; kind <= numSymKinds; kind++ {
		for _, se := range st.syms {
			if se.kind == kind {
				st.rename(se, kind)
			}
		}
	}
}

func (st *symTab) rename(se *symEntry, kind int) {
	se.useThis = se.id
	suffix := 0
	for kind > 0 && st.ambiguous(se.useThis) {
		suffix++
		switch {
		case suffix == 1:
			se.useThis += "_"
		case suffix > len(se.context)+1:
			se.useThis += strconv.Itoa(kind)
		default:
			se.useThis += se.context[suffix-2 : suffix-1]
		}
	}
	if se.id == se.useThis {
		return
	}
	ctx := ""
	if se.context != "" {
		ctx = " of " + se.cType + " " + se.context
	}
	st.disambiguateReport = append(st.disambiguateReport, "\\* "+symKindName[se.kind]+" "+se.id+ctx+
		" at line "+strconv.Itoa(se.line)+" col "+strconv.Itoa(se.col)+" changed to "+se.useThis)
}

func (st *symTab) redefined(what, name string, line, col int) {
	st.errorReport += "\n" + what + " " + name + " redefined at line " + strconv.Itoa(line) + ", column " + strconv.Itoa(col)
}

func (st *symTab) extract(ast node) {
	switch a := ast.(type) {
	case *Uniprocess:
		st.extractProgram(a.decls, a.prcds)
		if a.body.len() > 0 {
			st.iPC = some(a.body.at(0).(*LabeledStmt).label)
		}
		st.extractLabeled(a.body, "", "")
	case *Multiprocess:
		st.extractProgram(a.decls, a.prcds)
		for _, proc := range a.procs {
			st.extractProcess(proc)
		}
	default:
		reportBug("Unexpected AST type.")
	}
}

func (st *symTab) extractProgram(decls []*VarDecl, prcds []*Procedure) {
	if len(prcds) > 0 {
		st.insert(symGlobal, "stack", "", "", 0, 0)
	}
	for _, d := range decls {
		st.extractVarDecl(d, "")
	}
	for _, p := range prcds {
		st.extractProcedure(p)
	}
}

func (st *symTab) extractProcedure(p *Procedure) {
	if st.findProc(p.name) < len(st.procs) {
		st.redefined("Procedure", p.name, p.line, p.col)
	} else {
		st.procs = append(st.procs, &procedureEntry{name: p.name, params: p.params, decls: p.decls, iPC: firstLabel(p.body), ast: p})
	}
	st.insert(symProcedure, p.name, "", "procedure", p.line, p.col)
	for _, d := range p.decls {
		st.extractPVar(symProcedureVar, "Procedure variable", d, p.name)
	}
	for _, d := range p.params {
		st.extractPVar(symParameter, "Parameter", d, p.name)
	}
	st.extractLabeled(p.body, p.name, "procedure")
}

func (st *symTab) extractProcess(p *Process) {
	if st.findProcess(p.name) < len(st.processes) {
		st.redefined("Process", p.name, p.line, p.col)
	} else {
		st.processes = append(st.processes, &processEntry{name: p.name, isEq: p.isEq, id: p.id, decls: p.decls, iPC: firstLabel(p.body), ast: p})
	}
	st.insert(symProcess, p.name, "", "process", p.line, p.col)
	for _, d := range p.decls {
		st.extractVarDecl(d, p.name)
	}
	st.extractLabeled(p.body, p.name, "process")
}

func (st *symTab) extractVarDecl(d *VarDecl, context string) {
	kind := symProcessVar
	if context == "" {
		kind = symGlobal
	}
	if !st.insert(kind, d.name, context, "process", d.line, d.col) {
		st.redefined(symKindName[kind], d.name, d.line, d.col)
	}
}

func (st *symTab) extractPVar(kind int, what string, d *PVarDecl, context string) {
	if !st.insert(kind, d.name, context, "procedure", d.line, d.col) {
		st.redefined(what, d.name, d.line, d.col)
	}
}

func (st *symTab) extractLabeled(stmts *seq, context, cType string) {
	for _, s := range stmts.items {
		ls := s.(*LabeledStmt)
		if !st.insert(symLabel, ls.label, context, cType, ls.line, ls.col) {
			st.redefined("Label", ls.label, ls.line, ls.col)
		}
		st.extractStmts(ls.stmts, context, cType)
	}
}

func (st *symTab) extractStmts(stmts *seq, context, cType string) {
	for _, s := range stmts.items {
		st.extractStmt(s, context, cType)
	}
}

func (st *symTab) extractStmt(ast node, context, cType string) {
	switch a := ast.(type) {
	case *While:
		st.extractStmts(a.unlabDo, context, cType)
		st.extractLabeled(a.labDo, context, cType)
	case *If:
		st.extractStmts(a.then, context, cType)
		st.extractStmts(a.els, context, cType)
	case *With:
		st.extractStmts(a.do, context, cType)
	default:
		st.extractBranches(ast, context, cType)
	}
}

func (st *symTab) extractBranches(ast node, context, cType string) {
	switch a := ast.(type) {
	case *LabelIf:
		st.extractStmts(a.unlabThen, context, cType)
		st.extractLabeled(a.labThen, context, cType)
		st.extractStmts(a.unlabElse, context, cType)
		st.extractLabeled(a.labElse, context, cType)
	case *Either:
		for _, or := range a.ors {
			st.extractStmts(or, context, cType)
		}
	case *LabelEither:
		st.extractLabelEither(a, context, cType)
	case *Assign, *When, *PrintS, *Assert, *Skip, *Call, *Return, *CallReturn, *CallGoto, *Goto:
	default:
		reportBug("Unexpected AST type " + astString(ast, false, false))
	}
}

func (st *symTab) extractLabelEither(a *LabelEither, context, cType string) {
	for _, c := range a.clauses {
		st.extractStmts(c.unlabOr, context, cType)
		st.extractLabeled(c.labOr, context, cType)
	}
}

// checkForDefaultInitValue rejects symbols named defaultInitValue.
func (st *symTab) checkForDefaultInitValue() *Error {
	errors := ""
	for _, se := range st.syms {
		if se.id != "defaultInitValue" {
			continue
		}
		if errors == "" {
			errors = "Cannot use `defaultInitValue' as "
		} else {
			errors += " or "
		}
		errors += symKindName[se.kind] + " name"
	}
	if errors != "" {
		return &Error{Message: errors}
	}
	return nil
}
