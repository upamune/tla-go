package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/value"
)

// TLCRuntime supplies per-check state to TLCGet and TLCSet.
type TLCRuntime interface {
	Get(key value.Value, context Context, source State, primed bool) (value.Value, error)
	Set(key, val value.Value, context Context) error
}

type tlcPrime struct{ source State }

func (e *evaluator) tlcRuntimeOperator(name string, args []value.Value) (value.Value, error) {
	runtime := e.program.tlcRuntime
	if runtime == nil {
		return value.Value{}, fmt.Errorf("%s requires a TLC runtime", name)
	}
	if name == "TLCGet" {
		var source State
		if e.primed != nil {
			source = e.primed.source
		}
		return runtime.Get(args[0], e.context, source, e.primed != nil)
	}
	if err := runtime.Set(args[0], args[1], e.context); err != nil {
		return value.Value{}, err
	}
	return value.Bool(true), nil
}
