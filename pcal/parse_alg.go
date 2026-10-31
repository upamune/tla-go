package pcal

// algorithm parses the whole algorithm after "--algorithm" or "--fair".
func (p *parser) algorithm(fair bool) node {
	if fair {
		if tok := p.getAlgToken(); tok != "algorithm" {
			p.parsingError("`--fair' not followed by `algorithm'")
		}
		p.params.FairAlgorithm = true
	}
	name := p.getAlgToken()
	if p.peekAt(1) == "{" {
		p.cSyntax = true
		p.mustGobble("{")
	} else {
		p.pSyntax = true
	}
	var decls []*VarDecl
	if p.peekIs(1, "variable", "variables") {
		decls = p.getVarDecls()
	}
	defs := p.getDefine()
	var macros []*Macro
	for p.peekAt(1) == "macro" {
		macros = append(macros, p.getMacro(macros))
	}
	for p.peekAt(1) == "procedure" {
		p.procedures = append(p.procedures, p.getProcedure())
		p.omitPC = false
	}
	if p.processNext() {
		return p.multiprocess(name, decls, defs, macros)
	}
	return p.uniprocess(name, decls, defs, macros, fair)
}

func (p *parser) getDefine() *expr {
	defs := &expr{}
	if p.peekAt(1) != "define" {
		return defs
	}
	p.mustGobble("define")
	if p.cSyntax {
		p.gobble("{")
	}
	defs = p.getExpr()
	if p.pSyntax {
		p.gobble("end")
		p.gobble("define")
	} else {
		p.gobble("}")
	}
	p.gobble(";")
	return defs
}

func (p *parser) processNext() bool {
	if p.peekAt(1) == "process" {
		return true
	}
	return p.peekAt(1) == "fair" && (p.peekAt(2) == "process" || (p.peekAt(2) == "+" && p.peekAt(3) == "process"))
}

func (p *parser) multiprocess(name string, decls []*VarDecl, defs *expr, macros []*Macro) node {
	mp := &Multiprocess{name: name, decls: decls, defs: defs, macros: macros, prcds: p.procedures}
	begin := location{p.algLine, p.algCol}
	for p.peekIs(1, "fair", "process") {
		fairness := p.processFairness()
		proc := p.getProcess()
		proc.fairness = fairness
		mp.procs = append(mp.procs, proc)
	}
	if p.pSyntax {
		p.gobble("end")
		p.gobble("algorithm")
	} else {
		p.gobble("}")
	}
	checkForDuplicateMacros(macros)
	omitStuttering := false
	for _, proc := range mp.procs {
		proc.body = p.finishBody(proc.body, macros)
		p.omitStutteringWhenDone = true
		p.checkBody(proc.body)
		omitStuttering = omitStuttering || p.omitStutteringWhenDone
	}
	p.omitStutteringWhenDone = omitStuttering
	p.finishProcedures(macros)
	if len(p.addedLabels) > 0 {
		p.reportAddedLabels(true)
	}
	if p.gotoDoneUsed {
		p.omitPC = false
		p.omitStutteringWhenDone = false
	}
	if p.gotoUsed {
		p.omitPC = false
	}
	mp.origin = &region{begin, p.lastLocationEnd()}
	return mp
}

func (p *parser) processFairness() int {
	if p.peekAt(1) == "fair" {
		p.mustGobble("fair")
		if p.peekAt(1) == "+" {
			p.mustGobble("+")
			return sfProc
		}
		return wfProc
	}
	switch p.params.FairnessOption {
	case "wf":
		return wfProc
	case "sf":
		return sfProc
	}
	return unfairProc
}

// finishBody expands macros, adds labels, and groups a body into labeled
// statements.
func (p *parser) finishBody(body *seq, macros []*Macro) *seq {
	expandMacrosInStmtSeq(body, macros)
	p.addLabelsToStmtSeq(body)
	return p.makeLabeledStmtSeq(body)
}

func (p *parser) finishProcedures(macros []*Macro) {
	for _, prcd := range p.procedures {
		p.currentProcedure = some(prcd.name)
		prcd.body = p.finishBody(prcd.body, macros)
	}
}

