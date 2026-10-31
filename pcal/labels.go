package pcal

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

func (p *parser) addLabelsToStmtSeq(stmts *seq) {
	var out []string
	p.innerAddLabels(stmts, true, false, nil, &out)
}

// labelState carries InnerAddLabels's per-sequence variables.
type labelState struct {
	inWith bool
	in     []string
	out    *[]string
	needs  bool
	had    bool
}

// innerAddLabels adds the labels required by the PlusCal labeling rules and
// returns whether the statement after the sequence needs a label.
func (p *parser) innerAddLabels(stmts *seq, firstLabeled, inWith bool, in []string, out *[]string) bool {
	*out = slices.Clone(in)
	st := &labelState{inWith: inWith, in: in, out: out, needs: firstLabeled}
	for _, stmt := range stmts.items {
		p.labelStmt(st, stmt)
	}
	return st.had || st.needs
}

// startStep handles a statement that is or must be labeled, which starts a
// new step.
func (p *parser) startStep(st *labelState, stmt node) {
	if stmt.base().lbl != "" {
		st.had = true
		*st.out = nil
		if st.inWith {
			failAt("Label in `with' statement", stmt)
		}
	}
	if !st.needs {
		return
	}
	if st.inWith {
		failAt("Statement follows `call' or `return' inside a `with' statement.", stmt)
	}
	p.needsLabel(stmt)
	st.needs = false
	st.had = true
	*st.out = nil
}

func (p *parser) labelStmt(st *labelState, stmt node) {
	p.startStep(st, stmt)
	switch s := stmt.(type) {
	case *LabelIf:
		p.labelIf(st, s)
	case *LabelEither:
		p.labelEither(st, s)
	case *While:
		p.labelWhile(st, s)
	case *With:
		p.labelWith(st, s)
	case *Assign:
		p.labelAssigned(st, stmt, assignedVars(s), false, "Second assignment to same variable inside a `with' statement")
	default:
		p.labelOther(st, stmt)
	}
}

func (p *parser) labelIf(st *labelState, s *LabelIf) {
	var thenOut, elseOut []string
	st.needs = p.innerAddLabels(s.unlabThen, false, st.inWith, *st.out, &thenOut)
	st.needs = p.innerAddLabels(s.unlabElse, false, st.inWith, *st.out, &elseOut) || st.needs
	*st.out = union(thenOut, elseOut)
}

func (p *parser) labelEither(st *labelState, s *LabelEither) {
	var newOut []string
	for _, clause := range s.clauses {
		var orOut []string
		st.needs = p.innerAddLabels(clause.unlabOr, false, st.inWith, *st.out, &orOut) || st.needs
		newOut = union(orOut, newOut)
	}
	*st.out = newOut
}

func (p *parser) labelWhile(st *labelState, s *While) {
	if st.inWith {
		failAt("`while' inside a `with' statement", s)
	}
	p.needsLabel(s)
	st.had = true
	*st.out = nil
	var newOut []string
	p.innerAddLabels(s.unlabDo, false, false, *st.out, &newOut)
	st.needs = false
}

func (p *parser) labelWith(st *labelState, s *With) {
	var newIn, newOut []string
	if st.inWith {
		newIn = slices.Clone(st.in)
	}
	st.needs = p.innerAddLabels(s.do, false, true, newIn, &newOut)
	*st.out = newOut
	if !st.inWith && !hasEmptyIntersection(st.in, *st.out) {
		p.needsLabel(s)
		st.had = true
		*st.out = nil
	}
}

func assignedVars(s *Assign) []string {
	var vars []string
	for _, a := range s.ass {
		if !slices.Contains(vars, a.lhs.name) {
			vars = append(vars, a.lhs.name)
		}
	}
	return vars
}

// labelAssigned records the variables a statement assigns, adding a label
// when one was already assigned in the current step.
func (p *parser) labelAssigned(st *labelState, stmt node, vars []string, needs bool, withMsg string) {
	if hasEmptyIntersection(*st.out, vars) {
		*st.out = union(*st.out, vars)
	} else {
		if st.inWith {
			failAt(withMsg, stmt)
		}
		p.needsLabel(stmt)
		st.had = true
		*st.out = slices.Clone(vars)
	}
	st.needs = needs
}

