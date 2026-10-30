package checker

import (
	"strings"
	"testing"
)

func TestTemporalEvaluationFailureMessage(t *testing.T) {
	for _, init := range []string{"[](x = 0)", "x = 0"} {
		source := "---- MODULE M ----\nVARIABLE x\nInit == " + init + "\nNext == [](x = 0)\n====\n"
		result, err := Check(source, "INIT Init NEXT Next")
		if err != nil || result.Status != EvaluationFailure || result.Failure == nil || !strings.Contains(result.Failure.Message, "temporal formula ([]A)") {
			t.Fatalf("init %s: result %+v, error %v", init, result, err)
		}
	}
}
