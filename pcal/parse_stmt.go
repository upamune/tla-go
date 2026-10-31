package pcal

var stmtParsers map[string]func(*parser) node

func init() {
	stmtParsers = map[string]func(*parser) node{
		"if":     func(p *parser) node { return p.getIf(0) },
		"either": (*parser).getEither,
		"with":   func(p *parser) node { return p.getWith(0, location{}) },
		"when":   func(p *parser) node { return p.getWhen("when") },
		"await":  func(p *parser) node { return p.getWhen("await") },
		"print":  (*parser).getPrint,
		"assert": (*parser).getAssert,
		"skip":   (*parser).getSkip,
		"call":   (*parser).getCallOrCallReturnOrCallGoto,
		"return": (*parser).getReturn,
		"goto":   (*parser).getGoto,
		"while":  (*parser).getWhile,
	}
}

var labelStopTokens = map[string]bool{
	"while": true, "if": true, "assert": true, "print": true, "else": true, "elsif": true,
	"either": true, "or": true, "end": true, "with": true, "when": true, "await": true,
	"skip": true, "call": true, "return": true, "goto": true,
}

func (p *parser) stmtSeqEnds(tok string) bool {
	if p.pSyntax {
		return tok == "end" || tok == "else" || tok == "elsif" || tok == "or"
	}
	return p.cSyntax && tok == "}"
}

func (p *parser) getStmtSeq() *seq {
	result := newSeq()
	for !p.stmtSeqEnds(p.peekAt(1)) {
		label := p.getLabel()
		labelLoc := p.labelLocation
		if p.cSyntax && p.peekAt(1) == "{" {
			result.add(p.getCStmtSeq(label).items...)
			continue
		}
		stmt := p.getStmt()
		stmt.base().lbl = label
		stmt.base().lblLocation = labelLoc
		result.add(stmt)
	}
	if p.cSyntax && result.len() == 0 {
		p.parsingError("Empty statement list")
	}
	return result
}

func (p *parser) getStmt() node {
	if parse, ok := stmtParsers[p.peekAt(1)]; ok {
		return parse(p)
	}
	if p.peekPast(1)[0] == '(' {
		return p.getMacroCall()
	}
	return p.getAssign()
}

func (p *parser) getCStmtSeq(label string) *seq {
	labelLoc := p.labelLocation
	p.mustGobble("{")
	stmts := p.getStmtSeq()
	p.gobble("}")
	p.gobble(";")
	if label != "" {
		first := stmts.at(0)
		if first.base().lbl != "" {
			failAt("Duplicate labeling of statement", first)
		}
		first.base().lbl = label
		first.base().lblLocation = labelLoc
	}
	return stmts
}

func (p *parser) getCStmt() *seq {
	label := p.getLabel()
	labelLoc := p.labelLocation
	if p.peekAt(1) == "{" {
		return p.getCStmtSeq(label)
	}
	stmt := p.getStmt()
	stmt.base().lbl = label
	stmt.base().lblLocation = labelLoc
	return newSeq(stmt)
}

func (p *parser) isLabelNext() bool {
	if labelStopTokens[p.peekAt(1)] {
		return false
	}
	tok := p.r.peek()
	return tok != "" && tok[0] == ':' && (len(tok) < 2 || tok[1] != '=')
}

func (p *parser) getLabel() string {
	if !p.isLabelNext() {
		return ""
	}
	label := p.getAlgToken()
	p.labelLocation = &location{p.lastTokLine - 1, p.lastTokCol - 1}
	if p.inGetMacro {
		p.parsingError("A label may not appear in a macro.")
	}
	if label == "Done" || label == "Error" {
		p.parsingError("Cannot use `" + label + "' as a label.")
	}
	p.gobble(":")
	p.hasLabel = true
	p.allLabels[label] = true
	switch p.peekAt(1) {
	case "+":
		p.gobble("+")
		p.plusLabels.add(label)
		p.omitPC = false
	case "-":
		p.gobble("-")
		p.minusLabels.add(label)
		p.omitPC = false
	}
	return label
}

