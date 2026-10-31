package pcal

import (
	"cmp"
	"slices"
)

// sortAssigns orders the assignments of a multiple assignment by variable,
// as Java's SortSass selection sort does.
func sortAssigns(as []*SingleAssign) []*SingleAssign {
	v := slices.Clone(as)
	var r []*SingleAssign
	for len(v) > 0 {
		idx := 0
		for i := 1; i < len(v); i++ {
			if cmp.Compare(v[idx].lhs.name, v[i].lhs.name) > 0 {
				idx = i
			}
		}
		r = append(r, v[idx])
		v = slices.Delete(v, idx, idx+1)
	}
	return r
}

// assignGroup is a run of assignments to the same variable.
type assignGroup struct {
	first, last int
	noSubscript bool
	multiVars   bool
}

func groupAt(as []*SingleAssign, i int) assignGroup {
	g := assignGroup{first: i}
	name := as[i].lhs.name
	lastNoSub := emptyExpr(as[i].lhs.sub)
	j := i
	for j < len(as) && as[j].lhs.name == name {
		g.noSubscript = g.noSubscript || lastNoSub
		j++
		if j < len(as) && emptyExpr(as[j].lhs.sub) {
			lastNoSub = true
		}
	}
	g.multiVars = j != len(as)
	g.last = j - 1
	return g
}

func (g *generator) assign(a *Assign, c *changed, context, prefix string) {
	cThis := c.clone()
	sb := ""
	a.ass = sortAssigns(a.ass)
	g.addToken(prefix)
	hasMultipleVars := false
	for i := 0; i < len(a.ass); {
		grp := g.checkedGroup(a, i, cThis)
		hasMultipleVars = hasMultipleVars || grp.multiVars
		if hasMultipleVars {
			sb += "/\\ "
		}
		if grp.first == grp.last {
			g.singleAssign(a.ass[i], c, context, prefix, sb)
		} else {
			g.groupAssign(a.ass, grp, c, context, prefix, sb, i == 0)
		}
		sb = ""
		i = grp.last + 1
		if i < len(a.ass) {
			g.endLine()
			sb = spaces(jlen(prefix))
		}
	}
	g.endLine()
	c.merge(cThis)
}

// checkedGroup returns the group of assignments starting at i after checking
// that its variable is declared and assigned only once.
func (g *generator) checkedGroup(a *Assign, i int, cThis *changed) assignGroup {
	sf := a.ass[i]
	if !slices.Contains(g.vars, sf.lhs.name) {
		failAt("Assignment to undeclared variable "+sf.lhs.name, sf)
	}
	grp := groupAt(a.ass, i)
	if cThis.set(sf.lhs.name) > 1 || (grp.last > grp.first && grp.noSubscript) {
		failAt("Multiple assignment to "+sf.lhs.name, a)
	}
	return grp
}

func (g *generator) subscriptsFor(e *expr, context string, c *changed) *expr {
	return g.addSubscripts(e, subExpr(g.selfFor(context)), c)
}

// isSelfIndexed reports whether a variable is a function of the process.
func (g *generator) isSelfIndexed(name string) bool {
	return g.mp && (name == "pc" || name == "stack" || g.isProcedureVar(name) || g.isProcessSetVar(name))
}

func (g *generator) singleAssign(sa *SingleAssign, c *changed, context, prefix, sb string) {
	name := sa.lhs.name
	sub := g.subscriptsFor(sa.lhs.sub, context, c)
	rhs := g.subscriptsFor(sa.rhs, context, c)
	switch {
	case g.isSelfIndexed(name):
		g.selfAssign(name, sub, rhs, prefix, sb)
	case !emptyExpr(sa.lhs.sub):
		g.addToken(sb + name + "' = [" + name + " EXCEPT !")
		g.addExpr(sub)
		g.addToken(" = ")
		g.addExpr(rhs)
		g.addToken("]")
	default:
		sb += name + "' = "
		parens := needsParentheses(rhs.lines())
		if parens {
			sb += "("
		}
		g.addToken(sb)
		g.addExpr(rhs)
		if parens {
			g.addToken(")")
		}
	}
}

// selfAssign assigns to a variable indexed by self.
func (g *generator) selfAssign(name string, sub, rhs *expr, prefix, sb string) {
	sb += name + "' = ["
	wrapCol := jlen(sb) + 2
	sb += name + " EXCEPT "
	if jlen(sb)+jlen(prefix) > g.params.SSWrapColumn && len(g.self.lines()) == 0 {
		g.addLine(sb)
		sb = spaces(wrapCol)
	}
	g.addToken(sb + "![")
	g.addExpr(g.self)
	g.addToken("]")
	if len(sub.lines()) > 0 {
		g.addExpr(sub)
	}
	g.addToken(" = ")
	g.addExpr(rhs)
	g.addToken("]")
}

