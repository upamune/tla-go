package checker

import (
	"math"
	"slices"
	"strconv"

	"github.com/upamune/tla-go/syntax"
	"github.com/upamune/tla-go/syntax/parser"
)

// A call occurrence selects a root; repeated calls of one predicate share it.
// The predicate may contain a separate cost-bearing interval expression.
type coverageParameterizedRoot struct {
	predicate *parser.Expression
	action    int
}

// The linked definition and its predicate identify a root in this subset.
// Instantiated and overridden definitions are rejected until their context is
// represented explicitly.
type coverageParameterizedIdentity struct {
	predicate *parser.Expression
	module    string
	name      string
}

func (m *model) addCoverageParameterizedCalls(c *coverageCollector, branches []*parser.Expression) bool {
	if !m.coverageParameterizedConfig(c) {
		return false
	}
	c.parameterCalls = make(map[*parser.Expression]*coverageParameterizedRoot, len(branches))
	roots := make(map[coverageParameterizedIdentity]*coverageParameterizedRoot)
	for _, branch := range branches {
		if !m.addCoverageParameterizedCall(c, roots, branch) {
			return false
		}
	}
	ranges := make([]parser.Range, 0, len(roots))
	for _, root := range roots {
		ranges = append(ranges, root.predicate.Location.Range)
	}
	return coverageSameModuleDistinct(ranges)
}

func (m *model) coverageParameterizedConfig(c *coverageCollector) bool {
	init := m.defs[c.initial]
	return len(m.names) == 1 && !m.fromSpecification && m.graph == nil && m.tlcRuntime.options.MaxSetSize <= 0 && m.tlcRuntime.options.Workers <= 1 && coverageDirectDefinition(init) &&
		m.coverageDirectInitial(init.Body) && len(m.cfg.Invariants)+len(m.cfg.Constraints)+len(m.cfg.Constants)+len(m.cfg.Substitutions) == 0
}

func (m *model) addCoverageParameterizedCall(c *coverageCollector, roots map[coverageParameterizedIdentity]*coverageParameterizedRoot, call *parser.Expression) bool {
	if call.Kind != "Apply" {
		return false
	}
	d, ok := m.defs[call.Text]
	if !ok || !m.coverageParameterizedDefinition(d, call) {
		return false
	}
	identity := coverageParameterizedIdentity{predicate: d.Body, module: d.Location.Range.Module, name: d.Name}
	root := roots[identity]
	if root == nil {
		var found bool
		root, found = m.newCoverageParameterizedRoot(c, d)
		if !found {
			return false
		}
		roots[identity] = root
	}
	c.parameterCalls[call] = root
	return true
}

func (m *model) newCoverageParameterizedRoot(c *coverageCollector, d parser.Definition) (*coverageParameterizedRoot, bool) {
	if _, exists := c.byName[d.Name]; exists {
		return nil, false
	}
	head, ok := m.coverageParameterizedHead(d)
	if !ok || !m.coverageBodySupported(d.Body, false) || !c.add(d, false) {
		return nil, false
	}
	index := c.byName[d.Name]
	c.actions[index].Range = head
	return &coverageParameterizedRoot{predicate: d.Body, action: index}, true
}

func (m *model) coverageParameterizedDefinition(d parser.Definition, call *parser.Expression) bool {
	if !coverageParameterizedOperator(d) || m.trueConfigured[d.Name] {
		return false
	}
	if len(d.Params) == 0 || len(d.Params) != len(call.Args) || !m.coverageParameterizedArguments(d.Params, call.Args) {
		return false
	}
	return m.coverageParameterizedBody(d) && coverageParameterizedBodyArguments(d, call)
}

func coverageParameterizedBodyArguments(d parser.Definition, call *parser.Expression) bool {
	if d.Body.Kind == "Apply" && d.Body.Text == "\\in" {
		return coverageParameterizedIntervalArguments(d.Params, call.Args, d.Body)
	}
	return true
}

func coverageParameterizedIntervalArguments(params []parser.Declaration, args []*parser.Expression, body *parser.Expression) bool {
	if len(body.Args) != 2 || body.Args[1] == nil || !coverageParameterizedInterval(params, body.Args[1]) {
		return false
	}
	for _, end := range body.Args[1].Args {
		if !coverageParameterizedEndpointArgument(params, args, end) {
			return false
		}
	}
	return coverageParameterizedFiniteUpper(params, args, body.Args[1].Args[1])
}