// startStmt consumes the keyword of a statement and records its position.
func (p *parser) startStmt(kw string, b *nodeBase) location {
	p.mustGobble(kw)
	b.col, b.line = p.lastTokCol, p.lastTokLine
	return p.lastLocationStart()
}

// getTest reads a condition, which C syntax puts in parentheses.
func (p *parser) getTest() *expr {
	if p.cSyntax {
		p.gobble("(")
	}
	test := p.getExpr()
	if p.cSyntax {
		p.gobble(")")
	}
	return test
}

func (p *parser) getWhile() node {
	result := &While{}
	begin := p.startStmt("while", &result.nodeBase)
	result.test = p.getTest()
	if len(result.test.tokens) == 0 {
		p.parsingError("Empty while test at ")
	}
	if p.pSyntax {
		p.gobble("do")
		result.unlabDo = p.getStmtSeq()
		p.gobble("end")
		p.gobble("while")
		p.gobble(";")
	} else {
		result.unlabDo = p.getCStmt()
	}
	if result.unlabDo.len() == 0 {
		p.parsingError("Missing body of while statement at")
	}
	result.labDo = newSeq()
	result.origin = &region{begin, result.unlabDo.last().base().origin.endLoc()}
	return result
}

func (p *parser) getIf(depth int) *LabelIf {
	kw := "if"
	if depth > 0 {
		kw = "elsif"
	}
	result := &LabelIf{}
	begin := p.startStmt(kw, &result.nodeBase)
	result.test = p.getTest()
	if len(result.test.tokens) == 0 {
		p.parsingError("Empty if test at ")
	}
	if p.pSyntax {
		p.gobble("then")
		result.unlabThen = p.getStmtSeq()
		result.unlabElse = p.getPElse(depth)
	} else {
		result.unlabThen = p.getCStmt()
		result.unlabElse = p.getCElse()
	}
	result.labThen, result.labElse = newSeq(), newSeq()
	last := result.unlabThen
	if result.unlabElse.len() != 0 {
		last = result.unlabElse
	}
	result.origin = &region{begin, last.last().base().origin.endLoc()}
	return result
}

func (p *parser) getPElse(depth int) *seq {
	var result *seq
	switch p.peekAt(1) {
	case "else":
		p.mustGobble("else")
		result = p.getStmtSeq()
	case "elsif":
		result = newSeq(p.getIf(depth + 1))
	case "end":
		result = newSeq()
	default:
		p.parsingError("Expecting \"else\", \"elsif\", or \"end\"")
	}
	if depth == 0 {
		p.gobble("end")
		p.gobble("if")
		p.gobble(";")
	}
	return result
}

func (p *parser) getCElse() *seq {
	if p.peekAt(1) != "else" {
		return newSeq()
	}
	p.mustGobble("else")
	return p.getCStmt()
}

func (p *parser) getEither() node {
	result := &LabelEither{}
	begin := p.startStmt("either", &result.nodeBase)
	hasOr := false
	for {
		clause := &Clause{labOr: newSeq()}
		if p.pSyntax {
			clause.unlabOr = p.getStmtSeq()
		} else {
			clause.unlabOr = p.getCStmt()
		}
		if clause.unlabOr.len() == 0 {
			failAt("`either' statement with empty `or' clause", result)
		}
		clause.origin = &region{clause.unlabOr.at(0).base().origin.beginLoc(), clause.unlabOr.last().base().origin.endLoc()}
		result.clauses = append(result.clauses, clause)
		if p.peekAt(1) != "or" {
			break
		}
		p.mustGobble("or")
		hasOr = true
	}
	if p.pSyntax {
		p.mustGobble("end")
		p.gobble("either")
		p.gobble(";")
	}
	if !hasOr {
		failAt("`either' statement has no `or'", result)
	}
	result.origin = &region{begin, result.clauses[len(result.clauses)-1].origin.endLoc()}
	return result
}