// groupAssign assigns several subscripted parts of one variable.
func (g *generator) groupAssign(as []*SingleAssign, grp assignGroup, c *changed, context, prefix, sb string, first bool) {
	name := as[grp.first].lhs.name
	sb += name + "' = [" + name + " EXCEPT "
	cc := jlen(sb)
	if first {
		cc += jlen(prefix)
	}
	subscript := g.mp && (g.isProcedureVar(name) || g.isProcessSetVar(name))
	for i := grp.first; i <= grp.last; i++ {
		sa := as[i]
		sub := g.subscriptsFor(sa.lhs.sub, context, c)
		rhs := g.subscriptsFor(sa.rhs, context, c)
		sb += "!"
		if subscript {
			g.addToken(sb + "[")
			g.addExpr(g.selfFor(context))
			g.addToken("]")
		} else {
			g.addToken(sb)
		}
		g.addExpr(sub)
		g.addToken(" = ")
		g.addExpr(rhs)
		sb = ""
		if i == grp.last {
			g.addToken("]")
			continue
		}
		g.addToken(",")
		g.endLine()
		sb = spaces(cc)
	}
}

func (g *generator) ifStmt(a *If, c *changed, context, prefix string) {
	cThen, cElse := c.clone(), c.clone()
	test := g.subscriptsFor(a.test, context, c)
	sb := prefix + "IF "
	here := jlen(sb)
	g.addToken(sb)
	g.addExpr(test)
	g.endLine()
	thenHead := spaces(here) + "THEN "
	here = jlen(thenHead)
	sb = thenHead + "/\\ "
	for _, s := range a.then.items {
		g.stmt(s, cThen, context, sb, here+3)
		sb = spaces(here) + "/\\ "
	}
	lineUncThen := len(g.tlacode)
	g.addLine(sb)
	sb = g.elseBranch(a, cElse, context, here)
	g.elseUnchanged(sb, cElse, cThen)
	g.thenUnchanged(lineUncThen, cThen, cElse)
	c.merge(cThen)
	c.merge(cElse)
}

func (g *generator) elseBranch(a *If, cElse *changed, context string, here int) string {
	sb := spaces(here-len("THEN ")) + "ELSE "
	here = jlen(sb)
	if a.els.len() == 0 {
		g.addLine(sb + "/\\ TRUE")
		return spaces(here) + "/\\ "
	}
	sb += "/\\ "
	for _, s := range a.els.items {
		g.stmt(s, cElse, context, sb, here+3)
		sb = spaces(here) + "/\\ "
	}
	return sb
}

func (g *generator) elseUnchanged(sb string, cElse, cThen *changed) {
	switch cElse.numUnchanged(cThen) {
	case 0:
		return
	case 1:
		g.addToken(sb + unchangedConjunct(cElse.unchanged(cThen)))
	default:
		unc := cElse.unchangedLines(cThen, g.wrapColumn()-jlen(sb)-len("UNCHANGED << "))
		g.unchangedTuple(sb+"UNCHANGED << ", unc)
	}
	g.endLine()
}

// thenUnchanged replaces the placeholder line of the THEN branch by its
// UNCHANGED conjunct.
func (g *generator) thenUnchanged(line int, cThen, cElse *changed) {
	sb := g.tlacode[line]
	g.tlacode = slices.Delete(g.tlacode, line, line+1)
	var lines []string
	switch cThen.numUnchanged(cElse) {
	case 0:
		return
	case 1:
		lines = []string{sb + unchangedConjunct(cThen.unchanged(cElse))}
	default:
		unc := cThen.unchangedLines(cElse, g.wrapColumn()-jlen(sb)-len("UNCHANGED << "))
		lines = tupleLines(sb+"UNCHANGED << ", unc)
	}
	g.tlacode = slices.Insert(g.tlacode, line, lines...)
}

// tupleLines formats an UNCHANGED tuple whose variables are split over
// several lines.
func tupleLines(head string, unc []string) []string {
	var lines []string
	sb := head + unc[0]
	for _, u := range unc[1:] {
		lines = append(lines, sb)
		sb = spaces(jlen(head)) + u
	}
	return append(lines, sb+" >>")
}

func (g *generator) either(a *Either, c *changed, context, prefix string) {
	all := c.clone()
	cOrs := make([]*changed, len(a.ors))
	ucLocs := make([]int, len(a.ors))
	prefixIndent := jlen(prefix)
	sb := prefix + "\\/ "
	here := jlen(sb)
	for i, or := range a.ors {
		if i != 0 {
			sb = spaces(prefixIndent) + "\\/ "
		}
		sb += "/\\ "
		cc := c.clone()
		for _, s := range or.items {
			g.stmt(s, cc, context, sb, here+3)
			sb = spaces(here) + "/\\ "
		}
		cOrs[i] = cc
		all.merge(cc)
		ucLocs[i] = len(g.tlacode)
		g.addLine("Replace by UNCHANGED")
	}
	for i := range slices.Backward(a.ors) {
		g.tlacode = slices.Delete(g.tlacode, ucLocs[i], ucLocs[i]+1)
		if line, ok := eitherUnchanged(cOrs[i], all, here); ok {
			g.tlacode = slices.Insert(g.tlacode, ucLocs[i], line)
		}
	}
	c.merge(all)
}

func eitherUnchanged(cOr, all *changed, here int) (string, bool) {
	notChanged := cOr.unchanged(all)
	switch cOr.numUnchanged(all) {
	case 0:
		return "", false
	case 1:
		return spaces(here) + "/\\ " + unchangedConjunct(notChanged), true
	}
	return spaces(here) + "/\\ UNCHANGED <<" + notChanged + ">>", true
}