func (p *parser) labelOther(st *labelState, stmt node) {
	isCallOrReturn, setsPrcdVars, needs := p.classifyTransfer(stmt)
	var vars []string
	if isCallOrReturn {
		needs = true
		vars = []string{"stack"}
		if setsPrcdVars {
			vars = append(vars, p.procedureVars()...)
		}
	}
	st.needs = false
	p.labelAssigned(st, stmt, vars, needs, "Call or return makes second assignment to a variable inside a `with' statement")
}

// classifyTransfer reports whether stmt is a call or return, whether it sets
// procedure variables, and whether it is a goto.
func (p *parser) classifyTransfer(stmt node) (callOrReturn, setsPrcdVars, isGoto bool) {
	switch s := stmt.(type) {
	case *Call:
		return true, p.isCurrentProcedure(s.to), false
	case *CallGoto:
		return true, p.isCurrentProcedure(s.to), false
	case *Return, *CallReturn:
		return true, true, false
	case *Goto:
		return false, false, true
	}
	return false, false, false
}

func (p *parser) isCurrentProcedure(name string) bool {
	return p.currentProcedure.ok && p.currentProcedure.val == name
}

func (p *parser) procedureVars() []string {
	var vars []string
	for _, prcd := range p.procedures {
		for _, d := range prcd.params {
			vars = append(vars, d.name)
		}
		for _, d := range prcd.decls {
			vars = append(vars, d.name)
		}
	}
	return vars
}

func (p *parser) needsLabel(stmt node) {
	b := stmt.base()
	if b.lbl != "" {
		return
	}
	lbl := p.nextLabel()
	for p.allLabels[lbl] {
		lbl = p.nextLabel()
	}
	b.lbl = lbl
	p.addedLabels = append(p.addedLabels, lbl)
	p.addedLabelsLocs = append(p.addedLabelsLocs, b.location())
}

func (p *parser) nextLabel() string {
	lbl := p.params.LabelRoot + strconv.Itoa(p.nextLabelNum)
	p.nextLabelNum++
	return lbl
}

func hasEmptyIntersection(s, t []string) bool {
	for _, x := range t {
		if slices.Contains(s, x) {
			return false
		}
	}
	return true
}

func union(s, t []string) []string {
	result := slices.Clone(s)
	for _, x := range t {
		if !slices.Contains(result, x) {
			result = append(result, x)
		}
	}
	return result
}

func (p *parser) makeLabeledStmtSeq(stmts *seq) *seq {
	result := innerMakeLabeledStmtSeq(stmts)
	p.checkLabeledStmtSeq(result)
	return result
}

// innerMakeLabeledStmtSeq groups a statement sequence whose first statement
// is labeled into labeled statements.
func innerMakeLabeledStmtSeq(stmts *seq) *seq {
	result := newSeq()
	for next := 0; next < stmts.len(); {
		stmt := stmts.at(next).base()
		assertBug(stmt.lbl != "", "Missing Label in MakeLabeledStmtSeq")
		ls := &LabeledStmt{label: stmt.lbl, stmts: newSeq()}
		ls.pos = stmt.pos
		begin := stmt.lblLocation
		ls.stmts.add(stmts.at(next))
		for next++; next < stmts.len() && stmts.at(next).base().lbl == ""; next++ {
			ls.stmts.add(stmts.at(next))
		}
		fixStmtSeq(ls.stmts)
		if begin == nil {
			b := ls.stmts.at(0).base().origin.beginLoc()
			begin = &b
		}
		ls.origin = &region{*begin, ls.stmts.last().base().origin.endLoc()}
		result.add(ls)
	}
	return result
}

// splitLabeled moves the leading unlabeled statements of s to a new
// sequence and makes labeled statements of the rest.
func splitLabeled(s *seq) (unlab, lab *seq) {
	i := 0
	for i < s.len() && s.at(i).base().lbl == "" {
		i++
	}
	unlab = newSeq(s.items[:i]...)
	fixStmtSeq(unlab)
	return unlab, innerMakeLabeledStmtSeq(newSeq(s.items[i:]...))
}

func fixStmtSeq(stmts *seq) {
	for _, stmt := range stmts.items {
		switch s := stmt.(type) {
		case *LabelIf:
			s.unlabThen, s.labThen = splitLabeled(s.unlabThen)
			s.unlabElse, s.labElse = splitLabeled(s.unlabElse)
		case *LabelEither:
			for _, c := range s.clauses {
				c.unlabOr, c.labOr = splitLabeled(c.unlabOr)
			}
		case *While:
			s.unlabDo, s.labDo = splitLabeled(s.unlabDo)
		}
	}
}