func (p *parser) getWith(depth int, begin location) node {
	if depth == 0 {
		p.gobble("with")
		begin = p.lastLocationStart()
		if p.cSyntax {
			p.gobble("(")
		}
	}
	result := &With{}
	result.col, result.line = p.lastTokCol, p.lastTokLine
	result.name = p.getAlgToken()
	result.isEq = p.gobbleEqualOrIn()
	result.exp = p.getExpr()
	if p.pSyntax || p.peekAt(1) != ")" {
		p.gobbleCommaOrSemicolon()
	}
	if len(result.exp.tokens) == 0 {
		p.parsingError("Empty with expression at ")
	}
	result.do = p.getWithBody(depth, begin)
	if result.do.len() == 0 {
		failAt("Missing body of with statement", result)
	}
	result.origin = &region{begin, result.do.last().base().origin.endLoc()}
	return result
}

func (p *parser) getWithBody(depth int, begin location) *seq {
	if p.pSyntax && p.peekAt(1) == "do" {
		p.gobble("do")
		body := p.getStmtSeq()
		p.gobble("end")
		p.gobble("with")
		p.gobble(";")
		return body
	}
	if p.cSyntax && p.peekAt(1) == ")" {
		p.mustGobble(")")
		return p.getCStmt()
	}
	return newSeq(p.getWith(depth+1, begin))
}

func (p *parser) getAssign() node {
	result := &Assign{}
	result.col, result.line = p.curTokCol[0]+1, p.curTokLine[0]+1
	result.ass = []*SingleAssign{p.getSingleAssign()}
	for p.peekAt(1) == "||" {
		p.getAlgToken()
		var sa *SingleAssign
		if err := catch(func() { sa = p.getSingleAssign() }, nil); err != nil {
			p.parsingError("Bad assignment statement at ")
		}
		result.ass = append(result.ass, sa)
	}
	p.gobble(";")
	first, last := result.ass[0], result.ass[len(result.ass)-1]
	result.origin = &region{first.origin.beginLoc(), last.origin.endLoc()}
	return result
}

func (p *parser) getSingleAssign() *SingleAssign {
	result := newSingleAssign()
	result.col, result.line = p.curTokCol[0]+1, p.curTokLine[0]+1
	result.lhs = p.getLhs()
	p.gobble(":=")
	result.rhs = p.getExpr()
	if len(result.rhs.tokens) == 0 {
		p.parsingError("Empty right-hand side of assignment at ")
	}
	result.origin = &region{result.lhs.origin.beginLoc(), result.rhs.origin.endLoc()}
	return result
}

func (p *parser) getLhs() *Lhs {
	result := &Lhs{}
	result.col, result.line = p.curTokCol[0]+1, p.curTokLine[0]+1
	result.name = p.getAlgToken()
	begin, end := p.lastLocationStart(), p.lastLocationEnd()
	result.sub = p.getExpr()
	if result.sub.origin != nil {
		end = result.sub.origin.endLoc()
	}
	result.origin = &region{begin, end}
	return result
}

func (p *parser) getPrint() node {
	result := &PrintS{}
	begin := p.startStmt("print", &result.nodeBase)
	result.exp = p.getExpr()
	if len(result.exp.tokens) == 0 {
		p.parsingError("Empty expression in print statement at ")
	}
	result.origin = &region{begin, result.exp.origin.endLoc()}
	p.gobble(";")
	return result
}

func (p *parser) getAssert() node {
	result := &Assert{}
	p.startStmt("assert", &result.nodeBase)
	result.exp = p.getExpr()
	if len(result.exp.tokens) == 0 {
		p.parsingError("Empty expression in assert statement at ")
	}
	p.gobble(";")
	result.origin = &region{location{result.line - 1, result.col - 1}, result.exp.origin.endLoc()}
	return result
}

func (p *parser) getSkip() node {
	result := &Skip{}
	begin := p.startStmt("skip", &result.nodeBase)
	result.origin = &region{begin, location{begin.line, begin.column + 4}}
	p.gobble(";")
	return result
}

func (p *parser) getWhen(kw string) node {
	result := &When{}
	p.startStmt(kw, &result.nodeBase)
	result.exp = p.getExpr()
	result.origin = &region{location{result.line - 1, result.col - 1}, result.exp.origin.endLoc()}
	if len(result.exp.tokens) == 0 {
		p.parsingError("Empty expression in when statement at ")
	}
	p.gobble(";")
	return result
}

