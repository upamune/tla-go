package checker

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamGeneralTemporalMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-general-ltl-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var tc struct {
			Name, Source, Config, Status, Error string
			Generated, Distinct, Depth          int
		}
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			got, err := Check(tc.Source, tc.Config)
			if tc.Error != "" {
				if err == nil {
					t.Fatalf("got %+v; want an error like %q", got, tc.Error)
				}
				return
			}
			if err != nil || string(got.Status) != tc.Status || got.GeneratedStates != tc.Generated || got.DistinctStates != tc.Distinct || got.Depth != tc.Depth {
				t.Fatalf("got %+v, %v; want %s %d %d %d", got, err, tc.Status, tc.Generated, tc.Distinct, tc.Depth)
			}
			if got.Status == LivenessViolation && (got.Violation == nil || got.Violation.LoopStart == nil || *got.Violation.LoopStart >= len(got.Violation.Trace)) {
				t.Fatalf("invalid lasso %+v", got.Violation)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 1536 {
		t.Fatalf("got %d cases, want 1536", count)
	}
}

func TestGeneralTemporalStateFormulas(t *testing.T) {
	for _, tc := range []struct {
		formula string
		status  Status
	}{
		{"[]<>[]TRUE", Success},
		{"<>[]<>(x = 0)", LivenessViolation},
		{"<>([](x = 1) \\/ [](x = 2))", LivenessViolation},
		{"[]<>(x = 0) \\/ []<>(x = 1)", LivenessViolation},
		{"~([]<>(x = 0) /\\ []<>(x = 1))", LivenessViolation},
		{"[](x = 0 => <>(x = 1 /\\ [](x = 0)))", LivenessViolation},
		{"[](\\E i \\in {0,1} : <>[](x=i))", LivenessViolation},
		{"<>(\\A i \\in {0,1} : []<>(x=i))", LivenessViolation},
		{"[](IF x=0 THEN <>[](x=1) ELSE []<>(x=2))", LivenessViolation},
		{"[]<>(\\A i \\in 0..1 : (x=i) \\/ <>(x=i))", LivenessViolation},
	} {
		t.Run(tc.formula, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nP == " + tc.formula + "\n===="
			got, err := Check(source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE")
			if err != nil || got.Status != tc.status || got.GeneratedStates != 4 || got.DistinctStates != 3 {
				t.Fatalf("got %+v, %v; want %s 4 3", got, err, tc.status)
			}
			if tc.status == LivenessViolation && (got.Violation == nil || got.Violation.LoopStart == nil || *got.Violation.LoopStart >= len(got.Violation.Trace)) {
				t.Fatalf("invalid lasso %+v", got.Violation)
			}
		})
	}
}