func (p *parser) reportAddedLabels(mustBeAllowed bool) {
	if mustBeAllowed && !p.params.LabelFlag {
		p.addedLabelsError()
	}
	if !p.params.ReportLabels {
		p.log.info("Labels added.")
		return
	}
	if len(p.addedLabels) > 1 {
		p.log.info("The following labels were added:")
	} else {
		p.log.info("The following label was added:")
	}
	for i, lbl := range p.addedLabels {
		p.log.info("  " + lbl + " at " + p.addedLabelsLocs[i])
	}
}

func (p *parser) addedLabelsError() {
	msg := "Missing label at the following location:"
	if len(p.addedLabels) > 1 {
		msg = "Missing labels at the following locations:"
	}
	for _, loc := range p.addedLabelsLocs {
		msg += "\n     " + loc
	}
	fail(msg)
}

func (p *parser) uniprocess(name string, decls []*VarDecl, defs *expr, macros []*Macro, fair bool) node {
	up := &Uniprocess{name: name, decls: decls, defs: defs, macros: macros, prcds: p.procedures}
	begin := location{p.algLine, p.algCol}
	p.uniprocessFairness(fair)
	p.gobbleBegin()
	up.body = p.getStmtSeq()
	checkForDuplicateMacros(macros)
	expandMacrosInStmtSeq(up.body, macros)
	p.finishProcedures(macros)
	if p.cSyntax {
		p.gobble("}")
		p.gobble("}")
	} else {
		p.gobble("end")
		p.gobble("algorithm")
	}
	p.addLabelsToStmtSeq(up.body)
	up.body = p.makeLabeledStmtSeq(up.body)
	if len(p.addedLabels) > 0 {
		p.reportAddedLabels(p.hasLabel)
	}
	if p.gotoUsed {
		p.omitPC = false
	}
	if p.gotoDoneUsed {
		p.omitPC = false
		p.omitStutteringWhenDone = false
	} else {
		p.checkBody(up.body)
	}
	up.origin = &region{begin, p.lastLocationEnd()}
	return up
}

func (p *parser) uniprocessFairness(fair bool) {
	if p.params.inputVersion == version15 && p.peekAt(1) == "fair" {
		p.gobble("fair")
		if p.peekAt(1) == "+" {
			p.gobble("+")
		}
		p.params.FairnessOption = "wf"
	}
	if !fair {
		return
	}
	if opt := p.params.FairnessOption; opt != "" && opt != "wf" && opt != "wfNext" {
		p.log.warning("Option `" + opt + "' specified for --fair algorithm.")
	}
	p.params.FairnessOption = "wf"
}

// checkBody decides whether the algorithm can omit pc and the stuttering
// disjunct: its body must be a single "while TRUE" loop without labels.
func (p *parser) checkBody(body *seq) {
	if body == nil || body.len() == 0 {
		return
	}
	w, ok := p.singleWhileTrue(body)
	if !ok {
		return
	}
	if w.labDo != nil && w.labDo.len() > 0 {
		p.omitPC = false
	}
	if w.unlabDo != nil && containsLabels(w.unlabDo) {
		p.omitPC = false
	}
}

func containsLabels(stmts *seq) bool {
	for _, stmt := range stmts.items {
		switch stmt.(type) {
		case *LabelIf, *LabelEither, *LabeledStmt:
			return true
		}
	}
	return false
}

// singleWhileTrue returns the "while TRUE" loop that is the only statement
// of body, or clears both omission flags when there is none.
func (p *parser) singleWhileTrue(body *seq) (*While, bool) {
	lbl, ok := body.at(0).(*LabeledStmt)
	if body.len() > 1 || !ok {
		p.keepPC()
		return nil, false
	}
	if lbl.stmts == nil || lbl.stmts.len() == 0 {
		return nil, false
	}
	w, ok := lbl.stmts.at(0).(*While)
	if lbl.stmts.len() > 1 || !ok || !isSingleTrue(w.test) {
		p.keepPC()
		return nil, false
	}
	return w, true
}

func isSingleTrue(e *expr) bool {
	return len(e.tokens) == 1 && len(e.tokens[0]) == 1 && e.tokens[0][0].str == "TRUE"
}

func (p *parser) keepPC() {
	p.omitPC = false
	p.omitStutteringWhenDone = false
}