// getArgs reads a parenthesized argument list after its name.
func (p *parser) getArgs(gobbleOpen func(string), kind string) []*expr {
	gobbleOpen("(")
	var args []*expr
	more := p.peekPast(0)[0] != ')'
	for more {
		arg := p.getExpr()
		args = append(args, arg)
		if len(arg.tokens) == 0 {
			p.parsingError("Empty argument in " + kind + " statement at ")
		}
		if p.peekAt(1) != ")" {
			p.gobble(",")
		} else {
			more = false
		}
	}
	p.gobble(")")
	return args
}

func (p *parser) getCall() *Call {
	result := &Call{}
	p.startStmt("call", &result.nodeBase)
	result.to = p.getAlgToken()
	result.args = p.getArgs(p.gobble, "call")
	result.origin = &region{location{result.line - 1, result.col - 1}, location{p.lastTokLine - 1, p.lastTokCol}}
	p.gobble(";")
	if !p.proceduresCalled.contains(result.to) {
		p.proceduresCalled.add(result.to)
	}
	return result
}

func (p *parser) getReturn() node {
	result := &Return{}
	begin := p.startStmt("return", &result.nodeBase)
	result.from = p.currentProcedure
	result.origin = &region{begin, location{begin.line, begin.column + 6}}
	p.gobble(";")
	return result
}

func (p *parser) getCallOrCallReturnOrCallGoto() node {
	call := p.getCall()
	switch p.peekAt(1) {
	case "return":
		p.mustGobble("return")
		end := location{p.lastTokLine - 1, p.lastTokCol + 5}
		p.gobble(";")
		result := &CallReturn{to: call.to, from: p.currentProcedure, args: call.args}
		result.col, result.line = call.col, call.line
		result.origin = &region{call.origin.beginLoc(), end}
		return result
	case "goto":
		p.mustGobble("goto")
		result := &CallGoto{to: call.to, args: call.args}
		result.col, result.line = call.col, call.line
		result.after = p.getAlgToken()
		end := location{p.lastTokLine - 1, p.lastTokCol - 1 + jlen(result.to)}
		result.origin = &region{call.origin.beginLoc(), end}
		p.noteGoto(result.to)
		p.gobble(";")
		return result
	}
	return call
}

func (p *parser) noteGoto(to string) {
	p.gotoUsed = true
	if to == "Done" || to == "\"Done\"" {
		p.gotoDoneUsed = true
	}
}

func (p *parser) getGoto() node {
	result := &Goto{}
	p.startStmt("goto", &result.nodeBase)
	result.to = p.getAlgToken()
	result.origin = &region{location{result.line - 1, result.col - 1}, location{p.lastTokLine - 1, p.lastTokCol - 1 + jlen(result.to)}}
	p.noteGoto(result.to)
	p.gobble(";")
	return result
}

func (p *parser) getMacro(macros []*Macro) *Macro {
	result := &Macro{}
	p.inGetMacro = true
	begin := p.startStmt("macro", &result.nodeBase)
	result.name = p.getAlgToken()
	p.gobble("(")
	lookForComma := false
	for p.peekAt(1) != ")" {
		if lookForComma {
			p.gobble(",")
		}
		result.params = append(result.params, p.getAlgToken())
		lookForComma = true
	}
	p.mustGobble(")")
	p.gobbleBegin()
	result.body = p.getStmtSeq()
	p.gobbleEnd("macro")
	result.origin = &region{begin, p.lastLocationEnd()}
	p.skipSemicolon()
	expandMacrosInStmtSeq(result.body, macros)
	p.inGetMacro = false
	return result
}

func (p *parser) getMacroCall() node {
	result := &MacroCall{}
	result.name = p.getAlgToken()
	begin := p.lastLocationStart()
	result.col, result.line = p.lastTokCol, p.lastTokLine
	result.args = p.getArgs(p.mustGobble, "call")
	result.origin = &region{begin, p.lastLocationEnd()}
	p.gobble(";")
	return result
}
