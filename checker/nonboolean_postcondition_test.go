package checker

import "testing"

func TestNonBooleanPostconditionSource(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS TLC
VARIABLE x
Init == x=0
Next == UNCHANGED x
Post == TLCGet("level")
====`
	result, err := Check(source, "INIT Init NEXT Next POSTCONDITION Post")
	want := "TLC expected a boolean value, but did not find one. line 6, col 9 to line 6, col 23 of module M"
	if err != nil || result.Status != PostconditionViolation || result.Violation == nil || result.Violation.Message != want {
		t.Fatalf("result %+v, error %v; want %q", result, err, want)
	}
}
