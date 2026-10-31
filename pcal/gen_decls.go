package pcal

func (g *generator) globalVars(globals []*VarDecl, procs []*Procedure) []string {
	var vars []string
	for _, d := range globals {
		vars = append(vars, d.name)
	}
	if !g.omitPC {
		vars = append(vars, "pc")
	}
	if len(procs) > 0 {
		vars = append(vars, "stack")
	}
	return vars
}

// localVars returns the procedure and process variables, recording which
// of them are indexed by self.
func (g *generator) localVars(procs []*Procedure, processes []*Process) []string {
	var vars []string
	for _, p := range procs {
		for _, d := range procedureVars(p) {
			vars = append(vars, d.name)
			g.pcV = append(g.pcV, d.name)
		}
	}
	for _, p := range processes {
		for _, d := range p.decls {
			vars = append(vars, d.name)
			if !p.isEq {
				g.psV = append(g.psV, d.name)
			}
		}
	}
	return vars
}

// procedureVars returns the parameters and then the variables of p.
func procedureVars(p *Procedure) []*PVarDecl {
	return append(append([]*PVarDecl(nil), p.params...), p.decls...)
}

func (g *generator) varsAndDefs(globals []*VarDecl, procs []*Procedure, processes []*Process, defs *expr) {
	gVars := g.globalVars(globals, procs)
	lVars := g.localVars(procs, processes)
	g.vars = append(append(g.vars, gVars...), lVars...)
	if g.hasDefaultInit {
		g.addLine("CONSTANT defaultInitValue")
	}
	gVars = g.declareVariables(gVars, lVars, defs)
	g.addLine("")
	if len(gVars) == 0 {
		fail("The algorithm has no variables.")
	}
	g.varsTuple(gVars)
}

// declareVariables declares the variables, putting procedure and process
// variables after a define block, and returns all variables.
func (g *generator) declareVariables(gVars, lVars []string, defs *expr) []string {
	if emptyExpr(defs) {
		gVars = append(gVars, lVars...)
		g.varDecl(gVars)
		return gVars
	}
	g.varDecl(gVars)
	g.addLine("")
	g.addLine("(* define statement *)")
	g.addExpr(defs)
	g.addLine("")
	g.varDecl(lVars)
	return append(gVars, lVars...)
}

func (g *generator) varsTuple(vars []string) {
	g.addToken("vars == << ")
	indent := jlen(g.nextLine)
	for i, v := range vars {
		if i > 0 {
			g.addToken(", ")
		}
		if jlen(g.nextLine)+jlen(v)+1 > g.wrapColumn() {
			g.endLine()
			g.nextLine = spaces(indent)
		}
		g.addToken(v)
	}
	if jlen(g.nextLine)+len(" >>")+1 > g.wrapColumn() {
		g.endLine()
		g.nextLine = spaces(len("vars =="))
	}
	g.addToken(" >>")
	g.addLine("")
}

func (g *generator) varDecl(vars []string) {
	if len(vars) == 0 {
		return
	}
	kw := "VARIABLE "
	if len(vars) > 1 {
		kw = "VARIABLES "
	}
	g.addToken(kw)
	for i, v := range vars {
		if i > 0 {
			g.addToken(", ")
		}
		if jlen(g.nextLine)+jlen(v)+1 > g.wrapColumn() {
			g.endLine()
			g.nextLine += spaces(len(kw))
		}
		g.addToken(v)
	}
	g.endLine()
}

func (g *generator) procSet() {
	if len(g.st.processes) == 0 {
		return
	}
	g.addToken("ProcSet == ")
	for i, p := range g.st.processes {
		if i > 0 {
			g.addToken(" \\cup ")
		}
		open, closing := "(", ")"
		if p.isEq {
			open, closing = "{", "}"
		}
		g.addToken(open)
		g.addExpr(p.id)
		g.addToken(closing)
	}
	g.endLine()
	g.addLine("")
}

const initIndent = len("Init == ")

func (g *generator) init(globals []*VarDecl, procs []*Procedure, processes []*Process) {
	is := "Init == "
	if len(globals) > 0 {
		g.addLine(is + "(* Global variables *)")
		for _, d := range globals {
			g.varDeclInit(d)
		}
		is = spaces(initIndent)
	}
	for _, p := range procs {
		is = g.procedureInit(p, is)
	}
	for _, p := range processes {
		is = g.processInit(p, is)
	}
	if len(procs) > 0 {
		if g.mp {
			g.addLine(is + "/\\ stack = [self \\in ProcSet |-> << >>]")
		} else {
			g.addLine(is + "/\\ stack = << >>")
		}
		is = spaces(initIndent)
	}
	if !g.omitPC {
		g.pcInit(is)
	}
	g.addLine("")
}

