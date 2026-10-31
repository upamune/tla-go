package pcal

import "strings"

func (g *generator) stutteringDisjunct() bool {
	return !g.params.NoDoneDisjunct && !g.omitStutteringWhenDone
}

func (g *generator) next() {
	if g.omitPC && !g.mp {
		return
	}
	if g.stutteringDisjunct() {
		g.terminating()
	}
	nextS := wrapDisjuncts(g.nextStep, g.wrapColumn()-len("Next == \\/ "), len(" \\/ "))
	nextSS := g.procedureSteps()
	sb := "Next == "
	for _, s := range nextS {
		g.addLine(sb + s)
		sb = spaces(nextCol) + " \\/ "
	}
	if len(nextSS) > 0 {
		sb = g.procedureDisjunct(sb, nextSS)
	}
	sb = g.processSetDisjuncts(sb, len(nextS) == 0 && len(nextSS) == 0)
	if g.stutteringDisjunct() {
		g.addLine(sb + "Terminating")
	}
	g.addLine("")
}

// nextCol is the column of the disjuncts of Next.
const nextCol = len("Next == ") + 2

func (g *generator) terminating() {
	g.addLine("(* Allow infinite stuttering to prevent deadlock on termination. *)")
	if g.mp {
		g.addLine("Terminating == /\\ \\A self \\in ProcSet: pc[self] = \"Done\"")
		g.addLine(spaces(len("Terminating == ")) + "/\\ UNCHANGED vars")
	} else {
		g.addLine("Terminating == pc = \"Done\" /\\ UNCHANGED vars")
	}
	g.addLine("")
}

// procedureDisjunct adds the disjunct for the steps of all procedures.
func (g *generator) procedureDisjunct(sb string, nextSS []string) string {
	sb += "(\\E self \\in ProcSet: "
	col2 := jlen(sb)
	if len(nextSS) > 1 {
		sb += " \\/ "
	}
	for _, s := range nextSS {
		g.addLine(sb + s)
		sb = spaces(col2) + " \\/ "
	}
	return spaces(nextCol) + " \\/ "
}

// processSetDisjuncts adds a disjunct for each process set. first reports
// whether no disjunct precedes them.
func (g *generator) processSetDisjuncts(sb string, first bool) string {
	for i, v := range g.processSetSteps() {
		for _, line := range v {
			g.addLine(sb + line)
			sb = spaces(nextCol + 4)
			if first && i == 0 {
				sb = spaces(nextCol - 2)
			}
		}
		sb = spaces(nextCol) + " \\/ "
	}
	return sb
}

// wrapDisjuncts joins names with " \/ " into lines of at most limit
// characters; extra is the width allowance Java adds to each name.
func wrapDisjuncts(names []string, limit, extra int) []string {
	var lines []string
	sb := ""
	for _, a := range names {
		if jlen(a)+extra+jlen(sb) > limit {
			lines = append(lines, sb)
			sb = ""
		}
		if sb != "" {
			sb += " \\/ "
		}
		sb += a
	}
	if sb != "" {
		lines = append(lines, sb)
	}
	return lines
}

func (g *generator) procedureSteps() []string {
	if !g.mp || len(g.st.procs) == 0 {
		return nil
	}
	names := make([]string, len(g.st.procs))
	for i, p := range g.st.procs {
		names[i] = p.name + "(self)"
	}
	lines := wrapDisjuncts(names, g.wrapColumn()-len("Next == \\/ (\\E self \\in ProcSet: \\/ "), len(" \\/ "))
	if len(lines) > 0 {
		lines[len(lines)-1] += ")"
	}
	return lines
}

func (g *generator) processSetSteps() [][]string {
	if !g.mp {
		return nil
	}
	var result [][]string
	for _, p := range g.st.processes {
		if p.isEq {
			continue
		}
		var vec []string
		sb := "(\\E self \\in "
		col := jlen(sb)
		sv := p.id.lines()
		sb += sv[0]
		for _, s := range sv[1:] {
			vec = append(vec, sb)
			sb = spaces(col) + s
		}
		vec = append(vec, sb+": "+p.name+"(self))")
		result = append(result, vec)
	}
	return result
}

const safetyFormula = "Init /\\ [][Next]_vars"

// wfNext reports whether Spec requires weak fairness of Next.
func (g *generator) wfNext() bool {
	opt := g.params.FairnessOption
	return opt == "wfNext" || g.params.FairAlgorithm || (!g.mp && (opt == "wf" || opt == "sf"))
}

func (g *generator) spec() {
	opt := g.params.FairnessOption
	if opt == "nof" || (!g.mp && opt == "") {
		g.unfairSpec()
		return
	}
	wfNext := g.wfNext()
	var fairness []*processFairness
	if g.mp {
		fairness = g.processFairness()
	}
	if !wfNext && len(fairness) == 0 {
		g.unfairSpec()
		return
	}
	g.fairSpec(wfNext, fairness)
}

func (g *generator) unfairSpec() {
	g.addLine("Spec == " + safetyFormula)
	g.addLine("")
}

