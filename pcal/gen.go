package pcal

import (
	"slices"
	"strings"
)

// generator ports PcalTLAGen without the TLA-to-PlusCal mapping, which does
// not affect the generated text.
type generator struct {
	params                 *Params
	omitPC                 bool
	omitStutteringWhenDone bool
	hasDefaultInit         bool

	tlacode    []string
	nextLine   string
	self       *expr
	selfIsSelf bool
	vars       []string
	pcV, psV   []string
	st         *symTab
	mp         bool

	nextStep        []string
	nextStepSelf    []string
	currentProcName string
}

func (g *generator) generate(ast node, report []string) (code []string, err *Error) {
	err = catch(func() {
		for _, line := range report {
			g.addLine(line)
		}
		switch a := ast.(type) {
		case *Uniprocess:
			g.uniprocess(a)
		case *Multiprocess:
			g.multiprocess(a)
		}
	}, nil)
	return g.tlacode, err
}

func (g *generator) wrapColumn() int { return g.params.WrapColumn }

func (g *generator) addToken(tok string) { g.nextLine += tok }

func (g *generator) addLine(line string) {
	g.endLine()
	if line == "" {
		g.tlacode = append(g.tlacode, "")
		return
	}
	g.addToken(line)
	g.endLine()
}

func (g *generator) endLine() {
	if g.nextLine != "" {
		g.tlacode = append(g.tlacode, g.nextLine)
		g.nextLine = ""
	}
}

// addExpr appends an expression, indenting its continuation lines to the
// current column.
func (g *generator) addExpr(e *expr) {
	sv := e.lines()
	indent := len(g.nextLine)
	next := 0
	if indent != 0 {
		g.nextLine += sv[0]
		next = 1
		if len(sv) > 1 {
			g.endLine()
		}
	}
	if len(sv) > 1 {
		pad := spaces(indent)
		for ; next < len(sv)-1; next++ {
			g.tlacode = append(g.tlacode, pad+sv[next])
		}
		g.nextLine = pad + sv[next]
	} else if indent == 0 {
		g.nextLine += sv[0]
	}
}

func (g *generator) isProcedureVar(v string) bool { return slices.Contains(g.pcV, v) }

func (g *generator) isProcessSetVar(v string) bool { return slices.Contains(g.psV, v) }

func emptyExpr(e *expr) bool { return e == nil || len(e.tokens) == 0 }

func (g *generator) uniprocess(a *Uniprocess) {
	g.mp = false
	g.currentProcName = "Next"
	g.varsAndDefs(a.decls, a.prcds, nil, a.defs)
	g.init(a.decls, a.prcds, nil)
	for _, p := range a.prcds {
		g.procedure(p)
	}
	for _, s := range a.body.items {
		ls := s.(*LabeledStmt)
		g.nextStep = append(g.nextStep, ls.label)
		g.labeledStmt(ls, "")
	}
	g.next()
	g.spec()
	g.termination()
}

func (g *generator) multiprocess(a *Multiprocess) {
	g.mp = true
	g.varsAndDefs(a.decls, a.prcds, a.procs, a.defs)
	g.procSet()
	g.init(a.decls, a.prcds, a.procs)
	for _, p := range a.prcds {
		g.procedure(p)
	}
	for _, p := range a.procs {
		g.process(p)
	}
	g.next()
	g.spec()
	g.termination()
}

func selfAsExpr() *expr { return exprOf(newToken("self", 0, tokIdent)) }

func (g *generator) procedure(a *Procedure) {
	arg := ""
	if g.mp {
		g.self = selfAsExpr()
		g.selfIsSelf = true
		g.nextStepSelf = append(g.nextStepSelf, a.name+"(self)")
		arg = "(self)"
	} else {
		g.nextStep = append(g.nextStep, a.name)
	}
	for _, s := range a.body.items {
		g.labeledStmt(s.(*LabeledStmt), "procedure")
	}
	g.disjunction(a.name+arg+" == ", a.body, arg)
}

