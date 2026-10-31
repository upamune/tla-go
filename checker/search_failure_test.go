package checker

import "testing"

// Counts and exit codes come from checkSequentially at the pinned tla-ts commit.
func TestRuntimeEvaluationFailures(t *testing.T) {
	for _, tc := range []struct {
		name, definitions, message                      string
		generated, distinct, queued, depth, trace, exit int
		degrees                                         bool
	}{
		{"initial invariant", "Init == x = 0\nNext == x' = x\nInv == 1 \\div 0 = x", "The second argument of \\div is 0.", 1, 1, 1, 1, 0, 75, false},
		{"constant invariant", "Init == x = 0\nNext == x' = x\nInv == 1 \\div 0 = 0", "The second argument of \\div is 0.", 1, 1, 1, 1, 0, 75, false},
		{"later invariant", "Init == x = 0\nNext == x' = x + 1\nInv == 1 \\div (1 - x) >= 0", "Evaluating invariant Inv failed.\nThe second argument of \\div is 0.", 2, 2, 0, 2, 2, 76, true},
		{"initial generation", "Init == x = 1 \\div 0\nNext == x' = x\nInv == TRUE", "The second argument of \\div is 0.", 0, 0, 0, 0, 0, 75, false},
		{"next generation", "Init == x = 0\nNext == x' = 1 \\div 0\nInv == TRUE", "The second argument of \\div is 0.", 1, 1, 0, 1, 1, 255, false},
		{"initial assert", "Init == x = 0 /\\ Assert(FALSE, \"oops\")\nNext == x' = x\nInv == TRUE", "The first argument of Assert evaluated to FALSE; the second argument was:\n\"oops\"", 0, 0, 0, 0, 0, 75, false},
		{"constant assert invariant", "Init == x = 0\nNext == x' = x\nInv == Assert(FALSE, \"oops\")", "The first argument of Assert evaluated to FALSE; the second argument was:\n\"oops\"", 1, 1, 1, 1, 0, 75, false},
		{"next assert", "Init == x = 0\nNext == Assert(x < 0, \"oops\") /\\ x' = x\nInv == TRUE", "The first argument of Assert evaluated to FALSE; the second argument was:\n\"oops\"", 1, 1, 0, 1, 1, 14, false},
		{"invariant assert", "Init == x = 0\nNext == x' = x + 1\nInv == Assert(x < 1, \"oops\")", "Evaluating invariant Inv failed.\nThe first argument of Assert evaluated to FALSE; the second argument was:\n\"oops\"", 2, 2, 0, 2, 2, 76, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\n" + tc.definitions + "\n===="
			result, err := Check(source, "INIT Init NEXT Next INVARIANT Inv")
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != EvaluationFailure || result.Failure == nil {
				t.Fatalf("got %+v", result)
			}
			f := result.Failure
			if f.Message != tc.message || f.ExitCode != tc.exit || len(f.Trace) != tc.trace || result.GeneratedStates != tc.generated || result.DistinctStates != tc.distinct || result.QueuedStates != tc.queued || result.Depth != tc.depth || result.HasOutDegrees != tc.degrees {
				t.Fatalf("got %+v failure=%+v", result, f)
			}
			if tc.degrees && result.OutDegrees[1] != 0 {
				t.Fatalf("outdegrees: %v", result.OutDegrees)
			}
		})
	}
}

func TestUnknownUnsupportedModelStillReturnsError(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = Unknown(x)\n===="
	if _, err := Check(source, "INIT Init NEXT Next"); err == nil {
		t.Fatal("expected unresolved symbol error")
	}
}

func TestActionPropertyEvaluationFailure(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\nProp == [][1 \\div (1 - x) >= 0]_x\n===="
	result, err := Check(source, "INIT Init NEXT Next PROPERTY Prop")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != EvaluationFailure || result.Failure == nil {
		t.Fatalf("got %+v", result)
	}
	want := "Evaluating action property Prop failed.\nThe second argument of \\div is 0."
	if result.Failure.Message != want || result.Failure.ExitCode != 255 || len(result.Failure.Trace) != 3 || result.GeneratedStates != 3 || result.DistinctStates != 3 || result.QueuedStates != 0 || result.Depth != 3 || result.HasOutDegrees {
		t.Fatalf("got %+v failure=%+v", result, result.Failure)
	}
}

func TestFunctionApplicationFailures(t *testing.T) {
	for _, tc := range []struct{ expression, message string }{
		{"<<1>>[2]", "Attempted to apply tuple\n<<1>>\nto integer 2 which is out of domain."},
		{"[i \\in {1, 2} |-> i][0]", "In applying the function\n<<1, 2>>,\nthe first argument is:\n0\nwhich is not in its domain.\n"},
		{"1[1]", unexpectedExceptionMessage(runtimeExceptionClass, "A non-function (an integer) was applied as a function.\nline 4, col 14 to line 4, col 17 of module M")},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = " + tc.expression + "\n===="
			result, err := Check(source, "INIT Init NEXT Next")
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != EvaluationFailure || result.Failure == nil || result.Failure.Message != tc.message || result.Failure.ExitCode != 75 || len(result.Failure.Trace) != 1 {
				t.Fatalf("got %+v failure=%+v", result, result.Failure)
			}
		})
	}
}

func TestInitialCoreFailureIncludesState(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = x\nInv == <<1>>[2] = x\n===="
	result, err := Check(source, "INIT Init NEXT Next INVARIANT Inv")
	if err != nil {
		t.Fatal(err)
	}
	if result.Failure == nil || result.Failure.ExitCode != 75 || result.Failure.InitialState == nil || result.Failure.InitialState["x"].String() != "0" || result.QueuedStates != 1 {
		t.Fatalf("got %+v failure=%+v", result, result.Failure)
	}
}
