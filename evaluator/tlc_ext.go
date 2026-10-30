package evaluator

import (
	"errors"
	"fmt"
	"sort"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

var tlcExtArities = map[string]int{"TLCDefer": 1, "TLCNoOp": 1, "TLCModelValue": 1, "AssertEq": 2, "AssertError": 2, "TLCGetOrDefault": 2, "TLCGetAndSet": 4, "ToTrace": 1}

func isTLCExtOperator(name string) bool { _, ok := tlcExtArities[name]; return ok }

func (e *evaluator) tlcExtOperator(expr *parser.Expression) (value.Value, error) {
	if len(expr.Args) != tlcExtArities[expr.Text] {
		return value.Value{}, fmt.Errorf("%s expects %d arguments", expr.Text, tlcExtArities[expr.Text])
	}
	switch expr.Text {
	case "TLCDefer":
		return value.Bool(true), nil
	case "AssertError":
		return e.tlcExtAssertError(expr.Args)
	case "TLCGetAndSet":
		return e.tlcGetAndSet(expr.Args)
	}
	args, err := e.evalArgs(expr.Args)
	if err != nil {
		return value.Value{}, err
	}
	return e.tlcExtValue(expr.Text, args)
}

func (e *evaluator) tlcExtValue(name string, args []value.Value) (value.Value, error) {
	switch name {
	case "TLCNoOp":
		return args[0], nil
	case "TLCModelValue":
		return tlcModelValue(args[0])
	case "AssertEq":
		return e.tlcAssertEq(args[0], args[1])
	case "TLCGetOrDefault":
		return e.tlcGetOrDefault(args[0], args[1])
	case "ToTrace":
		return tlcToTrace(args[0])
	}
	return value.Value{}, fmt.Errorf("unknown TLCExt operator %s", name)
}

func tlcModelValue(v value.Value) (value.Value, error) {
	s, ok := v.Text()
	if !ok || v.Kind() != value.StringKind {
		return value.Value{}, &RuntimeError{Message: fmt.Sprintf("The argument of ModelValue should be a string, but instead it is:\n%s", v), Standard: true}
	}
	return value.Model(s), nil
}

func (e *evaluator) tlcAssertEq(left, right value.Value) (value.Value, error) {
	equal, err := value.Equal(left, right)
	if err != nil {
		return value.Value{}, err
	}
	if !equal && e.context.OnPrint != nil {
		for _, v := range []value.Value{value.String("Assertion failed"), left, right} {
			s, err := v.StrictString()
			if err != nil {
				return value.Value{}, err
			}
			e.context.OnPrint(value.PrettyPrint(s))
		}
	}
	return value.Bool(equal), nil
}

func (e *evaluator) tlcExtAssertError(args []*parser.Expression) (value.Value, error) {
	if args[0].Kind != "String" {
		pos := args[0].Location.Position
		return value.Value{}, &RuntimeError{Message: fmt.Sprintf("In computing AssertError, a non-string expression (line %d, col %d) was used as the err of an AssertError(err, exp).", pos.Line, pos.Column), Standard: true}
	}
	expected := args[0].Text
	_, err := e.eval(args[1])
	if err == nil {
		return value.Bool(false), nil
	}
	if failure, ok := errors.AsType[*RuntimeError](err); ok {
		return value.Bool(failure.Message == expected), nil
	}
	if tlaError, ok := errors.AsType[*value.TLCError](err); ok {
		return value.Bool(tlaError.Error() == expected), nil
	}
	if failure, ok := errors.AsType[*value.UnexpectedError](err); ok {
		return value.Bool(failure.Message == expected), nil
	}
	return value.Value{}, err
}

func (e *evaluator) tlcGetOrDefault(key, fallback value.Value) (value.Value, error) {
	if runtime, ok := e.program.tlcRuntime.(interface {
		GetRegister(value.Value) (value.Value, bool)
	}); ok {
		if v, found := runtime.GetRegister(key); found {
			return v, nil
		}
	}
	return fallback, nil
}

func tlcToTrace(counter value.Value) (value.Value, error) {
	domain, err := counter.FunctionDomain()
	expected, _ := value.Set([]value.Value{value.String("action"), value.String("state")})
	if err != nil {
		return invalidCounterExample(counter)
	}
	matches, err := value.Equal(domain, expected)
	if err != nil || !matches {
		return invalidCounterExample(counter)
	}
	states, err := counter.Lookup(value.String("state"))
	if err != nil {
		return value.Value{}, err
	}
	entries, err := states.Elements()
	if err != nil {
		return value.Value{}, err
	}
	return tlcTraceEntries(entries)
}

func invalidCounterExample(counter value.Value) (value.Value, error) {
	return value.Value{}, &RuntimeError{Message: fmt.Sprintf("The argument of ToTrace should be a CounterExample, but instead it is:\n%s", counter), Standard: true}
}

type traceEntry struct {
	index int32
	state value.Value
}

func tlcTraceEntries(entries []value.Value) (value.Value, error) {
	ordered := make([]traceEntry, 0, len(entries))
	for _, entry := range entries {
		pair, err := entry.TupleElements()
		if err != nil || len(pair) != 2 {
			return value.Value{}, fmt.Errorf("invalid CounterExample state entry")
		}
		n, ok := pair[0].Int()
		if !ok {
			return value.Value{}, fmt.Errorf("invalid CounterExample state index")
		}
		ordered = append(ordered, traceEntry{n, pair[1]})
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].index < ordered[j].index })
	result := make([]value.Value, len(ordered))
	for i := range ordered {
		result[i] = ordered[i].state
	}
	return value.Tuple(result)
}

func (e *evaluator) tlcGetAndSet(args []*parser.Expression) (value.Value, error) {
	key, err := e.eval(args[0])
	if err != nil {
		return value.Value{}, err
	}
	op, err := e.operatorArgument(args[1], 2)
	if err != nil {
		return value.Value{}, err
	}
	values, err := e.evalArgs(args[2:])
	if err != nil {
		return value.Value{}, err
	}
	previous, err := e.tlcGetOrDefault(key, values[1])
	if err != nil {
		return value.Value{}, err
	}
	updated, err := e.invokeOperator(op, []value.Value{previous, values[0]})
	if err != nil {
		return value.Value{}, err
	}
	if err := e.setTLCExtRegister(key, updated); err != nil {
		return value.Value{}, err
	}
	return previous, nil
}

func (e *evaluator) setTLCExtRegister(key, v value.Value) error {
	if e.program.tlcRuntime == nil {
		return fmt.Errorf("TLCGetAndSet requires a TLC runtime")
	}
	return e.program.tlcRuntime.Set(key, v, e.context)
}

func (e *evaluator) tlcExtTrace() (value.Value, error) {
	runtime, ok := e.program.tlcRuntime.(interface {
		Trace(State) (value.Value, error)
	})
	if !ok {
		return value.Value{}, fmt.Errorf("Trace requires a TLC runtime")
	}
	return runtime.Trace(e.context.Current)
}

func (e *evaluator) tlcExtCounterExample() (value.Value, error) {
	runtime, ok := e.program.tlcRuntime.(interface{ CounterExample() (value.Value, error) })
	if !ok {
		return value.Value{}, fmt.Errorf("CounterExample requires a TLC runtime")
	}
	return runtime.CounterExample()
}

func isTLCExtStateOperator(name string) bool { return name == "Trace" || name == "CounterExample" }
func (e *evaluator) tlcExtStateOperator(name string) (value.Value, error) {
	if name == "Trace" {
		return e.tlcExtTrace()
	}
	return e.tlcExtCounterExample()
}