// TLC 1.7.4 wraps its interval enumerator after MaxInt32 rather than stopping.
func coverageParameterizedFiniteUpper(params []parser.Declaration, args []*parser.Expression, end *parser.Expression) bool {
	for i, param := range params {
		if end.Kind == "Identifier" && end.Text == param.Name {
			end = args[i]
			break
		}
	}
	n, err := strconv.ParseUint(end.Text, 10, 31)
	return err == nil && n < math.MaxInt32
}

func coverageParameterizedEndpointArgument(params []parser.Declaration, args []*parser.Expression, end *parser.Expression) bool {
	for i, param := range params {
		if end.Kind == "Identifier" && end.Text == param.Name {
			return coverageInitialNumber(args[i])
		}
	}
	return true
}

func coverageParameterizedOperator(d parser.Definition) bool {
	return d.Kind == "Operator" && !d.Recursive && !d.InstanceSubstituted && !d.ConfigConstant && d.ConfigSubstitutionTarget == ""
}

func (m *model) coverageParameterizedArguments(params []parser.Declaration, args []*parser.Expression) bool {
	var names []string
	for i, param := range params {
		_, definition := m.defs[param.Name]
		if param.Arity != 0 || definition || slices.Contains(m.names, param.Name) || slices.Contains(names, param.Name) || args[i] == nil || !coverageInitialLiteral("=", args[i]) {
			return false
		}
		names = append(names, param.Name)
	}
	return true
}

func (m *model) coverageParameterizedBody(d parser.Definition) bool {
	e := d.Body
	if e == nil {
		return false
	}
	if e.Kind == "Unchanged" {
		return m.coverageUnchangedVariables(e)
	}
	if !coverageParameterizedAssignment(e, m.names[0]) {
		return false
	}
	if e.Text == "\\in" {
		return coverageParameterizedInterval(d.Params, e.Args[1])
	}
	if coverageInitialLiteral("=", e.Args[1]) {
		return true
	}
	return coverageParameterizedFormal(d.Params, e.Args[1])
}

func coverageParameterizedAssignment(e *parser.Expression, name string) bool {
	return e.Kind == "Apply" && (e.Text == "=" || e.Text == "\\in") && len(e.Args) == 2 && e.Args[0] != nil && e.Args[1] != nil && coveragePrimeVariable(e.Args[0], name)
}

func coverageParameterizedInterval(params []parser.Declaration, e *parser.Expression) bool {
	if e.Kind != "Apply" || e.Text != ".." || len(e.Args) != 2 {
		return false
	}
	for _, end := range e.Args {
		if !coverageInitialNumber(end) && !coverageParameterizedFormal(params, end) {
			return false
		}
	}
	return true
}

func coverageParameterizedFormal(params []parser.Declaration, expr *parser.Expression) bool {
	if expr == nil || expr.Kind != "Identifier" {
		return false
	}
	for _, param := range params {
		if expr.Text == param.Name {
			return true
		}
	}
	return false
}

// Source tokens preserve the closing parenthesis, including multiline heads.
// Modules constructed without those tokens cannot provide this range safely.
func (m *model) coverageParameterizedHead(d parser.Definition) (parser.Range, bool) {
	module := m.sourceUnits[d.Location.Range.Module]
	if module == nil {
		return parser.Range{}, false
	}
	for i, token := range module.SourceTokens {
		if token.Line == d.Location.Range.Start.Line && token.Column == d.Location.Range.Start.Column && token.Text == d.Name {
			return coverageParameterTokenSpan(d.Location.Range, module.SourceTokens[i:])
		}
	}
	return parser.Range{}, false
}

func coverageParameterTokenSpan(r parser.Range, tokens []syntax.Token) (parser.Range, bool) {
	if len(tokens) < 3 || tokens[1].Text != "(" {
		return parser.Range{}, false
	}
	depth := 0
	for _, token := range tokens[1:] {
		switch token.Text {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				r.End = parser.Position{Line: token.Line, Column: token.EndColumn}
				return r, true
			}
		case "==":
			return parser.Range{}, false
		}
	}
	return parser.Range{}, false
}
