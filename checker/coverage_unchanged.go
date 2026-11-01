package checker

import (
	"math"
	"slices"
	"strconv"

	"github.com/upamune/tla-go/syntax/parser"
)

// A direct UNCHANGED action has one TLC cost node. Restrict it to all the
// model variables so its only continuation is always a complete successor.
func (m *model) addCoverageUnchangedNext(c *coverageCollector, next parser.Definition) bool {
	init := m.defs[c.initial]
	if !m.coverageDirectConfiguration(c, init, next) {
		return false
	}
	if !m.coverageDirectInitial(init.Body) || !m.coverageUnchangedSequence(next.Body) || !m.coverageBodySupported(next.Body, false) {
		return false
	}
	return c.add(next, false)
}

// Each conjunct must independently assign every variable. The first assigns
// the next state; later UNCHANGED conjuncts are evaluated as state guards.
func (m *model) coverageUnchangedSequence(expr *parser.Expression) bool {
	items := []*parser.Expression{expr}
	if expr.Kind == "And" {
		items = expr.Args
	}
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if item == nil || item.Kind != "Unchanged" || len(item.Args) != 1 || !m.coverageUnchangedOperand(item.Args[0], map[string]bool{}) {
			return false
		}
	}
	return true
}

func (m *model) coverageUnchangedOperand(expr *parser.Expression, seen map[string]bool) bool {
	if expr == nil {
		return false
	}
	if expr.Kind == "Identifier" {
		if len(m.names) == 1 && expr.Text == m.names[0] {
			return true
		}
		return m.coverageUnchangedAlias(expr.Text, seen)
	}
	return m.coverageUnchangedTuple(expr)
}

func (m *model) coverageUnchangedAlias(name string, seen map[string]bool) bool {
	d, ok := m.defs[name]
	if !ok || seen[name] || !coverageDirectDefinition(d) || m.trueConfigured[d.Name] || d.Body == nil {
		return false
	}
	seen[name] = true
	defer delete(seen, name)
	return m.coverageUnchangedOperand(d.Body, seen)
}

func (m *model) coverageDirectConfiguration(c *coverageCollector, init, next parser.Definition) bool {
	return coverageDirectDefinition(init) && coverageDirectDefinition(next) && next.Name != c.initial && !m.trueConfigured[next.Name] && len(m.cfg.Constraints) == 0
}

// Initial guards can change TLC's cost of a preceding membership, and an
// incomplete initial state follows a different error path. Accept only direct
// assignments of total literal values or literal intervals to every variable.
func (m *model) coverageDirectInitial(body *parser.Expression) bool {
	items := []*parser.Expression{body}
	if body.Kind == "And" {
		items = body.Args
	}
	seen := make(map[string]bool, len(m.names))
	var operator string
	for _, item := range items {
		if !m.coverageInitialAssignment(item, seen) {
			return false
		}
		if operator != "" && operator != item.Text {
			return false
		}
		operator = item.Text
	}
	return len(seen) == len(m.names) && (operator != "\\in" || len(m.names) == 1)
}

func (m *model) coverageInitialAssignment(e *parser.Expression, seen map[string]bool) bool {
	if e.Kind != "Apply" || len(e.Args) != 2 || e.Args[0].Kind != "Identifier" {
		return false
	}
	name := e.Args[0].Text
	if !slices.Contains(m.names, name) || seen[name] || !coverageInitialLiteral(e.Text, e.Args[1]) {
		return false
	}
	seen[name] = true
	return true
}

func coverageInitialLiteral(op string, e *parser.Expression) bool {
	if op == "=" {
		return coverageInitialNumber(e) || e.Kind == "Identifier" && (e.Text == "TRUE" || e.Text == "FALSE")
	}
	return op == "\\in" && (coverageInitialInterval(e) || coverageInitialSet(e))
}

// Restrict explicit sets to literals of one scalar type. Mixed values can
// trigger TLC's type-sensitive sorting errors while constructing the set.
func coverageInitialSet(e *parser.Expression) bool {
	if e == nil || e.Kind != "SetEnumeration" {
		return false
	}
	kind := ""
	for _, item := range e.Args {
		current := coverageInitialSetItem(item)
		if current == "" || kind != "" && kind != current {
			return false
		}
		kind = current
	}
	return true
}

func coverageInitialSetItem(e *parser.Expression) string {
	if e == nil {
		return ""
	}
	if coverageInitialNumber(e) {
		return "number"
	}
	if e.Kind == "Identifier" && (e.Text == "TRUE" || e.Text == "FALSE") {
		return "boolean"
	}
	return ""
}

func coverageInitialInterval(e *parser.Expression) bool {
	if !coverageLiteralInterval(e) || !coverageInitialNumber(e.Args[0]) || !coverageInitialNumber(e.Args[1]) {
		return false
	}
	lo, _ := strconv.ParseUint(e.Args[0].Text, 10, 31)
	hi, _ := strconv.ParseUint(e.Args[1].Text, 10, 31)
	// Java TLC's interval enumerator increments an int after yielding the
	// upper endpoint. At MaxInt32 it wraps and does not terminate normally.
	return hi < lo || hi < math.MaxInt32
}

func coverageInitialNumber(e *parser.Expression) bool {
	if e.Kind != "Number" {
		return false
	}
	_, err := strconv.ParseUint(e.Text, 10, 31)
	return err == nil
}

func coverageDirectDefinition(d parser.Definition) bool {
	return d.Kind == "Operator" && !d.Recursive && !d.InstanceSubstituted && !d.ConfigConstant && d.ConfigSubstitutionTarget == "" && len(d.Params) == 0
}

func (m *model) coverageUnchangedVariables(expr *parser.Expression) bool {
	if expr.Kind != "Unchanged" || len(expr.Args) != 1 {
		return false
	}
	operand := expr.Args[0]
	if operand.Kind == "Identifier" {
		return len(m.names) == 1 && operand.Text == m.names[0]
	}
	return m.coverageUnchangedTuple(operand)
}

func (m *model) coverageUnchangedTuple(expr *parser.Expression) bool {
	if expr.Kind != "Tuple" || len(expr.Args) != len(m.names) || len(expr.Args) == 0 {
		return false
	}
	seen := make(map[string]bool, len(expr.Args))
	for _, arg := range expr.Args {
		if arg.Kind != "Identifier" || seen[arg.Text] || !slices.Contains(m.names, arg.Text) {
			return false
		}
		seen[arg.Text] = true
	}
	return true
}
