// Package evaluator evaluates TLA+ expressions against an explicit state.
package evaluator

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

type State map[string]value.Value

// Context contains state and constant bindings. Next is required for primed expressions.
type Context struct {
	Current, Next State
	Constants     map[string]value.Value
	Locals        map[string]value.Value
	Lexical       *LexicalScope
	Enabled       EnabledCallback
	OnPrint       func(string)
	// UncachedArguments matches TLC's evaluation of ENABLED predicates.
	UncachedArguments bool
	// UncachedConstants evaluates configuration overrides before constant precomputation.
	UncachedConstants bool
}
type scope struct {
	parent      *scope
	values      map[string]value.Value
	definitions map[string]parser.Definition
	operators   map[string]operatorRef
	primeArgs   map[string]*primeArgument
}

type primeArgument struct {
	expression    *parser.Expression
	scope         *scope
	enabledScopes []string
	cacheMu       sync.Mutex
	cached        [2]bool
	values        [2]value.Value
}
type evaluator struct {
	context        Context
	program        *Program
	primed         *tlcPrime
	scope          *scope
	variables      map[string]bool
	finiteSets     bool
	sequences      bool
	naturals       bool
	integers       bool
	tlc            bool
	tlcext         bool
	bags           bool
	json           bool
	randomization  bool
	hasOperators   bool
	primeArguments bool
	keepLazy       bool
	active         map[string][]activeCall
	functionCache  map[functionCacheKey]value.Value
	depth          int
}

// Deferred values capture snapshots, so an evaluator can be reused after eval.
var evaluatorPool = sync.Pool{New: func() any { return new(evaluator) }}

// Program holds the immutable definition index for repeated evaluations.
type Program struct {
	root            scope
	variables       map[string]bool
	finiteSets      bool
	sequences       bool
	naturals        bool
	integers        bool
	tlc             bool
	tlcext          bool
	bags            bool
	json            bool
	randomization   bool
	onPrint         func(string)
	printStyle      PrintStyle
	onWarning       func(code int, message string)
	tlcRuntime      TLCRuntime
	ordering        ValueOrdering
	readFile        func(string) (string, error)
	writeFile       func(string, string) error
	randomMu        sync.Mutex
	defaultRandom   *JavaRandom
	constantDefs    map[string]bool
	constantCache   map[string]value.Value
	constantTLCEval map[*parser.Expression]bool
	tlcEvalCache    map[*parser.Expression]value.Value
}

// New prepares the definitions in a module for repeated evaluation.
func New(module *parser.Module) *Program {
	defs := make(map[string]parser.Definition, len(module.Definitions))
	for _, d := range module.Definitions {
		defs[d.Name] = d
	}
	variables := make(map[string]bool, len(module.Variables))
	for _, v := range module.Variables {
		variables[v.Name] = true
	}
	imports := module.Extends
	if len(module.InstanceStandardModules) != 0 {
		imports = append(append([]string(nil), imports...), module.InstanceStandardModules...)
	}
	return &Program{root: scope{definitions: defs}, variables: variables, finiteSets: slices.Contains(imports, "FiniteSets"), sequences: slices.Contains(imports, "Sequences"), naturals: hasNaturals(imports), integers: slices.Contains(imports, "Integers"), tlc: slices.Contains(imports, "TLC"), tlcext: slices.Contains(imports, "TLCExt"), bags: slices.Contains(imports, "Bags"), json: slices.Contains(imports, "Json"), randomization: slices.Contains(imports, "Randomization")}
}

// NewWithPrint configures the callback used by TLC Print and PrintT.
func NewWithPrint(module *parser.Module, onPrint func(string)) *Program {
	program := New(module)
	program.onPrint = onPrint
	return program
}

// NewWithRuntime configures TLC output and per-check runtime state.
func NewWithRuntime(module *parser.Module, onPrint func(string), runtime TLCRuntime) *Program {
	program := NewWithPrint(module, onPrint)
	program.tlcRuntime = runtime
	return program
}

func hasNaturals(extends []string) bool {
	return slices.Contains(extends, "Naturals") || slices.Contains(extends, "Integers") || slices.Contains(extends, "Sequences")
}

// ValueOrdering controls TLC's per-check enumeration order.
type ValueOrdering interface {
	Compare(value.Value, value.Value) (int, error)
	Intern(string)
}

// SetOrdering configures a check-local ordering before evaluation begins.
func (p *Program) SetOrdering(ordering ValueOrdering) { p.ordering = ordering }

// SetJSONFileSystem supplies the file callbacks used by the Json module.
func (p *Program) SetJSONFileSystem(read func(string) (string, error), write func(string, string) error) {
	p.readFile, p.writeFile = read, write
}

