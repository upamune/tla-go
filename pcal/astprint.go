package pcal

import (
	"strconv"
	"strings"
)

// astPrinter reproduces AST.toString, including the indentation stack that
// Java keeps in static fields.
type astPrinter struct {
	sb       strings.Builder
	indent   []int
	debug    bool
	before15 bool
}

func astString(n node, debug, before15 bool) string {
	p := &astPrinter{indent: []int{0}, debug: debug, before15: before15}
	n.print(p)
	return p.sb.String()
}

func (p *astPrinter) w(parts ...string) {
	for _, s := range parts {
		p.sb.WriteString(s)
	}
}

// push writes s and opens an indentation level after it (Indent).
func (p *astPrinter) push(s string) {
	p.indent = append(p.indent, p.indent[len(p.indent)-1]+jlen(s))
	p.w(s)
}

// pop closes an indentation level (EndIndent).
func (p *astPrinter) pop() { p.indent = p.indent[:len(p.indent)-1] }

// nl starts a new line at the current indentation (NewLine).
func (p *astPrinter) nl() { p.w("\n", spaces(p.indent[len(p.indent)-1])) }

func (p *astPrinter) lineCol(b *nodeBase) string {
	if !p.debug {
		return ""
	}
	return "(*" + strconv.Itoa(b.line) + ", " + strconv.Itoa(b.col) + "*)"
}

// open writes the debugging position and opens the node's indentation.
func (p *astPrinter) open(b *nodeBase) { p.push(p.lineCol(b)) }

// field writes an indented field whose value is printed by f.
func (p *astPrinter) field(name string, f func(), end string) {
	p.push(name)
	f()
	p.w(end)
	p.pop()
}

type printable interface{ print(p *astPrinter) }

func (e *expr) print(p *astPrinter) {
	if e == nil {
		p.w("null")
		return
	}
	p.w(e.String())
}

func printAll[T printable](p *astPrinter, xs []T) {
	p.push("<<")
	for i, x := range xs {
		if i > 0 {
			p.w(", ")
			p.nl()
		}
		x.print(p)
	}
	p.w(">>")
	p.pop()
}

func (p *astPrinter) list(s *seq) {
	if s == nil {
		p.w("null")
		return
	}
	printAll(p, s.items)
}

func (p *astPrinter) quotedList(l *strList) {
	if l == nil {
		p.w("null")
		return
	}
	p.push("<<")
	for i, x := range l.items {
		if i > 0 {
			p.w(", ")
		}
		p.w(`"`, x, `"`)
	}
	p.w(">>")
	p.pop()
}

func (p *astPrinter) lists(ors []*seq) {
	p.push("<< ")
	for i, s := range ors {
		if i > 0 {
			p.w(", ")
			p.nl()
		}
		p.list(s)
	}
	p.w(" >>")
	p.pop()
}

func eqOrIn(isEq bool) string {
	if isEq {
		return `"="`
	}
	return `"\\in"`
}

func (n *Uniprocess) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[type     |-> "uniprocess", `)
	p.nl()
	p.w(` name  |-> "`, n.name, `", `)
	p.nl()
	p.field(" decls  |-> ", func() { printAll(p, n.decls) }, ",")
	p.nl()
	p.field(" defs   |-> ", func() { n.defs.print(p) }, ",")
	p.nl()
	p.field(" prcds  |-> ", func() { printAll(p, n.prcds) }, ",")
	p.nl()
	p.field(" body  |-> ", func() { p.list(n.body) }, "]")
	p.pop()
}

func (n *Multiprocess) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[type    |-> "multiprocess", `)
	p.nl()
	p.w(` name  |-> "`, n.name, `", `)
	p.nl()
	p.field(" decls |-> ", func() { printAll(p, n.decls) }, ",")
	p.nl()
	p.field(" defs  |-> ", func() { n.defs.print(p) }, ",")
	p.nl()
	p.field(" prcds |-> ", func() { printAll(p, n.prcds) }, ",")
	p.nl()
	p.field(" procs |-> ", func() { printAll(p, n.procs) }, "]")
	p.pop()
}

