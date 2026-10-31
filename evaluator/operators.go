package evaluator

import (
	"fmt"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// Operators are separate from TLA+ values; closures retain their lexical scope.
type operatorRef struct {
	name           string
	arity          int
	definition     *parser.Definition
	lambda         *parser.Expression
	scope          *scope
	enabledScopes  []string
	scopesCaptured bool
}

func (e *evaluator) findOperator(name string) (operatorRef, bool) {
	return findScopedOperator(e.scope, name)
}

func findScopedOperator(start *scope, name string) (operatorRef, bool) {
	for s := start; s != nil; s = s.parent {
		if op, ok := s.operators[name]; ok {
			return op, true
		}
		if s.hasValue(name) {
			return operatorRef{}, false
		}
		if _, ok := s.definitions[name]; ok {
			return operatorRef{}, false
		}
	}
	return operatorRef{}, false
}

func (e *evaluator) applyDefinition(d parser.Definition, arguments []*parser.Expression, defined *scope) (value.Value, error) {
	if len(d.Params) != len(arguments) {
		return value.Value{}, fmt.Errorf("operator %s expects %d arguments", d.Name, len(d.Params))
	}
	if d.ConfigConstant {
		return e.evalInScope(defined, d.Body)
	}
	if !hasOperatorParameters(d.Params) {
		if d.Kind == "Operator" {
			return e.operatorDefinition(d, placeholderArguments(len(arguments)), nil, defined, arguments, e.scope)
		}
		values, err := e.evalArgs(arguments)
		if err != nil {
			return value.Value{}, err
		}
		return e.definition(d, values, defined)
	}
	return e.applyHigherOrderDefinition(d, arguments, defined)
}

func (e *evaluator) applyHigherOrderDefinition(d parser.Definition, arguments []*parser.Expression, defined *scope) (value.Value, error) {
	values := make([]value.Value, len(arguments))
	operators := make(map[string]operatorRef)
	for i, param := range d.Params {
		v, op, err := e.definitionArgument(param, arguments[i], d.Kind == "Operator")
		if err != nil {
			return value.Value{}, err
		}
		values[i] = v
		if param.Arity > 0 {
			operators[param.Name] = op
		}
	}
	if d.Kind == "Function" {
		return e.functionDefinition(d, values, operators, defined)
	}
	return e.operatorDefinition(d, values, operators, defined, arguments, e.scope)
}

func hasOperatorParameters(params []parser.Declaration) bool {
	for _, param := range params {
		if param.Arity > 0 {
			return true
		}
	}
	return false
}

func (e *evaluator) definitionArgument(param parser.Declaration, expr *parser.Expression, lazy bool) (value.Value, operatorRef, error) {
	if param.Arity > 0 {
		op, err := e.operatorArgument(expr, param.Arity)
		if err == nil && !op.scopesCaptured {
			op.enabledScopes = capturedOperatorScopes(e.activeEnabledScopes(), expr.InstanceScopes)
			op.scopesCaptured = true
		}
		return value.Bool(false), op, err
	}
	if lazy {
		return value.Bool(false), operatorRef{}, nil
	}
	v, err := e.eval(expr)
	return v, operatorRef{}, err
}

func capturedOperatorScopes(active, introduced []string) []string {
	if len(active) == 0 {
		return introduced
	}
	if len(introduced) == 0 {
		return active
	}
	return append(append([]string(nil), active...), introduced...)
}

func valueBindings(params []parser.Declaration, args []value.Value, operators map[string]operatorRef) ([]binding, error) {
	bindings := make([]binding, 0, len(args))
	for i, param := range params {
		if param.Arity == 0 {
			bindings = append(bindings, binding{name: param.Name, value: args[i]})
			continue
		}
		op, ok := operators[param.Name]
		if !ok || op.arity != param.Arity {
			return nil, fmt.Errorf("invalid operator argument %s", param.Name)
		}
	}
	return bindings, nil
}

// operatorScope binds value parameters. With expressions, each one is an
// argument evaluated on demand in the caller's scope.
func (e *evaluator) operatorScope(defined *scope, params []parser.Declaration, args []value.Value, operators map[string]operatorRef, expressions []*parser.Expression, caller *scope, enabledScopes []string) (*scope, error) {
	if len(params) == 1 && params[0].Arity == 0 && len(expressions) == 1 {
		return e.frames.lazyScope(defined, params[0].Name, args[0], operators, expressions[0], caller, enabledScopes), nil
	}
	bindings, err := valueBindings(params, args, operators)
	if err != nil {
		return nil, err
	}
	if len(expressions) > 0 {
		bindLazyArguments(bindings, params, expressions, caller, enabledScopes)
	}
	return &scope{parent: defined, bound: bindings, operators: operators}, nil
}

// operatorCallKey identifies operator arguments by scope address. The key
// outlives the call, so those scopes must not be lent again.
func (e *evaluator) operatorCallKey(d parser.Definition, args []value.Value, ops map[string]operatorRef, defined *scope) (string, error) {
	key, err := e.callKey(d, args, defined)
	if err != nil {
		return "", err
	}
	for _, param := range d.Params {
		if param.Arity > 0 {
			pin(ops[param.Name].scope)
		}
	}
	return appendOperatorBindings(key, d, ops), nil
}

func appendOperatorBindings(key string, d parser.Definition, ops map[string]operatorRef) string {
	var b strings.Builder
	b.WriteString(key)
	for _, param := range d.Params {
		if param.Arity == 0 {
			continue
		}
		op := ops[param.Name]
		appendKey(&b, fmt.Sprintf("%p:%p:%s", op.scope, op.lambda, op.name))
	}
	return b.String()
}

func (e *evaluator) operatorArgument(expr *parser.Expression, arity int) (operatorRef, error) {
	if expr.Kind == "Lambda" {
		if len(expr.Names) != arity {
			return operatorRef{}, fmt.Errorf("lambda expects %d arguments", len(expr.Names))
		}
		return operatorRef{arity: arity, lambda: expr, scope: e.scope}, nil
	}
	if expr.Kind != "Identifier" && expr.Kind != "OperatorReference" {
		return operatorRef{}, fmt.Errorf("expected an operator argument at line %d, column %d", expr.Location.Line, expr.Location.Column)
	}
	return e.namedOperator(expr.Text, arity)
}

func (e *evaluator) namedOperator(name string, arity int) (operatorRef, error) {
	if op, ok := e.findOperator(name); ok {
		if op.arity != arity {
			return operatorRef{}, fmt.Errorf("operator %s has wrong arity", name)
		}
		return op, nil
	}
	if def, scope, ok := e.findDefinition(name); ok {
		return definitionOperator(def, scope, arity)
	}
	if expected, ok := parser.SpecialOperatorArity(name); ok && arity == expected {
		return operatorRef{name: name, arity: arity}, nil
	}
	if !e.supportsBuiltin(name, arity) {
		return operatorRef{}, fmt.Errorf("unknown operator %s with arity %d", name, arity)
	}
	return operatorRef{name: name, arity: arity}, nil
}

func definitionOperator(def parser.Definition, defined *scope, arity int) (operatorRef, error) {
	if def.Kind != "Operator" || len(def.Params) != arity {
		return operatorRef{}, fmt.Errorf("operator %s has wrong arity", def.Name)
	}
	for _, param := range def.Params {
		if param.Arity != 0 {
			return operatorRef{}, fmt.Errorf("higher-order operator %s cannot be passed as an argument", def.Name)
		}
	}
	return operatorRef{name: def.Name, arity: arity, definition: &def, scope: defined}, nil
}

func (e *evaluator) invokeOperator(op operatorRef, args []value.Value) (value.Value, error) {
	if len(args) != op.arity {
		return value.Value{}, fmt.Errorf("operator expects %d arguments", op.arity)
	}
	if op.definition != nil {
		return e.definition(*op.definition, args, op.scope)
	}
	if op.lambda != nil {
		return e.invokeLambda(op, args)
	}
	result, err := e.invokeBuiltin(op.name, args)
	if err != nil {
		return result, err
	}
	return e.orderSetResult(op.name, result)
}

func (e *evaluator) invokeLambda(op operatorRef, args []value.Value) (value.Value, error) {
	if e.depth >= 4096 {
		return value.Value{}, fmt.Errorf("evaluation recursion limit exceeded")
	}
	return e.evalLambda(op, valueScope(op.scope, op.lambda.Names, args))
}

func (e *evaluator) invokeLambdaExpressions(op operatorRef, expressions []*parser.Expression) (value.Value, error) {
	if len(expressions) != op.arity {
		return value.Value{}, fmt.Errorf("operator expects %d arguments", op.arity)
	}
	if e.depth >= 4096 {
		return value.Value{}, fmt.Errorf("evaluation recursion limit exceeded")
	}
	return e.evalLambda(op, &scope{parent: op.scope, bound: lazyNameBindings(op.lambda.Names, expressions, e.scope, e.activeEnabledScopes())})
}

func (e *evaluator) evalLambda(op operatorRef, s *scope) (value.Value, error) {
	saved := *e
	e.depth++
	e.scope = s
	result, err := e.eval(op.lambda.Args[0])
	*e = saved
	return result, err
}