// PrintStyle selects how Print and PrintT render values.
type PrintStyle struct {
	// Terse renders SUBSET, function-set, record-set, and Cartesian-product
	// values as formulas, as Java TLC does with -terse.
	Terse bool
	// UnformattedPrintT skips the 80-column layout of PrintT values, as Java
	// TLC does when -userFile redirects Print output to a file.
	UnformattedPrintT bool
}

// SetPrintStyle configures Print and PrintT before evaluation begins.
func (p *Program) SetPrintStyle(style PrintStyle) { p.printStyle = style }

// SetWarningHandler receives Java TLC warnings raised during evaluation,
// such as an EXCEPT applied to a key outside a function's domain (2144).
func (p *Program) SetWarningHandler(handler func(code int, message string)) {
	p.onWarning = handler
}

// SetRandomSeed resets the generator used when no checker runtime is attached.
func (p *Program) SetRandomSeed(seed int64) {
	p.randomMu.Lock()
	defer p.randomMu.Unlock()
	p.defaultRandom = NewJavaRandom(seed)
}

// SetConstantDefinitions enables check-local caching for constant definitions.
func (p *Program) SetConstantDefinitions(names map[string]bool) {
	p.constantDefs = names
	p.constantCache = make(map[string]value.Value, len(names))
}

// SetConstantTLCEvaluations identifies closed constant-level TLCEval calls.
func (p *Program) SetConstantTLCEvaluations(expressions map[*parser.Expression]bool) {
	p.constantTLCEval = expressions
	p.tlcEvalCache = make(map[*parser.Expression]value.Value, len(expressions))
}

// Evaluate evaluates one parsed expression. Unsupported forms return an error.
func Evaluate(module *parser.Module, expression *parser.Expression, context Context) (value.Value, error) {
	return New(module).Evaluate(expression, context)
}

// Evaluate evaluates an expression with this program's definitions.
func (p *Program) Evaluate(expression *parser.Expression, context Context) (value.Value, error) {
	if p.tlcRuntime == nil && (p.tlc || p.randomization) {
		p.randomMu.Lock()
		defer p.randomMu.Unlock()
		if p.defaultRandom == nil {
			p.defaultRandom = NewJavaRandom(0)
		}
	}
	return p.evaluate(expression, context)
}

func (p *Program) evaluate(expression *parser.Expression, context Context) (value.Value, error) {
	if context.OnPrint == nil {
		context.OnPrint = p.onPrint
	}
	e := evaluatorPool.Get().(*evaluator)
	*e = evaluator{context: context, program: p, scope: &p.root, variables: p.variables, finiteSets: p.finiteSets, sequences: p.sequences, naturals: p.naturals, integers: p.integers, tlc: p.tlc, tlcext: p.tlcext, bags: p.bags, json: p.json, randomization: p.randomization}
	defer func() {
		*e = evaluator{}
		evaluatorPool.Put(e)
	}()
	if context.Lexical != nil {
		e.scope = context.Lexical.scope
		e.hasOperators = context.Lexical.hasOperators
	}
	if len(context.Locals) > 0 {
		e.scope = &scope{parent: e.scope, values: context.Locals}
	}
	result, err := e.eval(expression)
	if err == nil {
		return result, nil
	}
	if failure, ok := ValueRuntimeError(err); ok {
		return value.Value{}, failure
	}
	if failure, ok := errors.AsType[*value.TLCError](err); ok {
		return value.Value{}, &RuntimeError{Message: failure.Error(), Standard: failure.Standard, Sources: failure.Sources}
	}
	return result, err
}

func (e *evaluator) eval(expr *parser.Expression) (value.Value, error) {
	result, err := e.evalExpression(expr)
	if err != nil && expr != nil {
		AddUnexpectedSource(err, expr)
	}
	if err == nil && result.Kind() == value.StringKind && e.program.ordering != nil {
		text, _ := result.Text()
		e.program.ordering.Intern(text)
	}
	return result, err
}

func (e *evaluator) evalExpression(expr *parser.Expression) (value.Value, error) {
	if expr == nil {
		return value.Value{}, fmt.Errorf("missing expression")
	}
	switch expr.Kind {
	case "Number":
		return numericLiteral(expr.Text)
	case "String":
		return value.String(expr.Text), nil
	case "Identifier":
		return e.expressionIdentifier(expr)
	case "Apply":
		return e.apply(expr)
	case "Always", "Eventually", "WeakFairness", "StrongFairness":
		return temporalEvaluationError(expr)
	case "And", "Or":
		return e.junction(expr)
	}
	return e.evalCompound(expr)
}

