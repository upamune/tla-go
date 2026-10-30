package checker

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamBagsTest(t *testing.T) {
	source, err := os.ReadFile("testdata/bags/BagsTest.tla")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile("testdata/bags/BagsTest.cfg")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Check(string(source), string(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != Success || result.GeneratedStates != 0 || result.DistinctStates != 0 || result.Depth != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestBagsWithNaturals(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Bags, Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\nBound == x < 2\nInv == x \\in Nat\n===="
	result, err := Check(source, "INIT Init NEXT Next INVARIANT Inv CONSTRAINT Bound CHECK_DEADLOCK FALSE")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != Success || result.DistinctStates != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestBagErrorsFailAssumptions(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Bags\nASSUME BagCardinality(<<0>>) = 0\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n===="
	result, err := Check(source, "INIT Init NEXT Next")
	if err != nil {
		t.Fatal(err)
	}
	want := "Applying BagCardinality to the following value,\nwhich is not a bag:\n<<0>>"
	if result.Status != AssumptionViolation || result.Violation == nil || result.Violation.Message != want || result.GeneratedStates != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestBagsDoesNotExportNaturals(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Bags\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\n===="
	if _, err := Check(source, "INIT Init NEXT Next"); err == nil {
		t.Fatal("Bags must not export the Naturals operators")
	}
}

func TestUpstreamBagsModels(t *testing.T) {
	f, err := os.Open("testdata/upstream-bags-checks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var sample struct {
			Name, Source, Config string
			Want                 expectedCheck
		}
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(sample.Name, func(t *testing.T) {
			result, err := Check(sample.Source, sample.Config)
			if err != nil {
				t.Fatal(err)
			}
			got := project(result)
			got.Trace = nil
			if !reflect.DeepEqual(got, sample.Want) {
				t.Fatalf("got %#v, want %#v", got, sample.Want)
			}
			if result.Status == LivenessViolation {
				checkLasso(t, sample.Source, sample.Config, result.Violation)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("got %d cases, want 5", count)
	}
}

// Diagnostics are taken from the pinned upstream Bags unit tests.
func TestUpstreamBagAssumptionErrors(t *testing.T) {
	for _, tc := range []struct{ expr, message string }{
		{"BagCardinality(<<0>>)", "Applying BagCardinality to the following value,\nwhich is not a bag:\n<<0>>"},
		{"<<0>> (+) EmptyBag", "The first argument of (+) should be a bag, but instead it is:\n<<0>>"},
		{"EmptyBag (-) 1", "The second argument of (-) should be a bag, but instead it is:\n1"},
		{`BagIn(1, <<"a">>)`, "The second argument of BagIn should be a bag, but instead it is:\n<<\"a\">>"},
		{"BagUnion({1, 2})", "Attempted to apply BagUnion to the following set, whose\nelement is not a bag:\n{1, 2}"},
		{"BagUnion(Nat)", "Applying BagUnion to the following value,\nwhich is not a finite enumerable set:\nNat"},
		{"SetToBag(Nat)", "Applying BagToSet to the following value,\nwhich is not a function with a finite domain:\nNat"},
		{"1 \\sqsubseteq EmptyBag", "Applying \\sqsubseteq to the following value,\nwhich is not a function with a finite domain:\n1"},
		{"BagOfAll(LAMBDA n : n, 1)", "The second argument of BagOfAll should be a function with a finite domain, but instead it is:\n1"},
		{"<<2147483647>> (+) <<1>>", "Overflow when computing 2147483647+1"},
		{`SubBag(<<"a">>)`, "The first argument of >= should be an integer, but instead it is:\n\"a\""},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Bags, Naturals\nASSUME (" + tc.expr + ") = (" + tc.expr + ")\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n===="
			result, err := Check(source, "INIT Init NEXT Next")
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != AssumptionViolation || result.Violation == nil || result.Violation.Message != tc.message {
				t.Fatalf("got %+v, want %q", result, tc.message)
			}
		})
	}
}

func TestUpstreamBagsPrintedSuites(t *testing.T) {
	for _, tc := range []struct {
		name string
		want []string
	}{
		{"test36", []string{`"Test 1 OK"  TRUE`, `"Test 2 OK"  TRUE`, `"Test 3 OK"  TRUE`, `"Test 4 OK"  TRUE`, `"Test 5 OK"  TRUE`, `"Test 6 OK"  TRUE`, `"Test 7 OK"  TRUE`, `"Test 8 OK"  TRUE`, `"Test 9 OK"  TRUE`, `"Test 10 OK"  TRUE`, `"TLC Fails Tests 11 & 12 because SubBag not implemented"  TRUE`, `"Test 13 OK"  TRUE`, `"Test 14 OK"  TRUE`, `"Test 15 OK"  TRUE`, `"Test 16 OK"  TRUE`}},
		{"test50", []string{`"Test 1 OK"  TRUE`, `"Test 2 OK"  TRUE`, `"Test 3 OK"  TRUE`, `"Test 4 OK"  TRUE`, `"Test 5 OK"  TRUE`, `"Test 6 OK"  TRUE`, `"Test 7 OK"  TRUE`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/bags/" + tc.name + ".tla")
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := os.ReadFile("testdata/bags/" + tc.name + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			var lines []string
			result, err := CheckWithOptions(context.Background(), string(source), string(cfg), Options{OnPrint: func(line string) { lines = append(lines, line) }})
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != Success || result.GeneratedStates != 2 || result.DistinctStates != 1 || result.Depth != 1 || !reflect.DeepEqual(lines, tc.want) {
				t.Fatalf("result: %+v\nlines: %#v\nwant: %#v", result, lines, tc.want)
			}
		})
	}
}
