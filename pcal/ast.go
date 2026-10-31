package pcal

// node is a PlusCal AST node, as in Java's AST class hierarchy.
type node interface {
	base() *nodeBase
	print(p *astPrinter)
}

// nodeBase holds the fields common to every AST node. A zero macroLine
// stands for Java's -1: Java only stores positive macro lines.
type nodeBase struct {
	pos
	lbl         string
	lblLocation *location
	origin      *region
	macroBegin  *location
	macroEnd    *location
}

func (b *nodeBase) base() *nodeBase { return b }

// copyPos copies the source and macro positions of another node.
func (b *nodeBase) copyPos(o *nodeBase) {
	b.pos = o.pos
	b.origin = o.origin
}

// jstring is a Java String reference that may be null.
type jstring struct {
	val string
	ok  bool
}

func some(s string) jstring { return jstring{s, true} }

func (j jstring) String() string {
	if !j.ok {
		return "null"
	}
	return j.val
}

// seq is a mutable statement list. Java shares and mutates statement
// vectors between the parsed and the exploded AST, and the checksum of the
// parsed AST observes those mutations, so lists are shared by reference.
type seq struct{ items []node }

func newSeq(items ...node) *seq { return &seq{items: items} }

func (s *seq) len() int { return len(s.items) }

func (s *seq) at(i int) node { return s.items[i] }

func (s *seq) last() node { return s.items[len(s.items)-1] }

func (s *seq) add(n ...node) { s.items = append(s.items, n...) }

func (s *seq) clone() *seq { return &seq{items: append([]node(nil), s.items...)} }

func (s *seq) removeLast() { s.items = s.items[:len(s.items)-1] }

// Process fairness, as in AST.
const (
	unfairProc = 0
	wfProc     = 1
	sfProc     = 2
)

var fairnessString = []string{"unfair", "wf", "sf"}

// If sources record how an If was produced from a broken LabelIf.
const (
	brokenThen = 2
	brokenElse = 4
)

type (
	// Uniprocess is a single-process algorithm.
	Uniprocess struct {
		nodeBase
		name   string
		decls  []*VarDecl
		defs   *expr
		macros []*Macro
		prcds  []*Procedure
		body   *seq
	}

	// Multiprocess is an algorithm with process declarations.
	Multiprocess struct {
		nodeBase
		name   string
		decls  []*VarDecl
		defs   *expr
		macros []*Macro
		prcds  []*Procedure
		procs  []*Process
	}

	// Procedure is a procedure declaration.
	Procedure struct {
		nodeBase
		name             string
		minusLabels      *strList
		plusLabels       *strList
		proceduresCalled *strList
		params           []*PVarDecl
		decls            []*PVarDecl
		body             *seq
	}

	// Process is a process declaration.
	Process struct {
		nodeBase
		name             string
		fairness         int
		minusLabels      *strList
		plusLabels       *strList
		proceduresCalled *strList
		isEq             bool
		id               *expr
		decls            []*VarDecl
		body             *seq
	}

	// VarDecl declares a global or process variable.
	VarDecl struct {
		nodeBase
		name string
		isEq bool
		val  *expr
	}

	// PVarDecl declares a procedure parameter or variable.
	PVarDecl struct {
		nodeBase
		name string
		val  *expr
	}

	// LabeledStmt is a label and the statements it labels.
	LabeledStmt struct {
		nodeBase
		label string
		stmts *seq
	}

	// While is a while loop.
	While struct {
		nodeBase
		test    *expr
		unlabDo *seq
		labDo   *seq
	}

	// Assign is a multiple assignment.
	Assign struct {
		nodeBase
		ass []*SingleAssign
	}

	// SingleAssign is one assignment of a multiple assignment.
	SingleAssign struct {
		nodeBase
		lhs *Lhs
		rhs *expr
	}

	// Lhs is an assignment target.
	Lhs struct {
		nodeBase
		name string
		sub  *expr
	}

	// If is a conditional without labels.
	If struct {
		nodeBase
		test   *expr
		then   *seq
		els    *seq
		source int
	}

	// Either is a nondeterministic choice without labels.
	Either struct {
		nodeBase
		ors []*seq
	}

	// With is a with statement.
	With struct {
		nodeBase
		name string
		isEq bool
		exp  *expr
		do   *seq
	}

	// When is an await statement.
	When struct {
		nodeBase
		exp *expr
	}

	// PrintS is a print statement.
	PrintS struct {
		nodeBase
		exp *expr
	}

	// Assert is an assert statement.
	Assert struct {
		nodeBase
		exp *expr
	}

	// Skip is a skip statement.
	Skip struct{ nodeBase }

	// LabelIf is a conditional that may contain labels.
	LabelIf struct {
		nodeBase
		test      *expr
		unlabThen *seq
		labThen   *seq
		unlabElse *seq
		labElse   *seq
	}

	// LabelEither is an either statement that may contain labels.
	LabelEither struct {
		nodeBase
		clauses []*Clause
	}

	// Clause is one clause of a LabelEither.
	Clause struct {
		nodeBase
		unlabOr *seq
		labOr   *seq
		broken  bool
	}

	// Call is a procedure call.
	Call struct {
		nodeBase
		returnTo string
		to       string
		args     []*expr
	}

	// Return is a return statement.
	Return struct {
		nodeBase
		from jstring
	}

	// CallReturn is a call immediately followed by a return.
	CallReturn struct {
		nodeBase
		from jstring
		to   string
		args []*expr
	}

	// CallGoto is a call immediately followed by a goto.
	CallGoto struct {
		nodeBase
		after string
		to    string
		args  []*expr
	}

	// Goto is a goto statement.
	Goto struct {
		nodeBase
		to string
	}

	// Macro is a macro definition.
	Macro struct {
		nodeBase
		name   string
		params []string
		body   *seq
	}

	// MacroCall is a macro call.
	MacroCall struct {
		nodeBase
		name string
		args []*expr
	}
)

// defaultVarInit returns the initial value of a declaration without one.
func defaultVarInit() *expr { return exprOf(newToken("defaultInitValue", 0, tokIdent)) }

func newVarDecl() *VarDecl { return &VarDecl{isEq: true, val: defaultVarInit()} }

func newPVarDecl() *PVarDecl { return &PVarDecl{val: defaultVarInit()} }

func newSingleAssign() *SingleAssign { return &SingleAssign{lhs: &Lhs{}} }

func (d *PVarDecl) toVarDecl() *VarDecl {
	v := &VarDecl{name: d.name, val: d.val, isEq: true}
	v.origin = d.origin
	return v
}
