package evaluator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (e *evaluator) evalArgs(args []*parser.Expression) ([]value.Value, error) {
	result := make([]value.Value, 0, len(args))
	for _, arg := range args {
		v, err := e.eval(arg)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, nil
}

func (e *evaluator) apply(expr *parser.Expression) (value.Value, error) {
	if expr.Text == "~>" || expr.Text == "-+->" {
		return temporalEvaluationError(expr)
	}
	if isMarkedBuiltin(expr) {
		return e.markedBuiltin(expr)
	}
	if e.hasOperators {
		if op, ok := e.findOperator(expr.Text); ok {
			return e.applyOperator(op, expr.Args)
		}
	}
	if expr.Text == "=>" {
		return e.implies(expr)
	}
	if def, scope, ok := e.findDefinition(expr.Text); ok {
		return e.applyDefinition(def, expr.Args, scope)
	}
	return e.builtin(expr)
}

func isMarkedBuiltin(expr *parser.Expression) bool {
	return expr.BuiltinTLC || expr.BuiltinNatural || expr.BuiltinStandard != ""
}

func (e *evaluator) markedBuiltin(expr *parser.Expression) (value.Value, error) {
	if expr.BuiltinTLC {
		builtin := *expr
		builtin.Text = strings.TrimPrefix(expr.Text, "!tlc!")
		return e.tlcOperator(&builtin)
	}
	if expr.BuiltinNatural {
		return e.applyNaturalBuiltin(expr)
	}
	return e.applyStandardBuiltin(expr)
}

func (e *evaluator) applyOperator(op operatorRef, expressions []*parser.Expression) (value.Value, error) {
	if op.definition != nil {
		return e.applyDefinition(*op.definition, expressions, op.scope)
	}
	if op.lambda != nil {
		return e.invokeLambdaExpressions(op, expressions)
	}
	if kind, ok := parser.SpecialOperatorKind(op.name); ok {
		return e.eval(&parser.Expression{Kind: kind, Args: expressions})
	}
	args, err := e.evalArgs(expressions)
	if err != nil {
		return value.Value{}, err
	}
	return e.invokeOperator(op, args)
}

func (e *evaluator) builtin(expr *parser.Expression) (value.Value, error) {
	if e.randomization && isRandomOperator(expr.Text) {
		return e.randomOperator(expr)
	}
	if e.bags && isBagOperator(expr.Text) {
		return e.bagOperator(expr)
	}
	if e.json && isJSONOperator(expr.Text) {
		return e.jsonOperator(expr)
	}
	return e.otherBuiltin(expr)
}

func (e *evaluator) otherBuiltin(expr *parser.Expression) (value.Value, error) {
	if e.isTLCBuiltin(expr.Text) {
		return e.callTLCBuiltin(expr)
	}
	if e.sequences && isSequenceOperator(expr.Text) {
		return e.sequenceOperator(expr)
	}
	if e.finiteSets && (expr.Text == "Cardinality" || expr.Text == "IsFiniteSet") {
		return e.finiteSetsOperator(expr)
	}
	return e.builtinByArity(expr)
}

func (e *evaluator) builtinByArity(expr *parser.Expression) (value.Value, error) {
	if len(expr.Args) == 1 {
		return e.unaryBuiltin(expr)
	}
	if len(expr.Args) == 2 {
		left, err := e.eval(expr.Args[0])
		if err != nil {
			return value.Value{}, err
		}
		right, err := e.eval(expr.Args[1])
		if err != nil {
			return value.Value{}, err
		}
		result, err := e.binaryWithSource(expr, left, right)
		if err != nil {
			return result, err
		}
		return e.orderSetResult(expr.Text, result)
	}
	return value.Value{}, fmt.Errorf("unknown operator %s with %d arguments", expr.Text, len(expr.Args))
}

func (e *evaluator) binaryWithSource(expr *parser.Expression, left, right value.Value) (value.Value, error) {
	if expr.Text == "\\cap" {
		return value.SetOperationWithSource(expr.Text, left, right, expr)
	}
	result, err := comparisonCheckedBinary(expr.Text, left, right, e.naturals && !e.integers)
	if err != nil && expr.Text == "<=>" {
		if _, ok := errors.AsType[*nonBooleanError](err); ok {
			message := "Attempted to evaluate an expression of form P <=> Q when P or Q was not a boolean."
			return value.Value{}, booleanRuntimeError(message, expr, nil)
		}
	}
	return result, err
}

func (e *evaluator) unaryBuiltin(expr *parser.Expression) (value.Value, error) {
	v, err := e.eval(expr.Args[0])
	if err != nil {
		return value.Value{}, err
	}
	if expr.Text == "SUBSET" {
		return value.SubsetsWithSource(v, expr)
	}
	result, err := unary(expr.Text, v)
	if err != nil && expr.Text == "~" {
		message := fmt.Sprintf("Attempted to apply the operator ~ to a non-boolean\n(%s)", v.TLCKindDescription())
		return value.Value{}, booleanRuntimeError(message, expr, nil)
	}
	return result, err
}

func boolean(v value.Value) (bool, error) {
	b, ok := v.Bool()
	if !ok {
		return false, &nonBooleanError{value: v}
	}
	return b, nil
}

func (e *evaluator) condition(expr *parser.Expression) (bool, error) {
	v, err := e.eval(expr)
	if err != nil {
		return false, err
	}
	if b, ok := v.Bool(); ok {
		return b, nil
	}
	return false, &nonBooleanError{value: v, origin: expr}
}

func (e *evaluator) junction(expr *parser.Expression) (value.Value, error) {
	want := expr.Kind == "And"
	for index, operand := range expr.Args {
		b, err := e.condition(operand)
		if err != nil {
			return value.Value{}, booleanJunctionError(err, expr, index)
		}
		if b != want {
			return value.Bool(!want), nil
		}
	}
	return value.Bool(want), nil
}

func (e *evaluator) implies(expr *parser.Expression) (value.Value, error) {
	if len(expr.Args) != 2 {
		return value.Value{}, fmt.Errorf("=> expects 2 arguments")
	}
	left, err := e.condition(expr.Args[0])
	if err != nil {
		return value.Value{}, booleanOperatorError(err, expr, expr.Args[0], "=>", "P", nil)
	}
	if !left {
		return value.Bool(true), nil
	}
	right, err := e.condition(expr.Args[1])
	if err != nil {
		return value.Bool(right), booleanOperatorError(err, expr, expr.Args[1], "=>", "Q", nil)
	}
	return value.Bool(right), nil
}

func (e *evaluator) prime(expr *parser.Expression) (value.Value, error) {
	if e.context.Next == nil {
		return value.Value{}, fmt.Errorf("prime without a next state")
	}
	if name, ok := e.context.Lexical.EnabledVariable(expr); e.context.UncachedArguments && ok {
		if v, ok := e.context.Next[name]; ok {
			return v, nil
		}
		return value.Value{}, fmt.Errorf("unbound primed variable %s", name)
	}
	saved := *e
	e.context.Current = saved.context.Next
	e.context.Next = nil
	e.primeArguments = true
	if e.program.tlcRuntime != nil {
		e.primed = &tlcPrime{source: saved.context.Current}
	}
	result, err := e.eval(expr.Args[0])
	*e = saved
	return result, err
}

func (e *evaluator) unchanged(expr *parser.Expression) (value.Value, error) {
	current, err := e.eval(expr.Args[0])
	if err != nil {
		return value.Value{}, err
	}
	next, err := e.prime(expr)
	if err != nil {
		return value.Value{}, err
	}
	same, err := value.Equal(current, next)
	return value.Bool(same), err
}

func (e *evaluator) ifExpression(expr *parser.Expression) (value.Value, error) {
	b, err := e.condition(expr.Args[0])
	if err != nil {
		if failure, ok := errors.AsType[*nonBooleanError](err); ok && failure.origin == expr.Args[0] {
			message := fmt.Sprintf("A non-boolean expression (%s) was used as the condition of an IF.", failure.value.TLCKindDescription())
			return value.Value{}, booleanRuntimeError(message, expr, nil)
		}
		return value.Value{}, err
	}
	if b {
		return e.eval(expr.Args[1])
	}
	return e.eval(expr.Args[2])
}

func (e *evaluator) caseExpression(expr *parser.Expression) (value.Value, error) {
	for _, arm := range expr.Arms {
		b, err := e.condition(arm.Condition)
		if err != nil {
			return value.Value{}, booleanCaseError(err, arm.Condition)
		}
		if b {
			return e.eval(arm.Body)
		}
	}
	if len(expr.Args) > 0 {
		return e.eval(expr.Args[0])
	}
	if expr.Text == "!config-table!" {
		return value.Value{}, e.configTableError(expr)
	}
	return value.Value{}, fmt.Errorf("no CASE guard is true")
}

func (e *evaluator) let(expr *parser.Expression) (value.Value, error) {
	if len(expr.Instances) > 0 {
		return value.Value{}, fmt.Errorf("LET instances are unsupported")
	}
	defs := make(map[string]parser.Definition, len(expr.Definitions))
	for _, d := range expr.Definitions {
		defs[d.Name] = d
	}
	return e.evalInScope(&scope{parent: e.scope, definitions: defs}, expr.Args[0])
}

func (e *evaluator) set(expr *parser.Expression) (value.Value, error) {
	values, err := e.evalArgs(expr.Args)
	if err != nil {
		return value.Value{}, err
	}
	v, err := value.SetUnnormalizedWithSource(values, expr)
	return e.orderNewSet(v, values, err)
}

func (e *evaluator) tuple(args []*parser.Expression) (value.Value, error) {
	values, err := e.evalArgs(args)
	if err != nil {
		return value.Value{}, err
	}
	return value.Tuple(values)
}

func (e *evaluator) record(expr *parser.Expression) (value.Value, error) {
	pairs := make([]value.Pair, 0, len(expr.Fields))
	for _, field := range expr.Fields {
		v, err := e.eval(field.Value)
		if err != nil {
			return value.Value{}, err
		}
		pairs = append(pairs, value.Pair{Key: e.program.literal(field.Name), Value: v})
	}
	return e.orderedRecord(pairs)
}

func (e *evaluator) functionApplication(expr *parser.Expression) (value.Value, error) {
	fn, err := e.functionOperand(expr.Args[0])
	if err != nil {
		return value.Value{}, err
	}
	args, err := e.evalArgs(expr.Args[1:])
	if err != nil {
		return value.Value{}, err
	}
	if len(args) == 0 {
		return value.Value{}, fmt.Errorf("missing function argument")
	}
	key := args[0]
	if len(args) > 1 {
		key, err = value.Tuple(args)
		if err != nil {
			return value.Value{}, err
		}
	}
	return functionLookup(fn, key, e.keepLazy)
}

func (e *evaluator) field(expr *parser.Expression) (value.Value, error) {
	record, err := e.eval(expr.Args[0])
	if err != nil {
		return value.Value{}, err
	}
	return record.Lookup(e.program.literal(expr.Text))
}

func (e *evaluator) isTLCBuiltin(name string) bool {
	return e.tlcext && isTLCExtOperator(name) || e.tlc && isTLCOperator(name)
}

func (e *evaluator) callTLCBuiltin(expr *parser.Expression) (value.Value, error) {
	if e.tlcext && isTLCExtOperator(expr.Text) {
		return e.tlcExtOperator(expr)
	}
	return e.tlcOperator(expr)
}