func (g *generator) fairSpec(wfNext bool, fairness []*processFairness) {
	g.addLine("Spec == /\\ " + safetyFormula)
	indent := len("Spec == /\\ ")
	if wfNext {
		g.addLine("        /\\ WF_vars(Next)")
	}
	for _, f := range fairness {
		for _, line := range javaSplitLines("        /\\ " + f.format(indent, g.wrapColumn())) {
			g.addLine(line)
		}
	}
	g.addLine("")
}

func (g *generator) termination() {
	if g.omitPC || g.omitStutteringWhenDone {
		return
	}
	pc := "pc"
	if g.mp {
		pc = "\\A self \\in ProcSet: pc[self]"
	}
	g.addLine("Termination == <>(" + pc + " = \"Done\")")
	g.addLine("")
}

// minusLabelGuard returns the "(pc[self] # ...) /\ " guard that excludes
// the steps of the minus labels.
func minusLabelGuard(qSelf string, labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	if len(labels) == 1 {
		return "(pc[" + qSelf + "] # \"" + labels[0] + "\") /\\ "
	}
	return "(pc[" + qSelf + "] \\notin {\"" + strings.Join(labels, "\", \"") + "\"}) /\\ "
}

// plusLabelConjuncts returns the strong fairness conditions of plus labels.
func plusLabelConjuncts(labels []string, arg string) (string, bool) {
	if len(labels) == 0 {
		return "", false
	}
	parts := make([]string, len(labels))
	for i, l := range labels {
		parts[i] = "SF_vars(" + l + arg + ")"
	}
	return strings.Join(parts, " /\\ "), true
}

func (g *generator) processFairness() []*processFairness {
	var result []*processFairness
	for _, p := range g.st.processes {
		if p.ast.fairness != unfairProc {
			result = append(result, g.fairnessOf(p))
		}
	}
	return result
}

func (g *generator) fairnessOf(p *processEntry) *processFairness {
	xf := "SF"
	if p.ast.fairness == wfProc {
		xf = "WF"
	}
	pSelf := p.id.lines()
	qSelf := "self"
	if p.isEq && len(pSelf) <= 1 {
		qSelf = pSelf[0]
	}
	pf := &processFairness{prefix: fairnessPrefix(p, pSelf)}
	name, arg := p.name+"(self)", "(self)"
	if p.isEq {
		name, arg = p.name, ""
	}
	pf.body = formulaPair{wf: xf + "_vars(" + minusLabelGuard(qSelf, p.ast.minusLabels.items) + name + ")"}
	if xf == "WF" {
		pf.body.sf, pf.body.hasSF = plusLabelConjuncts(p.ast.plusLabels.items, arg)
	}
	for _, called := range p.ast.proceduresCalled.items {
		pe := g.st.procs[g.st.findProc(g.st.useThis(symProcedure, called, ""))]
		f := formulaPair{wf: xf + "_vars(" + minusLabelGuard(qSelf, pe.ast.minusLabels.items) + pe.name + "(" + qSelf + "))"}
		if xf == "WF" {
			f.sf, f.hasSF = plusLabelConjuncts(pe.ast.plusLabels.items, "("+qSelf+")")
		}
		pf.procedures = append(pf.procedures, f)
	}
	return pf
}

// fairnessPrefix returns the lines that bind self in a fairness formula.
func fairnessPrefix(p *processEntry, pSelf []string) []string {
	makeLetIn := p.isEq && len(pSelf) > 1
	if !makeLetIn && p.isEq {
		return nil
	}
	if makeLetIn {
		return append(bindSelf(pSelf, "LET self == ", ""), "IN ")
	}
	return bindSelf(pSelf, "\\A self \\in ", " : ")
}

// bindSelf puts begin before the lines of the process id and end after
// them.
func bindSelf(pSelf []string, begin, end string) []string {
	prefix := make([]string, len(pSelf))
	for j, line := range pSelf {
		pad := spaces(jlen(begin))
		if j == 0 {
			pad = begin
		}
		prefix[j] = pad + line
	}
	prefix[len(prefix)-1] += end
	return prefix
}

// formulaPair ports PcalTLAGen.FormulaPair.
type formulaPair struct {
	wf, sf string
	hasSF  bool
}

func (f formulaPair) singleLine() string {
	if !f.hasSF {
		return f.wf
	}
	return f.wf + " /\\ " + f.sf
}

func (f formulaPair) singleLineWidth() int { return jlen(f.singleLine()) }

func (f formulaPair) multiLine(col int) string {
	val := "/\\ " + f.wf
	if !f.hasSF {
		return val
	}
	return val + "\n" + spaces(col) + "/\\ " + f.sf
}

// processFairness ports PcalTLAGen.ProcessFairness.
type processFairness struct {
	prefix     []string
	body       formulaPair
	procedures []formulaPair
}

