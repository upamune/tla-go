package evaluator

import (
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (e *evaluator) constantDefinition(d parser.Definition, args []value.Value, operators map[string]operatorRef, defined *scope) (value.Value, error) {
	if cached, ok := e.program.constantCache[d.Name]; ok {
		return cached, nil
	}
	evaluate := func() (value.Value, error) {
		return e.evaluateOperatorDefinition(d, args, operators, defined, nil, nil)
	}
	var result value.Value
	var err error
	if runtime, ok := e.program.tlcRuntime.(interface {
		EvaluateAtStartup(func() (value.Value, error)) (value.Value, error)
	}); ok {
		result, err = runtime.EvaluateAtStartup(evaluate)
	} else {
		result, err = evaluate()
	}
	if err == nil {
		e.program.constantCache[d.Name] = result
	}
	return result, err
}

func (e *evaluator) constantTLCEvaluation(expr *parser.Expression) (value.Value, error) {
	if cached, ok := e.program.tlcEvalCache[expr]; ok {
		return cached, nil
	}
	result, err := e.eval(expr.Args[0])
	if err == nil {
		e.program.tlcEvalCache[expr] = result
	}
	return result, err
}
