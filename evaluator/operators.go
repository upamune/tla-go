package evaluator

import (
	"fmt"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// Operators are separate from TLA+ values; closures retain their lexical scope.
type operatorRef struct {
	name       string
	arity      int
	definition *parser.Definition
	lambda     *parser.Expression
	scope      *scope
}

func (e *evaluator) findOperator(name string) (operatorRef, bool) {
	for s := e.scope; s != nil; s = s.parent {
		if op, ok := s.operators[name]; ok {
			return op, true
		}
		if _, ok := s.values[name]; ok {
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
		child := *e
		child.scope = defined
		return child.eval(d.Body)
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
		return value.Bool(false), op, err
	}
	if lazy {
		return value.Bool(false), operatorRef{}, nil
	}
	v, err := e.eval(expr)
	return v, operatorRef{}, err
}

func valueBindings(params []parser.Declaration, args []value.Value, operators map[string]operatorRef) (map[string]value.Value, error) {
	bindings := make(map[string]value.Value, len(args))
	for i, param := range params {
		if param.Arity == 0 {
			bindings[param.Name] = args[i]
			continue
		}
		op, ok := operators[param.Name]
		if !ok || op.arity != param.Arity {
			return nil, fmt.Errorf("invalid operator argument %s", param.Name)
		}
	}
	return bindings, nil
}

func (e *evaluator) operatorCallKey(d parser.Definition, args []value.Value, ops map[string]operatorRef, defined *scope) (string, error) {
	key, err := e.callKey(d, args, defined)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(key)
	for _, param := range d.Params {
		if param.Arity == 0 {
			continue
		}
		op := ops[param.Name]
		appendKey(&b, fmt.Sprintf("%p:%p:%s", op.scope, op.lambda, op.name))
	}
	return b.String(), nil
}

func (e *evaluator) operatorArgument(expr *parser.Expression, arity int) (operatorRef, error) {
	if expr.Kind == "Lambda" {
		if len(expr.Names) != arity {
			return operatorRef{}, fmt.Errorf("lambda expects %d arguments", len(expr.Names))
		}
		return operatorRef{arity: arity, lambda: expr, scope: e.scope}, nil
	}
	if expr.Kind != "Identifier" && expr.Kind != "OperatorReference" {
		return operatorRef{}, fmt.Errorf("expected an operator argument")
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
	values := make(map[string]value.Value, len(args))
	for i, name := range op.lambda.Names {
		values[name] = args[i]
	}
	child := *e
	child.depth++
	child.scope = &scope{parent: op.scope, values: values}
	return child.eval(op.lambda.Args[0])
}

func (e *evaluator) invokeLambdaExpressions(op operatorRef, expressions []*parser.Expression) (value.Value, error) {
	if len(expressions) != op.arity {
		return value.Value{}, fmt.Errorf("operator expects %d arguments", op.arity)
	}
	if e.depth >= 4096 {
		return value.Value{}, fmt.Errorf("evaluation recursion limit exceeded")
	}
	params := make([]parser.Declaration, len(op.lambda.Names))
	values := make(map[string]value.Value, len(params))
	for i, name := range op.lambda.Names {
		params[i].Name = name
		values[name] = value.Bool(false)
	}
	child := *e
	child.depth++
	child.scope = &scope{parent: op.scope, values: values, primeArgs: primeArgumentBindings(params, expressions, e.scope)}
	return child.eval(op.lambda.Args[0])
}