func (p *astPrinter) labelSets(minus, plus, called *strList) {
	p.w("minusLabels |-> ")
	p.quotedList(minus)
	p.w(", plusLabels |->")
	p.quotedList(plus)
	p.w(", proceduresCalled |->")
	p.quotedList(called)
	p.w(",")
}

func (n *Procedure) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[name   |-> "`, n.name, `", `)
	p.nl()
	if !p.before15 {
		p.labelSets(n.minusLabels, n.plusLabels, n.proceduresCalled)
		p.nl()
	}
	p.field(" params |-> ", func() { printAll(p, n.params) }, ",")
	p.nl()
	p.field(" decls  |-> ", func() { printAll(p, n.decls) }, ",")
	p.nl()
	p.field(" body   |-> ", func() { p.list(n.body) }, "]")
	p.pop()
}

func (n *Process) print(p *astPrinter) {
	p.open(&n.nodeBase)
	if p.before15 {
		p.w(`[name   |-> "`, n.name, `", `)
	} else {
		p.w(`[name   |-> "`, n.name, `",`)
		p.nl()
		p.w(` fairness |-> "`, fairnessString[n.fairness], `", `)
		p.labelSets(n.minusLabels, n.plusLabels, n.proceduresCalled)
	}
	p.nl()
	p.w(" eqOrIn |-> ", eqOrIn(n.isEq), ",")
	p.nl()
	p.w(" id     |-> ", n.id.String(), ",")
	p.nl()
	p.field(" decls  |-> ", func() { printAll(p, n.decls) }, ",")
	p.nl()
	p.field(" body   |-> ", func() { p.list(n.body) }, "]")
	p.pop()
}

func (n *VarDecl) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[var |-> "`, n.name, `", eqOrIn |-> `, eqOrIn(n.isEq), ", val |-> ", n.val.String(), "]")
	p.pop()
}

func (n *PVarDecl) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[var |-> "`, n.name, `", eqOrIn |-> "=", val |-> `, n.val.String(), "]")
	p.pop()
}

func (n *LabeledStmt) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[label |-> "`, n.label, `",`)
	p.nl()
	p.field(" stmts |-> ", func() { p.list(n.stmts) }, "]")
	p.pop()
}

func (n *While) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[type    |-> "while", `)
	p.nl()
	p.w(" test    |-> ", n.test.String(), ",")
	p.nl()
	p.field(" labDo   |-> ", func() { p.list(n.labDo) }, ",")
	p.nl()
	p.field(" unlabDo |-> ", func() { p.list(n.unlabDo) }, "]")
	p.pop()
}

func (n *Assign) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[type |-> "assignment",`)
	p.nl()
	p.field(" ass  |-> ", func() { printAll(p, n.ass) }, "]")
	p.pop()
}

func (n *SingleAssign) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w("[lhs |-> ")
	n.lhs.print(p)
	p.w(",")
	p.nl()
	p.w(" rhs |-> ")
	n.rhs.print(p)
	p.w("]")
	p.pop()
}

func (n *Lhs) print(p *astPrinter) {
	p.w(p.lineCol(&n.nodeBase), `[var |-> "`, n.name, `", sub |-> `)
	n.sub.print(p)
	p.w("]")
}

func (n *If) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[type    |-> "if", `)
	p.nl()
	p.w(" test    |-> ", n.test.String(), ",")
	p.nl()
	p.field(" then |-> ", func() { p.list(n.then) }, ",")
	p.nl()
	p.field(" else |-> ", func() { p.list(n.els) }, "]")
	p.pop()
}

func (n *Either) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[type |-> "either", `)
	p.nl()
	p.field(" ors  |-> ", func() { p.lists(n.ors) }, "]")
	p.pop()
}