func (p *parser) checkLabeledStmtSeq(stmts *seq) {
	for _, s := range stmts.items {
		p.classifyStmtSeq(s.(*LabeledStmt).stmts)
	}
}

// classifyStmtSeq returns 0 if the sequence has no call, return, goto, or
// label, 1 if it ends in a call, return, or goto, and 2 if it contains a
// label. It replaces label-free LabelIf and LabelEither nodes with If and
// Either nodes.
func (p *parser) classifyStmtSeq(stmts *seq) int {
	result := 0
	for i := 0; i < stmts.len(); i++ {
		result = p.classifyStmt(stmts, i, result)
		if result == 0 {
			continue
		}
		if i == stmts.len()-1 {
			return result
		}
		reportBug("Translator discovered later than it should have  that\n  Statement at " +
			stmts.at(i+1).base().location() + " must have a label")
	}
	return result
}

func (p *parser) classifyStmt(stmts *seq, i, result int) int {
	switch n := stmts.at(i).(type) {
	case *LabelIf:
		return p.classifyLabelIf(stmts, i, n)
	case *LabelEither:
		return p.classifyLabelEither(stmts, i, n)
	case *With:
		r := p.classifyStmtSeq(n.do)
		if r == 2 {
			fail("with statement at " + n.location() + " contains a labeled statement")
		}
		return r
	case *While:
		p.classifyWhile(i, n)
		return result
	}
	return p.classifySimple(stmts, i, result)
}

// classifySimple classifies the statements other than compound ones.
func (p *parser) classifySimple(stmts *seq, i, result int) int {
	switch n := stmts.at(i).(type) {
	case *Call:
		if _, ok := nextStmt(stmts, i).(*Return); !ok {
			return 1
		}
	case *Return, *CallReturn:
		return p.classifyReturn(n)
	case *Goto, *CallGoto:
		return 1
	case *Assign, *When, *PrintS, *Assert, *Skip, *MacroCall:
	default:
		reportBug("ParseAlgorithm.ClassifyStmtSeq encountered the unexpected statement type " + javaClassName(n))
	}
	return result
}

func nextStmt(stmts *seq, i int) node {
	if i < stmts.len()-1 {
		return stmts.at(i + 1)
	}
	return nil
}

func (p *parser) classifyWhile(i int, n *While) {
	if i != 0 {
		reportBug("ParseAlgorithm.ClassifyStmtSeq encountered `while' not at beginning of StmtSeq.")
	}
	p.classifyStmtSeq(n.unlabDo)
	p.checkLabeledStmtSeq(n.labDo)
}

func (p *parser) classifyReturn(n node) int {
	if !p.currentProcedure.ok {
		fail("return statement not in a procedure at " + n.base().location())
	}
	return 1
}

func (p *parser) classifyLabelIf(stmts *seq, i int, n *LabelIf) int {
	result := p.classifyIf(n)
	if result < 2 {
		newIf := &If{test: n.test, then: n.unlabThen, els: n.unlabElse}
		newIf.origin = n.origin
		stmts.items[i] = newIf
	}
	return result
}

func (p *parser) classifyIf(n *LabelIf) int {
	thenClass := p.classifyStmtSeq(n.unlabThen)
	labeled := false
	if n.labThen.len() > 0 {
		labeled = true
		p.checkLabeledStmtSeq(n.labThen)
	}
	elseClass := p.classifyStmtSeq(n.unlabElse)
	if n.labElse.len() > 0 {
		labeled = true
		p.checkLabeledStmtSeq(n.labElse)
	}
	if labeled || thenClass == 2 || elseClass == 2 {
		return 2
	}
	if thenClass+elseClass > 0 {
		return 1
	}
	return 0
}

func (p *parser) classifyLabelEither(stmts *seq, i int, n *LabelEither) int {
	result := 0
	for _, c := range n.clauses {
		result = max(result, p.classifyStmtSeq(c.unlabOr))
		if c.labOr.len() > 0 {
			result = 2
			p.checkLabeledStmtSeq(c.labOr)
		}
	}
	if result < 2 {
		newEither := &Either{}
		for _, c := range n.clauses {
			newEither.ors = append(newEither.ors, c.unlabOr)
		}
		newEither.origin = n.origin
		stmts.items[i] = newEither
	}
	return result
}

// javaClassName returns the Java class name used in bug reports.
func javaClassName(n node) string {
	return "class pcal.AST$" + strings.TrimPrefix(fmt.Sprintf("%T", n), "*pcal.")
}
