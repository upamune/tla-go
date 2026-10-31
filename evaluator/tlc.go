package evaluator

import (
	"errors"
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

var tlcArities = map[string]int{":>": 2, "@@": 2, "ToString": 1, "TLCEval": 1, "Permutations": 1, "Assert": 2, "SortSeq": 2, "Print": 2, "PrintT": 1, "TLCGet": 1, "TLCSet": 2, "RandomElement": 1}

func isTLCOperator(name string) bool { _, ok := tlcArities[name]; return ok }

func (e *evaluator) tlcOperator(expr *parser.Expression) (value.Value, error) {
	if len(expr.Args) != tlcArities[expr.Text] {
		return e.tlcArgumentMismatch(expr)
	}
	if expr.Text == "SortSeq" {
		return e.sortSequence(expr.Args)
	}
	if expr.Text == "TLCEval" && e.program.constantTLCEval[expr] {
		return e.constantTLCEvaluation(expr)
	}
	args, err := e.evalArgs(expr.Args)
	if err != nil {
		return value.Value{}, err
	}
	return e.callTLCValue(expr.Text, args)
}

func (e *evaluator) tlcArgumentMismatch(expr *parser.Expression) (value.Value, error) {
	for i, arg := range expr.Args {
		if expr.Text == "SortSeq" && i == len(expr.Args)-1 {
			if _, err := e.operatorArgument(arg, 2); err != nil {
				return value.Value{}, err
			}
		} else if _, err := e.eval(arg); err != nil {
			return value.Value{}, err
		}
	}
	message := fmt.Sprintf("array is not of length %d", tlcArities[expr.Text])
	if expr.Text == "Any" || expr.Text == "JavaTime" {
		message = "cannot convert MethodHandle()Value to (Value[])Value"
	}
	return value.Value{}, &RuntimeError{Message: message, Standard: true}
}

func (e *evaluator) callTLCValue(name string, args []value.Value) (value.Value, error) {
	if name == "RandomElement" {
		return e.randomElement(args[0])
	}
	if name == "TLCGet" || name == "TLCSet" {
		return e.tlcRuntimeOperator(name, args)
	}
	if name == "Print" || name == "PrintT" {
		return e.printTLC(name, args)
	}
	if name == "@@" {
		return e.program.combineFunctions(args[0], args[1])
	}
	return tlcValue(name, args)
}

func (e *evaluator) printTLC(name string, args []value.Value) (value.Value, error) {
	first, err := e.printedString(args[0])
	if err != nil {
		return value.Value{}, printFailure(name, err)
	}
	if name == "PrintT" {
		if e.context.OnPrint != nil {
			e.context.OnPrint(e.printTLayout(first))
		}
		return value.Bool(true), nil
	}
	second, err := e.printedString(args[1])
	if err != nil {
		return value.Value{}, printFailure(name, err)
	}
	if e.context.OnPrint != nil {
		e.context.OnPrint(value.PrettyPrint(first) + "  " + value.PrettyPrint(second))
	}
	return args[1], nil
}

func (e *evaluator) printedString(v value.Value) (string, error) {
	if e.program.printStyle.Terse {
		return v.TerseString()
	}
	return v.StrictString()
}

func (e *evaluator) printTLayout(text string) string {
	if e.program.printStyle.UnformattedPrintT {
		return text
	}
	return value.PrettyPrint(text)
}

func printFailure(name string, err error) error {
	failure, ok := errors.AsType[*value.UnexpectedError](err)
	if !ok {
		return &RuntimeError{Message: err.Error(), Standard: true}
	}
	wrapped := *failure
	signature := "tlc2.value.impl.Value"
	if name == "Print" {
		signature += ",tlc2.value.impl.Value"
	}
	wrapped.Message = fmt.Sprintf("Attempted to apply the operator overridden by the Java method\npublic static tlc2.value.impl.Value tlc2.module.TLC.%s(%s),\nbut it produced the following error:\n%s", name, signature, failure.Message)
	return &RuntimeError{Message: wrapped.Message, Unexpected: &wrapped}
}

func assert(args []value.Value) (value.Value, error) {
	ok, valid := args[0].Bool()
	if valid && ok {
		return value.Bool(true), nil
	}
	message := args[1].String()
	if formatted, err := args[1].StrictString(); err == nil {
		message = formatted
	}
	return value.Value{}, &RuntimeError{
		Message:     fmt.Sprintf("The first argument of Assert evaluated to FALSE; the second argument was:\n%s", value.PrettyPrint(message)),
		Legacy:      fmt.Sprintf("assertion failed: %s", args[1]),
		Standard:    true,
		ExitCode:    14,
		ToolCode:    2132,
		TrackSource: true,
	}
}

func tlcValue(name string, args []value.Value) (value.Value, error) {
	switch name {
	case ":>":
		return value.Function([]value.Pair{{Key: args[0], Value: args[1]}})
	case "ToString":
		text, err := args[0].StrictString()
		if err != nil {
			return value.Value{}, &RuntimeError{Message: err.Error(), Standard: true}
		}
		return value.String(text), nil
	case "TLCEval":
		return args[0], nil
	case "Assert":
		return assert(args)
	case "Permutations":
		return permutations(args[0])
	default:
		return value.Value{}, fmt.Errorf("unknown TLC operator %s", name)
	}
}

// combineFunctions computes f @@ g. Java TLC's CombineFcn collects the
// combined domain in a ValueVec.
func (p *Program) combineFunctions(left, right value.Value) (value.Value, error) {
	a, err := left.Pairs()
	if err != nil {
		return value.Value{}, err
	}
	b, err := right.Pairs()
	if err != nil {
		return value.Value{}, err
	}
	result, err := value.Function(append(a, b...))
	if err != nil || len(a)+len(b) <= p.setLimit() {
		return result, err
	}
	pairs, err := result.Pairs()
	if err != nil {
		return value.Value{}, err
	}
	return result, p.checkMethodSetSize(combineFcnSignature, len(pairs))
}

func permutations(set value.Value) (value.Value, error) {
	elements, err := set.Elements()
	if err != nil {
		return value.Value{}, err
	}
	var functions []value.Value
	if err = permutationVisit(elements, append([]value.Value(nil), elements...), 0, &functions); err != nil {
		return value.Value{}, err
	}
	return value.Set(functions)
}

func permutationVisit(domain, selected []value.Value, index int, result *[]value.Value) error {
	if index == len(selected) {
		pairs := make([]value.Pair, len(domain))
		for i, key := range domain {
			pairs[i] = value.Pair{Key: key, Value: selected[i]}
		}
		f, err := value.Function(pairs)
		if err != nil {
			return err
		}
		*result = append(*result, f)
		return nil
	}
	for i := index; i < len(selected); i++ {
		selected[index], selected[i] = selected[i], selected[index]
		err := permutationVisit(domain, selected, index+1, result)
		selected[index], selected[i] = selected[i], selected[index]
		if err != nil {
			return err
		}
	}
	return nil
}