func (n *With) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[type   |-> "with", `)
	p.nl()
	p.w(` var    |-> "`, n.name, `",`)
	p.nl()
	p.w(" eqOrIn |-> ", eqOrIn(n.isEq), ",")
	p.nl()
	p.w(" exp    |-> ", n.exp.String(), ",")
	p.nl()
	p.field(" do     |-> ", func() { p.list(n.do) }, "]")
	p.pop()
}

func (p *astPrinter) exprStmt(b *nodeBase, kind string, e *expr) {
	p.open(b)
	p.w(`[type |-> "`, kind, `", `)
	p.nl()
	p.w(" exp |-> ", e.String(), "]")
	p.pop()
}

func (n *When) print(p *astPrinter) { p.exprStmt(&n.nodeBase, "when", n.exp) }

func (n *PrintS) print(p *astPrinter) { p.exprStmt(&n.nodeBase, "print", n.exp) }

func (n *Assert) print(p *astPrinter) { p.exprStmt(&n.nodeBase, "assert", n.exp) }

func (n *Skip) print(p *astPrinter) { p.w(p.lineCol(&n.nodeBase), `[type |-> "skip"]`) }

func (n *LabelIf) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[type      |-> "labelIf",`)
	p.nl()
	p.w(" test      |-> ", n.test.String(), ",")
	p.nl()
	p.field(" unlabThen |-> ", func() { p.list(n.unlabThen) }, ",")
	p.nl()
	p.field(" labThen   |-> ", func() { p.list(n.labThen) }, ",")
	p.nl()
	p.field(" unlabElse |-> ", func() { p.list(n.unlabElse) }, ",")
	p.nl()
	p.field(" labElse   |-> ", func() { p.list(n.labElse) }, "]")
	p.nl()
	p.pop()
}

func (n *LabelEither) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[type    |-> "labelEither",`)
	p.nl()
	p.field(" clauses |-> ", func() { printAll(p, n.clauses) }, "]")
	p.nl()
	p.pop()
}

func (n *Clause) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.field("[unlabOr |-> ", func() { p.list(n.unlabOr) }, ",")
	p.nl()
	p.field(" labOr   |-> ", func() { p.list(n.labOr) }, "]")
	p.nl()
	p.pop()
}

func (n *Call) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[type     |-> "call",`)
	p.nl()
	p.w(` returnTo |-> "`, n.returnTo, `",`)
	p.nl()
	p.w(` to       |-> "`, n.to, `",`)
	p.nl()
	p.field(" args     |-> ", func() { printAll(p, n.args) }, "]")
	p.pop()
}

func (n *Return) print(p *astPrinter) {
	p.w(p.lineCol(&n.nodeBase), `[type |-> "return", from |-> "`, n.from.String(), `"]`)
}

// callTail prints the fields shared by CallReturn and CallGoto.
func (p *astPrinter) callTail(b *nodeBase, kind, key, val, to string, args []*expr) {
	p.open(b)
	p.w(`[type     |-> "`, kind, `",`)
	p.nl()
	p.w(key, val, `",`)
	p.nl()
	p.w(` to       |-> "`, to, `",`)
	p.nl()
	p.push("args     |-> ")
	printAll(p, args)
	p.w("]")
	p.nl()
	p.pop()
	p.pop()
}

func (n *CallReturn) print(p *astPrinter) {
	p.callTail(&n.nodeBase, "callReturn", ` from     |-> "`, n.from.String(), n.to, n.args)
}

func (n *CallGoto) print(p *astPrinter) {
	p.callTail(&n.nodeBase, "callGoto", ` after    |-> "`, n.after, n.to, n.args)
}

func (n *Goto) print(p *astPrinter) {
	p.w(p.lineCol(&n.nodeBase), `[type |-> "goto",  to |-> "`, n.to, `"]`)
}

func (n *Macro) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[name   |-> "`, n.name, `", `)
	p.nl()
	p.push(" params |-> ")
	p.push("<<")
	for i, s := range n.params {
		if i > 0 {
			p.w(", ")
			p.nl()
		}
		p.w(s)
	}
	p.w(">>")
	p.pop()
	p.w(",")
	p.pop()
	p.nl()
	p.field(" body   |-> ", func() { p.list(n.body) }, "]")
	p.pop()
}

func (n *MacroCall) print(p *astPrinter) {
	p.open(&n.nodeBase)
	p.w(`[type |-> "macrocall",`)
	p.nl()
	p.w(` name |-> "`, n.name, `",`)
	p.nl()
	p.field(" args     |-> ", func() { printAll(p, n.args) }, "]")
	p.pop()
}
