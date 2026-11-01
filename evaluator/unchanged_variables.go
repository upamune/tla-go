package evaluator

import (
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// unchangedVariables avoids constructing two temporary tuples for the common
// case UNCHANGED <<x, y, ...>>. The slow path is required for applicable
// lexical substitutions, instrumented evaluation, incomplete states and nonvariables.
func (e *evaluator) unchangedVariables(expr *parser.Expression) (bool, bool, error) {
	if !e.canOptimizeUnchanged(expr) {
		return false, false, nil
	}
	tuple := e.unchangedVariableTuple(expr.Args[0])
	if tuple == nil || tuple.Kind != "Tuple" || len(tuple.Args) > 8 {
		return false, false, nil
	}
	var current, next [8]value.Value
	if !e.loadUnchangedValues(tuple, &current, &next) {
		return false, false, nil
	}
	same, err := compareUnchangedValues(current[:len(tuple.Args)], next[:len(tuple.Args)])
	return same, true, err
}

func compareUnchangedValues(current, next []value.Value) (bool, error) {
	for i := range current {
		same, err := value.Equal(current[i], next[i])
		if err != nil || !same {
			return false, err
		}
	}
	return true, nil
}

func (e *evaluator) canOptimizeUnchanged(expr *parser.Expression) bool {
	if e.context.OnEvaluate != nil || e.context.Next == nil || len(expr.Args) != 1 {
		return false
	}
	if e.context.UncachedArguments {
		_, bound := e.context.Lexical.EnabledVariable(expr)
		return !bound
	}
	return true
}

func (e *evaluator) loadUnchangedValues(tuple *parser.Expression, current, next *[8]value.Value) bool {
	for i, arg := range tuple.Args {
		if !e.directStateVariable(arg) {
			return false
		}
		var ok bool
		current[i], ok = e.context.Current[arg.Text]
		if !ok {
			return false
		}
		next[i], ok = e.context.Next[arg.Text]
		if !ok {
			return false
		}
		if e.observableStringIntern(current[i], next[i]) {
			return false
		}
	}
	return true
}

// A custom ordering may mutate the states when a string is interned. The
// ordinary evaluator interleaves those callbacks with variable lookup.
func (e *evaluator) observableStringIntern(current, next value.Value) bool {
	return e.program.ordering != nil && (current.Kind() == value.StringKind || next.Kind() == value.StringKind)
}

func (e *evaluator) unchangedVariableTuple(expr *parser.Expression) *parser.Expression {
	if !plainIdentifier(expr) {
		return expr
	}
	d, scope, ok := e.findDefinition(expr.Text)
	if !ok || !e.isUnchangedTupleAlias(d) {
		return expr
	}
	for _, call := range e.active {
		if call.key.scope == scope && call.key.name == d.Name && call.key.arguments == "" {
			return expr
		}
	}
	return d.Body
}

func (e *evaluator) isUnchangedTupleAlias(d parser.Definition) bool {
	return d.Kind == "Operator" && len(d.Params) == 0 && !d.Recursive && e.depth < 4096 && !e.program.constantDefs[d.Name]
}

func plainIdentifier(expr *parser.Expression) bool {
	return expr != nil && expr.Kind == "Identifier" && !expr.BuiltinTLC && !expr.BuiltinNatural && expr.BuiltinStandard == ""
}

func (e *evaluator) directStateVariable(expr *parser.Expression) bool {
	if !plainIdentifier(expr) || !e.variables[expr.Text] {
		return false
	}
	for s := e.scope; s != nil; s = s.parent {
		if s.hasValue(expr.Text) {
			return false
		}
		if _, ok := s.definitions[expr.Text]; ok {
			return false
		}
		if _, ok := s.operators[expr.Text]; ok {
			return false
		}
	}
	return true
}