func (p *parser) getProcedure() *Procedure {
	result := &Procedure{}
	p.gobble("procedure")
	begin := p.lastLocationStart()
	result.col, result.line = p.lastTokCol, p.lastTokLine
	result.name = p.getAlgToken()
	p.currentProcedure = some(result.name)
	p.resetLabelLists()
	p.gobble("(")
	lookForComma := false
	for p.peekAt(1) != ")" {
		if lookForComma {
			p.gobble(",")
		}
		result.params = append(result.params, p.getPVarDecl())
		lookForComma = true
	}
	p.mustGobble(")")
	if !p.peekIs(1, "begin", "{") {
		result.decls = p.getPVarDecls()
	}
	p.gobbleBegin()
	result.body = p.getStmtSeq()
	p.gobbleEnd("procedure")
	end := p.lastLocationEnd()
	p.skipSemicolon()
	p.currentProcedure = jstring{}
	result.plusLabels, result.minusLabels, result.proceduresCalled = p.plusLabels, p.minusLabels, p.proceduresCalled
	result.origin = &region{begin, end}
	return result
}

func (p *parser) resetLabelLists() {
	p.plusLabels, p.minusLabels, p.proceduresCalled = &strList{}, &strList{}, &strList{}
}

func (p *parser) skipSemicolon() {
	if p.peekAt(1) == ";" {
		p.getAlgToken()
	}
}

func (p *parser) getProcess() *Process {
	result := &Process{}
	p.gobble("process")
	begin := p.lastLocationStart()
	result.col, result.line = p.lastTokCol, p.lastTokLine
	if p.cSyntax {
		p.gobble("(")
	}
	result.name = p.getAlgToken()
	result.isEq = p.gobbleEqualOrIn()
	result.id = p.getExpr()
	p.resetLabelLists()
	if p.cSyntax {
		p.gobble(")")
	}
	if len(result.id.tokens) == 0 {
		p.parsingError("Empty process id at ")
	}
	if !p.peekIs(1, "begin", "{") {
		result.decls = p.getVarDecls()
	}
	p.gobbleBegin()
	result.body = p.getStmtSeq()
	p.gobbleEnd("process")
	end := p.lastLocationEnd()
	p.skipSemicolon()
	result.plusLabels, result.minusLabels, result.proceduresCalled = p.plusLabels, p.minusLabels, p.proceduresCalled
	result.origin = &region{begin, end}
	return result
}

func (p *parser) gobbleVariables() {
	if p.peekAt(1) == "variables" {
		p.mustGobble("variables")
		return
	}
	p.gobble("variable")
}

func (p *parser) getPVarDecls() []*PVarDecl {
	p.gobbleVariables()
	var result []*PVarDecl
	for !p.peekIs(1, "begin", "{") {
		result = append(result, p.getPVarDecl())
		p.gobbleCommaOrSemicolon()
	}
	return result
}

func (p *parser) getPVarDecl() *PVarDecl {
	pv := newPVarDecl()
	pv.name = p.getAlgToken()
	begin, end := p.lastLocationStart(), p.lastLocationEnd()
	pv.col, pv.line = p.lastTokCol, p.lastTokLine
	if p.peekAt(1) == "=" {
		p.gobble("=")
		pv.val = p.getExpr()
		end = pv.val.origin.endLoc()
		if len(pv.val.tokens) == 0 {
			p.parsingError("Missing expression at ")
		}
	} else {
		p.hasDefaultInit = true
	}
	pv.origin = &region{begin, end}
	return pv
}

func (p *parser) getVarDecls() []*VarDecl {
	p.gobbleVariables()
	var result []*VarDecl
	for !p.peekIs(1, "begin", "{", "procedure", "process", "fair", "define", "macro") {
		result = append(result, p.getVarDecl())
	}
	return result
}

func (p *parser) getVarDecl() *VarDecl {
	pv := newVarDecl()
	pv.name = p.getAlgToken()
	begin, end := p.lastLocationStart(), p.lastLocationEnd()
	pv.col, pv.line = p.lastTokCol, p.lastTokLine
	if p.peekIs(1, "=", "\\in") {
		pv.isEq = p.gobbleEqualOrIn()
		pv.val = p.getExpr()
		end = pv.val.origin.endLoc()
		if len(pv.val.tokens) == 0 {
			p.parsingError("Missing expression at ")
		}
	} else {
		p.hasDefaultInit = true
	}
	p.gobbleCommaOrSemicolon()
	pv.origin = &region{begin, end}
	return pv
}

func checkForDuplicateMacros(macros []*Macro) {
	for i, m := range macros {
		for _, other := range macros[i+1:] {
			if m.name == other.name {
				fail("Multiple definitions of macro name `" + m.name + "'")
			}
		}
	}
}