// disjunction defines an action as the disjunction of its labeled steps.
func (g *generator) disjunction(head string, body *seq, arg string) {
	g.addToken(head)
	indent := spaces(len(head) + 2)
	for i, s := range body.items {
		disjunct := s.(*LabeledStmt).label + arg
		if i != 0 && len(g.nextLine)+7+len(disjunct) > g.wrapColumn() {
			g.endLine()
		}
		if i != 0 {
			pad := ""
			if g.nextLine == "" {
				pad = indent
			}
			g.addToken(pad + " \\/ ")
		}
		g.addToken(disjunct)
	}
	g.addLine("")
}

func (g *generator) process(a *Process) {
	g.currentProcName = a.name
	arg := "(self)"
	if a.isEq {
		g.self = a.id
		g.selfIsSelf = false
		arg = ""
		g.nextStep = append(g.nextStep, a.name)
	} else {
		g.self = selfAsExpr()
		g.selfIsSelf = true
		g.nextStepSelf = append(g.nextStepSelf, a.name+"(self)")
	}
	for _, s := range a.body.items {
		g.labeledStmt(s.(*LabeledStmt), "process")
	}
	if !g.omitPC {
		g.disjunction(a.name+arg+" == ", a.body, arg)
	}
}

func (g *generator) actionName(a *LabeledStmt, context string) string {
	name := a.label
	if g.omitPC {
		name = g.currentProcName
	}
	if g.mp && (context == "procedure" || g.selfIsSelf) {
		name += "(self)"
	}
	return name
}

func (g *generator) labeledStmt(a *LabeledStmt, context string) {
	head := g.actionName(a, context) + " == "
	col := len(head)
	start := len(g.tlacode)
	prefix := head + "/\\ "
	c := newChanged(g.vars)
	for _, s := range a.stmts.items {
		g.stmt(s, c, context, prefix, len(prefix))
		prefix = spaces(col) + "/\\ "
	}
	g.stepUnchanged(c, a.stmts.len(), start, col)
	g.addLine("")
}

// stepUnchanged adds the UNCHANGED conjunct of a step, or removes the "/\ "
// of a step that has a single conjunct.
func (g *generator) stepUnchanged(c *changed, numStmts, start, col int) {
	switch c.numUnchanged(nil) {
	case 0:
		if numStmts == 1 {
			g.removeConjunction(start, col+3)
		}
	case 1:
		g.addToken(spaces(col) + "/\\ " + unchangedConjunct(c.unchanged(nil)))
	default:
		g.unchangedTuple(spaces(col)+"/\\ UNCHANGED << ", c.unchangedLines(nil, g.wrapColumn()-col-len("/\\ UNCHANGED << ")))
	}
}

// unchangedConjunct states that a single variable is unchanged.
func unchangedConjunct(v string) string {
	if len(v) > 5 {
		return "UNCHANGED " + v
	}
	return v + "' = " + v
}

func (g *generator) unchangedTuple(head string, unc []string) {
	sb := head + unc[0]
	here := len(head)
	for _, u := range unc[1:] {
		g.addLine(sb)
		sb = spaces(here) + u
	}
	g.addToken(sb + " >>")
}

// removeConjunction removes the "/\ " of a definition with one conjunct.
func (g *generator) removeConjunction(start, colAfterAnd int) {
	for i := start; i < len(g.tlacode); i++ {
		line := g.tlacode[i]
		if i == start {
			g.tlacode[i] = line[:colAfterAnd-3] + line[colAfterAnd:]
		} else if len(line) > 3 {
			g.tlacode[i] = line[3:]
		}
	}
}

func (g *generator) stmt(s node, c *changed, context, prefix string, col int) {
	switch a := s.(type) {
	case *Assign:
		g.assign(a, c, context, prefix)
	case *If:
		g.ifStmt(a, c, context, prefix)
	case *Either:
		g.either(a, c, context, prefix)
	case *With:
		g.with(a, c, context, prefix, col)
	case *When:
		g.addToken(prefix)
		g.addExpr(g.addSubscripts(a.exp, subExpr(g.selfFor(context)), c))
		g.endLine()
	case *PrintS:
		exp := g.addSubscripts(a.exp, subExpr(g.selfFor(context)), c)
		g.addToken(prefix + "PrintT(")
		g.addExpr(exp)
		g.addToken(")")
		g.endLine()
	default:
		g.simpleStmt(s, c, context, prefix)
	}
}

