package checker

import (
	"fmt"
	"maps"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

type symbol struct {
	arity  int
	params []int
}

type sourceSymbol struct {
	module   string
	position parser.Position
}

type resolver struct {
	proofModule   *parser.Module
	globals       map[string]symbol
	positions     map[string]parser.Position
	sources       map[string]sourceSymbol
	reference     sourceSymbol
	assumeProve   map[string]bool
	position      parser.Position
	proofScope    bool
	blockedSelf   map[string]bool
	arithmetic    bool
	finiteSets    bool
	sequences     bool
	integers      bool
	tlc           bool
	tlcext        bool
	bags          bool
	json          bool
	randomization bool
}

var (
	bagOperators        = map[string]int{"IsABag": 1, "BagCardinality": 1, "BagIn": 2, "CopiesIn": 2, "\\oplus": 2, "\\ominus": 2, "BagUnion": 1, "\\sqsubseteq": 2, "BagOfAll": 2, "SubBag": 1, "BagToSet": 1, "SetToBag": 1}
	jsonOperators       = map[string]int{"ToJson": 1, "ToJsonArray": 1, "ToJsonObject": 1, "JsonSerialize": 2, "ndJsonSerialize": 2, "JsonDeserialize": 1, "ndJsonDeserialize": 1}
	randomOperators     = map[string]int{"RandomSubset": 2, "RandomSetOfSubsets": 3, "TestRandomSetOfSubsets": 3}
	tlcextOperators     = map[string]int{"Trace": 0, "CounterExample": 0, "TLCDefer": 1, "TLCNoOp": 1, "TLCModelValue": 1, "AssertEq": 2, "AssertError": 2, "TLCGetOrDefault": 2, "TLCGetAndSet": 4, "ToTrace": 1}
	tlcOperators        = map[string]int{":>": 2, "@@": 2, "ToString": 1, "TLCEval": 1, "Permutations": 1, "Assert": 2, "SortSeq": 2, "Print": 2, "PrintT": 1, "TLCGet": 1, "TLCSet": 2, "RandomElement": 1}
	sequenceOperators   = map[string]int{"Seq": 1, "Len": 1, "Append": 2, "Head": 1, "Tail": 1, "SubSeq": 3, "\\o": 2, "SelectSeq": 2}
	coreOperators       = map[string]int{"=": 2, "#": 2, "~": 1, "<=>": 2, "=>": 2, "~>": 2, "-+->": 2, "/\\": 2, "\\/": 2, "\\in": 2, "\\notin": 2, "\\subseteq": 2, "\\cup": 2, "\\cap": 2, "\\": 2, "DOMAIN": 1, "SUBSET": 1, "UNION": 1}
	arithmeticOperators = map[string]int{"+": 2, "-": 2, "-.": 1, "*": 2, "^": 2, "\\div": 2, "%": 2, "..": 2, "<": 2, "<=": 2, ">": 2, ">=": 2}
)

func validateResolved(module *parser.Module) error {
	r := resolver{proofModule: module, globals: map[string]symbol{"TRUE": {}, "FALSE": {}, "BOOLEAN": {}, "STRING": {}}, positions: map[string]parser.Position{}, sources: map[string]sourceSymbol{}, assumeProve: map[string]bool{}}
	for _, name := range module.Extends {
		r.includeModule(name)
	}
	if err := r.declarations(module); err != nil {
		return err
	}
	if err := r.validateBodies(module); err != nil {
		return err
	}
	if err := r.validateTheoremProofs(module); err != nil {
		return err
	}
	return r.validateProofDirectives(module)
}

func (r *resolver) includeModule(name string) {
	r.json = r.json || name == "Json"
	r.randomization = r.randomization || name == "Randomization"
	r.includeOtherModule(name)
}

func (r *resolver) includeOtherModule(name string) {
	switch name {
	case "Bags":
		r.bags = true
	case "TLC":
		r.tlc = true
	case "TLCExt":
		r.tlcext = true
	case "Integers":
		r.integers = true
	case "Sequences":
		r.sequences = true
	case "FiniteSets":
		r.finiteSets = true
	}
	r.arithmetic = r.arithmetic || standardHasNaturals(name)
}

func standardHasNaturals(name string) bool {
	switch name {
	case "Naturals", "Integers", "Sequences":
		return true
	default:
		return false
	}
}

func (r *resolver) declarations(module *parser.Module) error {
	if err := r.declareSourceSymbols(module, module.Variables, false); err != nil {
		return err
	}
	if err := r.declareSourceSymbols(module, module.Constants, true); err != nil {
		return err
	}
	for _, d := range module.Definitions {
		if err := r.declareDefinition(d); err != nil {
			return err
		}
	}
	r.declareRecursiveSources(module.RecursiveDeclarations)
	return nil
}

func (r *resolver) declareSourceSymbols(module *parser.Module, declarations []parser.Declaration, operators bool) error {
	for _, declaration := range declarations {
		arity := 0
		if operators {
			arity = declaration.Arity
		}
		if err := r.declare(declaration.Name, arity); err != nil {
			return err
		}
		r.positions[declaration.Name] = declaration.Location.Position
		r.sources[declaration.Name] = sourceDeclaration(module, declaration)
	}
	return nil
}

func (r *resolver) declareRecursiveSources(declarations []parser.Declaration) {
	for _, d := range declarations {
		if source, ok := r.sources[d.Name]; ok && knownPositionAfter(source.position, d.Location.Position) {
			source.position = d.Location.Position
			r.sources[d.Name] = source
		}
	}
}

func (r *resolver) declareDefinition(d parser.Definition) error {
	if err := r.declare(d.Name, len(d.Params)); err != nil {
		return err
	}
	r.globals[d.Name] = definitionSymbol(d)
	if d.Body != nil && d.Body.Kind == "AssumeProve" {
		r.assumeProve[d.Name] = true
	}
	r.positions[d.Name] = d.Location.Position
	if len(d.Origins) == 0 {
		r.sources[d.Name] = sourceSymbolLocation(d.Location)
	} else if d.InstanceRange.Module != "" {
		r.sources[d.Name] = sourceSymbol{module: d.InstanceRange.Module, position: d.InstanceRange.Start}
	}
	return nil
}

func (r *resolver) validateBodies(module *parser.Module) error {
	for _, d := range module.Definitions {
		if err := r.definition(d, nil); err != nil {
			return err
		}
	}
	for _, a := range module.Assumptions {
		r.position = a.Location.Position
		if err := r.expression(a.Expression, nil); err != nil {
			return err
		}
	}
	return r.validateAnonymousTheorems(module.Theorems)
}

func (r *resolver) validateAnonymousTheorems(theorems []parser.Theorem) error {
	for _, theorem := range theorems {
		if theorem.Name == "" {
			r.position = theorem.Location.Position
			if err := r.expression(theorem.Statement, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *resolver) declare(name string, arity int) error {
	if _, ok := r.globals[name]; ok {
		return fmt.Errorf("duplicate declaration %s", name)
	}
	r.globals[name] = symbol{arity: arity}
	return nil
}

func (r *resolver) definition(d parser.Definition, scope map[string]symbol) error {
	if len(d.InstanceStandardModules) != 0 {
		local := *r
		for _, name := range d.InstanceStandardModules {
			local.includeModule(name)
		}
		return local.definitionInScope(d, scope)
	}
	return r.definitionInScope(d, scope)
}

func (r *resolver) definitionInScope(d parser.Definition, scope map[string]symbol) error {
	previous := r.position
	if !r.proofScope && scope == nil {
		r.position = d.Location.Position
	}
	defer func() { r.position = previous }()
	if d.Kind != "Operator" && d.Kind != "Function" {
		return fmt.Errorf("unsupported definition %s", d.Name)
	}
	local := cloneScope(scope)
	if d.Kind == "Operator" && !d.Recursive {
		delete(local, d.Name)
	}
	if err := r.definitionParameters(d, local); err != nil {
		return err
	}
	return r.scopedDefinitionBody(d, local)
}

func (r *resolver) scopedDefinitionBody(d parser.Definition, local map[string]symbol) error {
	// Linked instances and internal definitions may contain substituted names;
	// their source-level binding cannot be inferred from the generated body.
	if d.Kind == "Operator" && !d.Recursive && !r.proofScope && len(d.Origins) == 0 && !strings.Contains(d.Name, "!") {
		bodyResolver := *r
		bodyResolver.blockedSelf = maps.Clone(r.blockedSelf)
		if bodyResolver.blockedSelf == nil {
			bodyResolver.blockedSelf = make(map[string]bool)
		}
		bodyResolver.blockedSelf[d.Name] = true
		return bodyResolver.definitionBody(d, local)
	}
	return r.definitionBody(d, local)
}

func (r *resolver) definitionBody(d parser.Definition, local map[string]symbol) error {
	for _, bound := range d.Bounds {
		if err := r.bound(bound, local); err != nil {
			return err
		}
	}
	if d.Kind == "Function" {
		local[d.Name] = symbol{}
	}
	return r.expression(d.Body, local)
}

func (r *resolver) definitionParameters(d parser.Definition, local map[string]symbol) error {
	for _, param := range d.Params {
		if param.Name == d.Name {
			return fmt.Errorf("parameter %s conflicts with its operator", param.Name)
		}
		if err := r.bound(parser.Bound{Names: []string{param.Name}}, local); err != nil {
			return err
		}
		local[param.Name] = symbol{arity: param.Arity}
	}
	return nil
}

// SANY allows a parameter to use a name defined later in the module.
func (r *resolver) declaredBefore(name string) bool {
	// A non-recursive definition is not visible in its own body, including
	// when checking whether a new local binding conflicts with a global.
	if r.blockedSelf[name] {
		return false
	}
	if _, exists := r.globals[name]; !exists {
		return false
	}
	pos := r.positions[name]
	if pos.Line == 0 || r.position.Line == 0 {
		return true
	}
	return pos.Line < r.position.Line || pos.Line == r.position.Line && pos.Column <= r.position.Column
}

func definitionSymbol(d parser.Definition) symbol {
	params := make([]int, len(d.Params))
	for i, param := range d.Params {
		params[i] = param.Arity
	}
	return symbol{arity: len(d.Params), params: params}
}

func cloneScope(scope map[string]symbol) map[string]symbol {
	copy := make(map[string]symbol, len(scope))
	maps.Copy(copy, scope)
	return copy
}

func (r *resolver) lookup(name string, scope map[string]symbol) (symbol, bool) {
	if arity, ok := scope[name]; ok {
		return arity, true
	}
	if r.blockedSelf[name] {
		return symbol{}, false
	}
	if arity, ok := r.globals[name]; ok && r.visibleGlobal(name) {
		return arity, true
	}
	if arity, ok := coreOperators[name]; ok {
		return symbol{arity: arity}, true
	}
	return r.lookupStandard(name)
}

func (r *resolver) visibleGlobal(name string) bool {
	if r.proofScope {
		return true
	}
	source, ok := r.sources[name]
	if !ok || source.module == "" || source.module != r.reference.module || source.position.Line == 0 || r.reference.position.Line == 0 {
		return true
	}
	return !knownPositionAfter(source.position, r.reference.position)
}

func sourceSymbolLocation(location parser.Location) sourceSymbol {
	return sourceSymbol{module: location.Range.Module, position: location.Range.Start}
}

func sourceDeclaration(module *parser.Module, declaration parser.Declaration) sourceSymbol {
	if location, ok := module.SourceDeclarations[declaration.Name]; ok {
		return sourceSymbolLocation(location)
	}
	if declaration.Location.Range.Module == "" && len(module.SourceTokens) == 0 {
		return sourceSymbol{}
	}
	return sourceSymbolLocation(declarationSourceLocation(module.Name, declaration.Location))
}

func (r *resolver) lookupStandard(name string) (symbol, bool) {
	if arity, ok := randomOperators[name]; r.randomization && ok {
		return symbol{arity: arity}, true
	}
	return r.lookupJSONStandard(name)
}

func (r *resolver) lookupJSONStandard(name string) (symbol, bool) {
	if arity, ok := jsonOperators[name]; r.json && ok {
		return symbol{arity: arity}, true
	}
	return r.lookupNonJSONStandard(name)
}

func (r *resolver) lookupNonJSONStandard(name string) (symbol, bool) {
	if r.tlc && (name == "Any" || name == "JavaTime") {
		return symbol{}, true
	}
	if r.arithmetic && name == "Nat" || r.integers && name == "Int" {
		return symbol{}, true
	}
	return r.lookupExtended(name)
}

func (r *resolver) lookupExtended(name string) (symbol, bool) {
	if arity, ok := tlcextOperators[name]; r.tlcext && ok {
		if name == "TLCGetAndSet" {
			return symbol{arity: arity, params: []int{0, 2, 0, 0}}, true
		}
		return symbol{arity: arity}, true
	}
	if r.bags {
		if sym, ok := lookupBagSymbol(name); ok {
			return sym, true
		}
	}
	if sym, ok := r.lookupTLC(name); ok {
		return sym, true
	}
	return r.lookupOtherStandards(name)
}

func (r *resolver) lookupTLC(name string) (symbol, bool) {
	arity, ok := tlcOperators[name]
	if !r.tlc || !ok {
		return symbol{}, false
	}
	if name == "SortSeq" {
		return symbol{arity: arity, params: []int{0, 2}}, true
	}
	return symbol{arity: arity}, true
}

func (r *resolver) lookupOtherStandards(name string) (symbol, bool) {
	if name == "-." && !r.integers {
		return symbol{}, false
	}
	if r.sequences {
		if arity, ok := sequenceOperators[name]; ok {
			if name == "SelectSeq" {
				return symbol{arity: arity, params: []int{0, 1}}, true
			}
			return symbol{arity: arity}, true
		}
	}
	return r.lookupArithmeticAndSets(name)
}

func (r *resolver) lookupArithmeticAndSets(name string) (symbol, bool) {
	if r.finiteSets && (name == "Cardinality" || name == "IsFiniteSet") {
		return symbol{arity: 1}, true
	}
	if r.arithmetic {
		arity, ok := arithmeticOperators[name]
		return symbol{arity: arity}, ok
	}
	return symbol{}, false
}

func (r *resolver) checkName(name string, arity int, scope map[string]symbol) error {
	if r.proofScope && !r.proofNameInScope(name, scope) {
		return &SemanticError{Err: fmt.Errorf("fact %s is referenced before its declaration", name)}
	}
	expected, ok := r.lookup(name, scope)
	if !ok {
		return &SemanticError{Err: fmt.Errorf("undefined symbol %s", name)}
	}
	if expected.arity != arity {
		return &SemanticError{Err: fmt.Errorf("operator %s expects %d arguments, got %d", name, expected.arity, arity)}
	}
	return nil
}

func lookupBagSymbol(name string) (symbol, bool) {
	if name == "EmptyBag" {
		return symbol{}, true
	}
	arity, ok := bagOperators[name]
	if name == "BagOfAll" {
		return symbol{arity: arity, params: []int{1, 0}}, true
	}
	return symbol{arity: arity}, ok
}
