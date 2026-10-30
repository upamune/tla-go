// Package parser implements the TLA+ syntactic front end.
// Its grammar is derived from tla-ts; see UPSTREAM-LICENSE.
package parser

import "github.com/upamune/tla-go/syntax"

// Position and Range use one-based lines and columns, matching the lexer.
type (
	Position struct{ Line, Column int }
	Range    struct {
		Module     string
		Start, End Position
	}
)

type Location struct {
	Position
	Range Range
}

// Expression is a TLA+ expression. Kind identifies which fields are present.
type Expression struct {
	Kind, Text     string
	Bulleted       bool
	Quantifier     string
	Temporal       bool
	Names          []string
	ArgumentCounts []int
	Selectors      []Selector
	Bounds         []Bound
	Fields         []Field
	Updates        []Update
	Arms           []CaseArm
	Definitions    []Definition
	Instances      []Instance
	Location       Location
	Args           []*Expression
	// Instance metadata preserves substitution boundaries for ENABLED.
	EnabledScopes   []string         `json:"-"`
	EnabledBindings []EnabledBinding `json:"-"`
}

// EnabledBinding identifies a variable before an instance substitution.
type EnabledBinding struct{ Scope, Name string }

// Selector is a component of a qualified operator or subexpression reference.
type Selector struct {
	Kind, Name, Operand string
	Args                []*Expression
}

type CaseArm struct{ Condition, Body *Expression }

// PathStep identifies a field or function index in an EXCEPT update.
type PathStep struct {
	Kind, Name string
	Args       []*Expression
}
type Update struct {
	Path  []PathStep
	Value *Expression
}

// Field is an entry in a record or record set.
type Field struct {
	Name  string
	Value *Expression
}

// Bound associates a bound name or tuple pattern with its domain.
type Bound struct {
	Names []string
	Tuple bool
	Set   *Expression
}
type Declaration struct {
	Name     string
	Arity    int
	Location Location
}
type Definition struct {
	Kind      string
	Recursive bool
	Bounds    []Bound
	Name      string
	// SelectorName preserves the source name of an alpha-renamed LET operator.
	SelectorName string `json:"-"`
	Params       []Declaration
	Body         *Expression
	Local        bool
	Location     Location
	BodyRange    Range
	// Origins records definition copies created while linking module instances.
	// Parsed syntax leaves it empty; it is not part of the serialized AST.
	Origins []DefinitionOrigin `json:"-"`
	// ConfigConstant represents a lazily applied constant-valued operator.
	ConfigConstant bool `json:"-"`
	// ConfigStandard identifies a standard value exposed for configuration overrides.
	ConfigStandard bool `json:"-"`
	// InstanceRange and InstanceParamCount preserve the source instance for TLC descriptions.
	InstanceRange      Range `json:"-"`
	InstanceParamCount int   `json:"-"`
}

// DefinitionOrigin identifies a definition before instance parameters were added.
type DefinitionOrigin struct {
	Module, Name string
	Prefix       int
}
type Substitution struct {
	Name       string
	Expression *Expression
}
type Instance struct {
	Name                           string
	Params                         []Declaration
	Module                         string
	Substitutions                  []Substitution
	Local                          bool
	Location                       Location
	InstanceRange, DefinitionRange Range
}
type Proof struct {
	Only        bool
	Facts, Defs []*Expression
	Kind        string
	Keyword     Range
	Location    Location
}
type Theorem struct {
	Name      string
	Statement *Expression
	Proof     *Proof
	Location  Location
}
type Assumption struct {
	Name       string
	Expression *Expression
	Axiom      bool
	Location   Location
}
type Module struct {
	// SourceTokens retain parse-time token order for TLC string interning.
	SourceTokens          []syntax.Token `json:"-"`
	Name                  string
	Extends               []string
	Constants, Variables  []Declaration
	Definitions           []Definition
	Instances             []Instance
	Modules               []Module
	Assumptions           []Assumption
	Theorems              []Theorem
	RecursiveDeclarations []Declaration
	Location              Location
}