func (g *generator) simpleStmt(s node, c *changed, context, prefix string) {
	switch a := s.(type) {
	case *Assert:
		g.assert(a, c, context, prefix)
	case *Skip:
		g.addToken(prefix)
		g.addToken("TRUE")
		g.endLine()
	default:
		reportBug("Unexpected AST type " + astString(s, false, false))
	}
}

func (g *generator) assert(a *Assert, c *changed, context, prefix string) {
	exp := g.addSubscripts(a.exp, subExpr(g.selfFor(context)), c)
	sb := prefix + "Assert("
	g.addToken(sb)
	g.addExpr(exp)
	msg := "\"Failure of assertion at " + a.location() + ".\")"
	if len(g.nextLine)+len(", ")+len(msg) < g.wrapColumn() {
		g.addToken(", " + msg)
	} else {
		g.addToken(", ")
		g.endLine()
		g.addToken(spaces(len(sb)) + msg)
	}
	g.endLine()
}

func (g *generator) with(a *With, c *changed, context, prefix string, col int) {
	exp := g.addSubscripts(a.exp, subExpr(g.selfFor(context)), c)
	if a.isEq {
		g.addToken(prefix + "LET " + a.name + " == ")
		g.addExpr(exp)
		g.addToken(" IN")
	} else {
		g.addToken(prefix + "\\E " + a.name + " \\in ")
		g.addExpr(exp)
		g.addToken(":")
	}
	g.endLine()
	sb := spaces(col + 2)
	if a.do.len() > 1 {
		sb += "/\\ "
	}
	for _, s := range a.do.items {
		g.stmt(s, c, context, sb, len(sb))
		sb = spaces(col+2) + "/\\ "
	}
}

func (g *generator) selfFor(context string) *expr {
	if !g.mp {
		return nil
	}
	if context == "procedure" {
		return selfAsExpr()
	}
	return g.self
}

// subExpr returns "[sub]", keeping sub's anchors as Java does.
func subExpr(sub *expr) *expr {
	if sub == nil {
		return nil
	}
	e := sub.cloneAndNormalize()
	for i, line := range e.tokens {
		for _, t := range line {
			t.column++
		}
		if i == 0 {
			e.tokens[0] = append([]*token{newToken("[", 0, tokBuiltin)}, line...)
		}
	}
	e.addTokenOffset(newToken("]", 0, tokBuiltin), 0)
	return e
}

// addSubscripts primes changed variables and subscripts process-local
// variables of an expression.
func (g *generator) addSubscripts(src, sub *expr, c *changed) *expr {
	e := src.cloneAndNormalize()
	var exprs []*expr
	var names []string
	for _, line := range e.tokens {
		for _, t := range line {
			if x := g.subscripted(t, sub, c); x != nil && !slices.Contains(names, t.str) {
				names = append(names, t.str)
				exprs = append(exprs, x)
			}
		}
	}
	if len(exprs) > 0 {
		e.substituteForAll(exprs, names, false)
	}
	return e
}

// needsSubscript reports whether t is pc, stack, or a process-local
// variable that must be indexed by self.
func (g *generator) needsSubscript(t *token, sub *expr) bool {
	if sub == nil {
		return false
	}
	if t.typ == tokAdded {
		return true
	}
	return g.mp && t.typ == tokIdent && (g.isProcedureVar(t.str) || g.isProcessSetVar(t.str))
}

// subscripted returns the replacement of token t, or nil if t is kept.
func (g *generator) subscripted(t *token, sub *expr, c *changed) *expr {
	prime := t.typ == tokIdent && c.isChanged(t.str)
	subr := g.needsSubscript(t, sub)
	if !subr && !prime {
		return nil
	}
	nt := t.clone()
	nt.source, nt.column = nil, 0
	x := &expr{tokens: [][]*token{{nt}}}
	if prime {
		x.addTokenOffset(newToken("'", 0, tokBuiltin), 0)
	}
	if !subr {
		return x
	}
	subexp := sub.cloneAndNormalize()
	x.normalize()
	subexp.prepend(x, 0)
	return subexp
}

// javaSplitLines splits s at newlines like Java's String.split, which drops
// trailing empty strings.
func javaSplitLines(s string) []string {
	parts := strings.Split(s, "\n")
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}