func (e *evaluator) expressionIdentifier(expr *parser.Expression) (value.Value, error) {
	if expr.BuiltinNatural && expr.Text == "!naturals!Nat" {
		return value.NaturalNumbers(), nil
	}
	if expr.BuiltinNatural && expr.Text == "!naturals!Int" {
		return value.Integers(), nil
	}
	if expr.BuiltinTLC {
		switch expr.Text {
		case "!tlc!Any":
			return value.Any(), nil
		case "!tlc!JavaTime":
			return javaTime()
		}
	}
	return e.standardIdentifier(expr)
}

func (e *evaluator) standardIdentifier(expr *parser.Expression) (value.Value, error) {
	if expr.BuiltinStandard == "Bags" && expr.Text == "EmptyBag" {
		return value.Function(nil)
	}
	if expr.BuiltinStandard == "TLCExt" && isTLCExtStateOperator(expr.Text) {
		return e.tlcExtStateOperator(expr.Text)
	}
	return e.identifier(expr.Text)
}

func (e *evaluator) evalCompound(expr *parser.Expression) (value.Value, error) {
	switch expr.Kind {
	case "If":
		return e.ifExpression(expr)
	case "Case":
		return e.caseExpression(expr)
	case "Let":
		return e.let(expr)
	case "FunctionSet", "RecordSet", "Product":
		return e.setConstructor(expr)
	case "Enabled":
		return e.enabled(expr)
	case "Prime":
		return e.prime(expr)
	case "Unchanged":
		return e.unchanged(expr)
	default:
		return e.evalCollection(expr)
	}
}

func (e *evaluator) evalCollection(expr *parser.Expression) (value.Value, error) {
	switch expr.Kind {
	case "SetEnumeration":
		return e.set(expr)
	case "Tuple":
		return e.tuple(expr.Args)
	case "FunctionApplication":
		return e.functionApplication(expr)
	case "Record":
		return e.record(expr)
	case "FieldAccess":
		return e.field(expr)
	case "Label":
		return e.eval(expr.Args[0])
	default:
		return e.evalBound(expr)
	}
}

func numericLiteral(text string) (value.Value, error) {
	base := 10
	if strings.HasPrefix(text, "\\") && len(text) > 2 {
		switch strings.ToLower(text[1:2]) {
		case "b":
			base = 2
		case "o":
			base = 8
		case "h":
			base = 16
		}
		text = text[2:]
	}
	n, err := strconv.ParseInt(text, base, 64)
	if err != nil {
		return value.Value{}, fmt.Errorf("invalid integer literal %q", text)
	}
	return value.Integer(n)
}

func (e *evaluator) scoped(name string) (value.Value, bool, error) {
	for s := e.scope; s != nil; s = s.parent {
		if v, ok := s.values[name]; ok {
			return v, true, nil
		}
		if d, ok := s.definitions[name]; ok {
			v, err := e.definition(d, nil, s)
			return v, true, err
		}
	}
	return value.Value{}, false, nil
}

func (e *evaluator) identifier(name string) (value.Value, error) {
	if arg, ok := lookupPrimeArgument(e.scope, name); ok {
		return e.evaluateArgument(arg)
	}
	v, ok, err := e.scoped(name)
	if ok {
		return v, err
	}
	if v, ok = e.context.Current[name]; ok {
		return v, nil
	}
	if v, ok = e.context.Constants[name]; ok {
		return v, nil
	}
	if e.variables[name] {
		return value.Value{}, &UnboundVariable{Name: name}
	}
	return e.standardConstant(name)
}

// UnboundVariable reports a declared variable missing from the current state.
type UnboundVariable struct {
	Name    string
	Sources []*parser.Expression
}

func (e *UnboundVariable) Error() string { return fmt.Sprintf("variable %s is unassigned", e.Name) }
func (e *evaluator) standardConstant(name string) (value.Value, error) {
	if result, ok, err := e.tlcStandardConstant(name); ok {
		return result, err
	}
	if name == "EmptyBag" && e.bags {
		return value.Function(nil)
	}
	if set, ok := e.numericSet(name); ok {
		return set, nil
	}
	return standardConstant(name)
}

func (e *evaluator) tlcStandardConstant(name string) (value.Value, bool, error) {
	if e.tlcext && isTLCExtStateOperator(name) {
		v, err := e.tlcExtStateOperator(name)
		return v, true, err
	}
	if name == "Any" && e.tlc {
		return value.Any(), true, nil
	}
	if name == "JavaTime" && e.tlc {
		v, err := javaTime()
		return v, true, err
	}
	return value.Value{}, false, nil
}