func TestGeneralTemporalActionFormulas(t *testing.T) {
	for _, formula := range []string{
		"[]<>(<<x' = (x + 1) % 3>>_x) \\/ []<>(x = 0)",
		"~([]<><<x' = 1>>_x \\/ <>[](x = 0))",
	} {
		source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nP == " + formula + "\n===="
		got, err := Check(source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE")
		if err != nil || got.Status != LivenessViolation || got.GeneratedStates != 4 || got.DistinctStates != 3 {
			t.Fatalf("%s: got %+v, %v", formula, got, err)
		}
		if got.Violation == nil || got.Violation.LoopStart == nil || *got.Violation.LoopStart >= len(got.Violation.Trace) {
			t.Fatalf("%s: invalid lasso %+v", formula, got.Violation)
		}
	}
}

func TestGeneralTemporalWithView(t *testing.T) {
	for _, tc := range []struct {
		view, formula       string
		status              Status
		generated, distinct int
	}{
		{"x", "[]<>[](x=0)", LivenessViolation, 4, 3},
	} {
		source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit==x=0\nNext==x'=(x+1)%3\nV==" + tc.view + "\nP==" + tc.formula + "\n===="
		got, err := Check(source, "INIT Init NEXT Next VIEW V PROPERTY P CHECK_DEADLOCK FALSE")
		if err != nil || got.Status != tc.status || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct {
			t.Fatalf("view %s, formula %s: got %+v, %v; want %s %d %d", tc.view, tc.formula, got, err, tc.status, tc.generated, tc.distinct)
		}
	}
}

func TestGeneralTemporalWithCollapsedView(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit==x=0\nNext==x'=(x+1)%3\nV==0\nP==[]<>[](x=0)\n===="
	got, err := Check(source, "INIT Init NEXT Next VIEW V PROPERTY P CHECK_DEADLOCK FALSE")
	if err != nil || got.Status != LivenessViolation || got.Violation == nil || got.Violation.Name != "" || len(got.Violation.Properties) != 0 || len(got.Violation.Trace) != 1 || got.Violation.LoopStart == nil || *got.Violation.LoopStart != 0 {
		t.Fatalf("got %+v, %v; want an unnamed one-state upstream lasso", got, err)
	}
}

func TestFalseConstantTemporalWithCollapsedView(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit==x=0\nNext==x'=(x+1)%3\nV==0\nP==[]<>[]FALSE\n===="
	got, err := Check(source, "INIT Init NEXT Next VIEW V PROPERTY P CHECK_DEADLOCK FALSE")
	if err != nil || got.Status != LivenessViolation || got.Violation == nil || got.Violation.Name != "P" || len(got.Violation.Trace) != 1 || got.Violation.LoopStart == nil || *got.Violation.LoopStart != 0 {
		t.Fatalf("got %+v, %v; want an upstream one-state lasso", got, err)
	}
}

func TestUpstreamConstantTemporalCollapsedViews(t *testing.T) {
	f, err := os.Open("testdata/upstream-constant-view-ltl.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	checked := 0
	for scanner.Scan() {
		var tc struct {
			Name, Source, Config, Status string
			Generated, Distinct, Depth   int
		}
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		if tc.Status != string(Success) {
			continue
		}
		checked++
		t.Run(tc.Name, func(t *testing.T) {
			got, err := Check(tc.Source, tc.Config)
			if err != nil || got.Status != Success || got.GeneratedStates != tc.Generated || got.DistinctStates != tc.Distinct || got.Depth != tc.Depth {
				t.Fatalf("got %+v, %v; want success %d %d %d", got, err, tc.Generated, tc.Distinct, tc.Depth)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if checked != 192 {
		t.Fatalf("got %d successful cases, want 192", checked)
	}
}

func TestConstantTemporalCollapsedViewStillRejectsOtherCases(t *testing.T) {
	for _, tc := range []struct{ spec, property string }{
		{"Init /\\ [][Next]_x /\\ WF_x(Next)", "[]<>[]TRUE"},
		{"Init /\\ [][Next]_x", "[]<>[]PrintT(TRUE)"},
		{"Init /\\ [][Next]_x", "WF_x(x'=1) => []<>(x=1)"},
	} {
		source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit==x=0\nNext==x'=(x+1)%3\nV==0\nSpec==" + tc.spec + "\nP==" + tc.property + "\n===="
		_, err := Check(source, "SPECIFICATION Spec VIEW V PROPERTY P CHECK_DEADLOCK FALSE")
		if err == nil {
			t.Fatalf("expected an error for %s with %s", tc.property, tc.spec)
		}
	}
}

func TestConstantTemporalWithSymmetry(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS TLC\nCONSTANT P\nVARIABLE x\nInit == x \\in P\nNext == x' \\in P\nSym == Permutations(P)\nProp == []<>[]TRUE\n===="
	config := "CONSTANT P = {p1,p2} INIT Init NEXT Next SYMMETRY Sym PROPERTY Prop CHECK_DEADLOCK FALSE"
	got, err := Check(source, config)
	if err != nil || got.Status != Success || got.GeneratedStates != 4 || got.DistinctStates != 1 || got.Depth != 1 {
		t.Fatalf("got %+v, %v; want upstream success 4 1 1", got, err)
	}
}

func TestGeneralTemporalRejectsUpstreamErrors(t *testing.T) {
	for _, formula := range []string{"<>[]TRUE => TRUE", "([]<>(x = 1) <=> <>[](x = 2))", "~(\\E i \\in {} : <>[](x=i))"} {
		source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nP == " + formula + "\n===="
		_, err := Check(source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE")
		if err == nil {
			t.Fatalf("%s: expected the upstream temporal error", formula)
		}
	}
}