func (g *generator) varDeclInit(d *VarDecl) {
	g.addToken(spaces(initIndent) + "/\\ ")
	op := " = "
	if !d.isEq {
		op = " \\in "
	}
	g.addToken(d.name + op)
	parens := needsParentheses(d.val.lines())
	if parens {
		g.addToken("(")
	}
	g.addExpr(d.val)
	if parens {
		g.addToken(")")
	}
	g.endLine()
}

func (g *generator) procedureInit(p *Procedure, is string) string {
	if len(p.params) == 0 && len(p.decls) == 0 {
		return is
	}
	g.addLine(is + "(* Procedure " + p.name + " *)")
	for _, d := range procedureVars(p) {
		if !g.mp {
			g.varDeclInit(d.toVarDecl())
			continue
		}
		g.addToken(spaces(initIndent) + "/\\ ")
		g.addToken(d.name + " = [ self \\in ProcSet |-> ")
		g.addExpr(g.addSubscripts(d.val, subExpr(g.selfFor("procedure")), newChanged(nil)))
		g.addToken("]")
		g.endLine()
	}
	return spaces(initIndent)
}

func (g *generator) processInit(p *Process, is string) string {
	if len(p.decls) == 0 {
		return is
	}
	g.addLine(is + "(* Process " + p.name + " *)")
	for _, d := range p.decls {
		g.addToken(spaces(initIndent) + "/\\ ")
		switch {
		case p.isEq:
			op := " = "
			if !d.isEq {
				op = " \\in "
			}
			g.addToken(d.name + op)
			g.addExpr(d.val)
		case d.isEq:
			g.addToken(d.name + " = [self \\in ")
			g.addExpr(p.id)
			g.addToken(" |-> ")
			g.addExpr(g.addSubscripts(d.val, subExpr(g.selfFor("procedure")), newChanged(nil)))
			g.addToken("]")
		default:
			g.addToken(d.name + " \\in [")
			g.addExpr(p.id)
			g.addToken(" -> ")
			g.addExpr(g.addSubscripts(d.val, chooseSelf(p.id), newChanged(nil)))
			g.addToken("]")
		}
		g.endLine()
	}
	return spaces(initIndent)
}

// chooseSelf returns "[CHOOSE self \in id : TRUE]", which replaces self in
// the initial value of a "\in" variable of a process set.
func chooseSelf(id *expr) *expr {
	sub := id.cloneAndNormalize()
	head := exprOf(newToken("[", 0, tokBuiltin), newToken("CHOOSE", 1, tokBuiltin),
		newToken("self", 8, tokIdent), newToken("\\in ", 13, tokBuiltin))
	head.origin = sub.origin
	sub.prepend(head, 1)
	tail := &expr{tokens: [][]*token{{newToken(":", 0, tokBuiltin), newToken("TRUE", 2, tokBuiltin), newToken("]", 6, tokBuiltin)}}}
	tail.prepend(sub, 1)
	return tail
}

func (g *generator) pcInit(is string) {
	if !g.mp {
		g.addLine(is + "/\\ pc = \"" + g.st.iPC.String() + "\"")
		return
	}
	useCase := len(g.st.processes) != 1
	if useCase {
		is += "/\\ pc = [self \\in ProcSet |-> CASE "
	} else {
		is += "/\\ pc = [self \\in ProcSet |-> "
	}
	colPC := jlen(is) - 3
	for p, pe := range g.st.processes {
		last := p == len(g.st.processes)-1
		g.addToken(g.pcCase(is, pe, useCase, last))
		g.endLine()
		is = spaces(colPC)
		if !last {
			is += "[] "
		}
	}
}

// pcCase returns the initial pc of a process, as a CASE arm when useCase.
func (g *generator) pcCase(is string, pe *processEntry, useCase, last bool) string {
	if !useCase {
		return is + "\"" + pe.iPC.String() + "\"]"
	}
	g.addToken(is + "self " + caseOp(pe.isEq))
	g.addExpr(pe.id)
	arm := " -> \"" + pe.iPC.String() + "\""
	if last {
		arm += "]"
	}
	return arm
}

func caseOp(isEq bool) string {
	if isEq {
		return "= "
	}
	return "\\in "
}