func (pf *processFairness) singleLineWidth() int {
	maxPrefix, width := 0, 0
	for i := 0; i < len(pf.prefix)-1; i++ {
		maxPrefix = max(maxPrefix, jlen(pf.prefix[i]))
		width = jlen(pf.prefix[len(pf.prefix)-1])
	}
	width += jlen(pf.body.wf)
	if pf.body.hasSF {
		width += jlen(pf.body.sf)
	}
	for _, f := range pf.procedures {
		width += f.singleLineWidth()
	}
	return max(maxPrefix, width)
}

func (pf *processFairness) prefixText(col int) string {
	var sb strings.Builder
	for i, line := range pf.prefix {
		if i != 0 {
			sb.WriteString(spaces(col))
		}
		sb.WriteString(line)
		if i != len(pf.prefix)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func (pf *processFairness) singleLine(col int) string {
	var val strings.Builder
	val.WriteString(pf.prefixText(col) + pf.body.singleLine())
	for _, f := range pf.procedures {
		val.WriteString(" /\\ " + f.singleLine())
	}
	return val.String()
}

func (pf *processFairness) format(col, wrap int) string {
	if col+pf.singleLineWidth() <= wrap || (!pf.body.hasSF && len(pf.procedures) == 0) {
		return pf.singleLine(col)
	}
	var val strings.Builder
	val.WriteString(pf.prefixText(col))
	curCol := col
	if len(pf.prefix) > 0 {
		curCol += jlen(pf.prefix[len(pf.prefix)-1])
	}
	val.WriteString(pairText(pf.body, curCol, wrap, ""))
	for i, f := range pf.procedures {
		if i == 0 {
			val.WriteString("\n")
		}
		val.WriteString(spaces(curCol) + pairText(f, curCol, wrap, "\n"))
	}
	return val.String()
}

// pairText formats a formula pair on one line if it fits, followed by
// newline, or on two lines otherwise.
func pairText(f formulaPair, col, wrap int, newline string) string {
	line := f.singleLine()
	if col+jlen(line)+3 <= wrap {
		return "/\\ " + line + newline
	}
	return f.multiLine(col)
}

// needsParentheses reports whether an initial value or right-hand side must
// be parenthesized. Like Java, it only examines the first line.
func needsParentheses(lines []string) bool {
	if len(lines) == 0 {
		return false
	}
	s := &parenScanner{}
	for n := 0; n < len(lines) && !s.need; n++ {
		s.scan(lines[0])
		if s.inString {
			s.need = true
		}
	}
	return s.need
}

type parenScanner struct {
	depth    int
	inString bool
	need     bool
}

func (s *parenScanner) scan(line string) {
	for i := 0; i < len(line) && !s.need; i++ {
		if s.inString {
			i += s.stringChar(line[i])
			continue
		}
		i += s.char(line, i)
	}
}

func (s *parenScanner) stringChar(c byte) int {
	switch c {
	case '"':
		s.inString = false
	case '\\':
		return 1
	}
	return 0
}

// char classifies the character at i and returns how many extra characters
// it consumed.
func (s *parenScanner) char(line string, i int) int {
	next := byte(' ')
	if i < len(line)-1 {
		next = line[i+1]
	}
	kind, skip := charKind(line, i, next)
	switch kind {
	case parenQuote:
		s.inString = true
	case parenMay:
		s.need = s.need || s.depth == 0
	case parenLeft:
		s.depth++
	case parenRight:
		s.need = s.need || s.depth == 0
		s.depth--
	}
	return skip
}

const (
	parenNone = iota
	parenQuote
	parenMay
	parenLeft
	parenRight
)

var simpleParenKinds = map[byte]int{
	'"': parenQuote, '=': parenMay, '#': parenMay,
	'(': parenLeft, '[': parenLeft, '{': parenLeft,
	')': parenRight, ']': parenRight, '}': parenRight,
}

type charClassifier func(line string, i int, next byte) (kind, skip int)

// angle classifies "<" and ">", which are parentheses when doubled.
func angle(paren int) charClassifier {
	return func(line string, i int, next byte) (int, int) {
		if next != line[i] {
			return parenMay, 0
		}
		return paren, 1
	}
}

var specialParenKinds = map[byte]charClassifier{
	'<': angle(parenLeft),
	'>': angle(parenRight),
	'|': func(line string, i int, next byte) (int, int) {
		return mayIf(next == '-' || (i > 0 && line[i-1] == '-')), 0
	},
	'\\': func(_ string, _ int, next byte) (int, int) {
		return mayIf(next != ' ' && next != 'o' && next != 'X'), 0
	},
	'/': func(_ string, _ int, next byte) (int, int) { return mayIf(next == '\\'), 0 },
}

func charKind(line string, i int, next byte) (kind, skip int) {
	c := line[i]
	if k, ok := simpleParenKinds[c]; ok {
		return k, 0
	}
	if f, ok := specialParenKinds[c]; ok {
		return f(line, i, next)
	}
	return parenNone, 0
}

func mayIf(b bool) int {
	if b {
		return parenMay
	}
	return parenNone
}