func javaTime() (value.Value, error) {
	return value.Integer(time.Now().UnixMilli() & 0x7fffffff)
}

func (e *evaluator) numericSet(name string) (value.Value, bool) {
	if name == "Nat" && e.naturals {
		return value.NaturalNumbers(), true
	}
	if name == "Int" && e.integers {
		return value.Integers(), true
	}
	return value.Value{}, false
}

func standardConstant(name string) (value.Value, error) {
	switch name {
	case "TRUE":
		return value.Bool(true), nil
	case "FALSE":
		return value.Bool(false), nil
	case "STRING":
		return value.Strings(), nil
	case "BOOLEAN":
		return value.Set([]value.Value{value.Bool(false), value.Bool(true)})
	}
	return value.Value{}, fmt.Errorf("unknown identifier %s", name)
}

func (e *evaluator) definition(d parser.Definition, args []value.Value, defined *scope) (value.Value, error) {
	if d.Kind == "Function" {
		return e.functionDefinition(d, args, nil, defined)
	}
	return e.operatorDefinition(d, args, nil, defined, nil, nil)
}

func (e *evaluator) operatorDefinition(d parser.Definition, args []value.Value, operators map[string]operatorRef, defined *scope, expressions []*parser.Expression, caller *scope) (value.Value, error) {
	if defined == &e.program.root && len(args) == 0 && e.program.constantDefs[d.Name] && !e.context.UncachedConstants {
		return e.constantDefinition(d, args, operators, defined)
	}
	return e.evaluateOperatorDefinition(d, args, operators, defined, expressions, caller)
}

func (e *evaluator) evaluateOperatorDefinition(d parser.Definition, args []value.Value, operators map[string]operatorRef, defined *scope, expressions []*parser.Expression, caller *scope) (value.Value, error) {
	if err := validateCall(d, args); err != nil {
		return value.Value{}, err
	}
	if len(expressions) > 0 && !d.Recursive {
		return e.evaluateOperatorBody(d, args, operators, defined, expressions, caller)
	}
	return e.evaluateGuardedOperator(d, args, operators, defined, expressions, caller)
}

func (e *evaluator) evaluateGuardedOperator(d parser.Definition, args []value.Value, operators map[string]operatorRef, defined *scope, expressions []*parser.Expression, caller *scope) (value.Value, error) {
	if e.active == nil {
		e.active = map[string][]activeCall{}
	}
	key, err := callArgumentsKey(d, args, defined)
	if err != nil {
		return value.Value{}, err
	}
	key = appendOperatorBindings(key, d, operators)
	if len(expressions) > 0 {
		key = lazyRecursiveCallKey(key, expressions, caller)
	}
	repeated, err := e.recursiveStateSeen(key)
	if err != nil {
		return value.Value{}, err
	}
	if repeated {
		return value.Value{}, fmt.Errorf("recursive call to %s with unchanged arguments and state", d.Name)
	}
	e.pushActiveCall(key)
	defer e.popActiveCall(key)
	return e.evaluateOperatorBody(d, args, operators, defined, expressions, caller)
}

func (e *evaluator) evaluateOperatorBody(d parser.Definition, args []value.Value, operators map[string]operatorRef, defined *scope, expressions []*parser.Expression, caller *scope) (value.Value, error) {
	if e.depth >= 4096 {
		return value.Value{}, fmt.Errorf("evaluation recursion limit exceeded")
	}
	bindings, err := valueBindings(d.Params, args, operators)
	if err != nil {
		return value.Value{}, err
	}
	child := *e
	child.depth++
	child.scope = &scope{parent: defined, values: bindings, operators: operators, primeArgs: primeArgumentBindings(d.Params, expressions, caller, e.activeEnabledScopes())}
	child.hasOperators = e.hasOperators || len(operators) > 0
	return child.eval(d.Body)
}

func validateCall(d parser.Definition, args []value.Value) error {
	if d.Kind != "Operator" {
		return fmt.Errorf("unsupported function definition %s", d.Name)
	}
	if len(args) != len(d.Params) {
		return fmt.Errorf("operator %s expects %d arguments", d.Name, len(d.Params))
	}
	return nil
}

func (e *evaluator) findDefinition(name string) (parser.Definition, *scope, bool) {
	for s := e.scope; s != nil; s = s.parent {
		if _, ok := s.values[name]; ok {
			return parser.Definition{}, nil, false
		}
		if _, ok := s.operators[name]; ok {
			return parser.Definition{}, nil, false
		}
		if d, ok := s.definitions[name]; ok {
			return d, s, true
		}
	}
	return parser.Definition{}, nil, false
}
