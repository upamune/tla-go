package checker

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

func TestUpstreamTLCStatistics(t *testing.T) {
	source, err := os.ReadFile("testdata/tlc-statistics/Statistics.tla")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile("testdata/tlc-statistics/Statistics.cfg")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/tlc-statistics/Statistics.out")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	result, err := CheckWithOptions(context.Background(), string(source), string(cfg), Options{ModernTLCStatistics: true, OnPrint: func(s string) { lines = append(lines, s) }})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != Success || result.GeneratedStates != 15 || result.DistinctStates != 5 || result.Depth != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := strings.Join(lines, "\n"); got != strings.TrimSpace(string(want)) {
		t.Fatalf("printed output differs:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestTLCGeneratedStopsAtViolation(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nNext == x' \\in {1, 2, 3} /\\ PrintT(<<x', TLCGet(\"generated\")>>)\nInv == x # 2\n===="
	var lines []string
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv", Options{ModernTLCStatistics: true, OnPrint: func(s string) { lines = append(lines, s) }})
	if err != nil || result.Status != InvariantViolation || result.GeneratedStates != 3 || result.DistinctStates != 3 || !reflect.DeepEqual(lines, []string{"<<1, 1>>", "<<2, 2>>"}) {
		t.Fatalf("got %+v, err %v, lines %q", result, err, lines)
	}
}

func TestTLCPauseCallback(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nNext == x < 3 /\\ x' = x + 1 /\\ TLCSet(\"pause\", x = 1)\n===="
	var lines []string
	var pauses []int
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next CHECK_DEADLOCK FALSE", Options{OnPrint: func(s string) { lines = append(lines, s) }, OnPause: func() { pauses = append(pauses, len(lines)) }})
	if err != nil || result.Status != Success || result.GeneratedStates != 4 || result.DistinctStates != 4 || result.Depth != 4 || !reflect.DeepEqual(lines, []string{"Press enter to resume model checking."}) || !reflect.DeepEqual(pauses, []int{1}) {
		t.Fatalf("got %+v, err %v, lines %q, pauses %v", result, err, lines, pauses)
	}
}

func TestUpstreamRuntimeModels(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		status                     Status
		generated, distinct, depth int
		trace                      []string
	}{
		{"Runtime", Success, 14, 8, 4, nil},
		{"Distinct", InvariantViolation, 10, 6, 3, []string{"1", "3", "5"}},
		{"Generated", InvariantViolation, 8, 5, 3, []string{"0", "2", "4"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/runtime/Runtime.tla")
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := os.ReadFile("testdata/runtime/" + tc.name + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			result, err := CheckWithOptions(context.Background(), string(source), string(cfg), Options{ModernTLCStatistics: true})
			if err != nil {
				t.Fatal(err)
			}
			var trace []string
			if result.Violation != nil {
				for _, step := range result.Violation.Trace {
					trace = append(trace, step.State["x"].String())
				}
			}
			if result.Status != tc.status || result.GeneratedStates != tc.generated || result.DistinctStates != tc.distinct || result.Depth != tc.depth || !reflect.DeepEqual(trace, tc.trace) {
				t.Fatalf("result: %+v, trace: %q", result, trace)
			}
		})
	}
}

func TestUpstreamTLCSetModels(t *testing.T) {
	source, err := os.ReadFile("testdata/tlc-set/TLCSet.tla")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"TLCSet", "TLCSetPost"} {
		t.Run(name, func(t *testing.T) {
			cfg, err := os.ReadFile("testdata/tlc-set/" + name + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			result, err := CheckWithOptions(context.Background(), string(source), string(cfg)+"\nCHECK_DEADLOCK FALSE", Options{ModernTLCStatistics: true})
			if err != nil || result.Status != Success || result.GeneratedStates != 4225 || result.DistinctStates != 4225 || result.QueuedStates != 1 || result.Depth != 4225 {
				t.Fatalf("got %+v, %v", result, err)
			}
		})
	}
}

func TestUpstreamTLCAction(t *testing.T) {
	source, err := os.ReadFile("testdata/tlc-action/Action.tla")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile("testdata/tlc-action/Action.cfg")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Check(string(source), string(cfg))
	if err != nil || result.Status != Success || result.GeneratedStates != 3 || result.DistinctStates != 3 || result.Depth != 3 {
		t.Fatalf("got %+v, %v", result, err)
	}
}

func TestUpstreamPostcondition(t *testing.T) {
	source, err := os.ReadFile("testdata/postconditions/Postconditions.tla")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile("testdata/postconditions/False.cfg")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Check(string(source), string(cfg))
	if err != nil || result.Status != PostconditionViolation || result.Violation == nil || result.Violation.Name != "FiveStates" || result.Violation.Message != "Postcondition FiveStates is false." || result.DistinctStates != 4 {
		t.Fatalf("got %+v, %v", result, err)
	}
}

func TestNoStateSkipsPostcondition(t *testing.T) {
	result, err := Check("---- MODULE M ----\nPost == FALSE\n====", "POSTCONDITION Post")
	if err != nil || result.Status != Success {
		t.Fatalf("got %+v, %v", result, err)
	}
}

func BenchmarkTLCSetModel(b *testing.B) {
	source, err := os.ReadFile("testdata/tlc-set/TLCSet.tla")
	if err != nil {
		b.Fatal(err)
	}
	cfg, err := os.ReadFile("testdata/tlc-set/TLCSet.cfg")
	if err != nil {
		b.Fatal(err)
	}
	config := string(cfg) + "\nCHECK_DEADLOCK FALSE"
	b.ReportAllocs()
	for b.Loop() {
		result, err := CheckWithOptions(context.Background(), string(source), config, Options{ModernTLCStatistics: true})
		if err != nil || result.Status != Success {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}

func TestUpstreamLiveLevels(t *testing.T) {
	source, err := os.ReadFile("testdata/tlc-live-level/LiveLevel.tla")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ReachesThree", "StepsUp", "LevelsAtMostTwo", "LevelsFromOne", "ReachesFour"} {
		t.Run(name, func(t *testing.T) {
			cfg, err := os.ReadFile("testdata/tlc-live-level/" + name + ".cfg")
			if err != nil {
				t.Fatal(err)
			}
			result, err := Check(string(source), string(cfg))
			if err != nil {
				t.Fatal(err)
			}
			if result.GeneratedStates != 5 || result.DistinctStates != 3 || result.Depth != 2 {
				t.Fatalf("unexpected counts: %+v", result)
			}
			if name != "ReachesFour" {
				if result.Status != Success {
					t.Fatalf("got %+v", result)
				}
				return
			}
			if result.Status != LivenessViolation || result.Violation == nil || result.Violation.LoopStart == nil || *result.Violation.LoopStart != 0 {
				t.Fatalf("got %+v", result)
			}
			var trace []string
			for _, step := range result.Violation.Trace {
				trace = append(trace, step.State["x"].String())
			}
			if !reflect.DeepEqual(trace, []string{"0", "1", "2"}) {
				t.Fatalf("got trace %q", trace)
			}
		})
	}
}

func TestUpstreamLevelMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-level-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var sample struct {
			Name, Source, Config, Status string
			Violation                    *struct {
				Name       string
				Properties []string
			}
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
			if string(result.Status) != sample.Status {
				t.Fatalf("got %s, want %s", result.Status, sample.Status)
			}
			if result.Status == LivenessViolation {
				if sample.Violation == nil || result.Violation == nil || result.Violation.Name != sample.Violation.Name || !reflect.DeepEqual(result.Violation.Properties, sample.Violation.Properties) {
					t.Fatalf("got violation %+v; want %+v", result.Violation, sample.Violation)
				}
				checkLevelLasso(t, sample.Source, sample.Config, result.Violation)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 320 {
		t.Fatalf("got %d cases, want 320", count)
	}
}

func checkLevelLasso(t *testing.T, source, configText string, v *Violation) {
	t.Helper()
	if v == nil || v.LoopStart == nil || *v.LoopStart < 0 || *v.LoopStart >= len(v.Trace) {
		t.Fatalf("invalid lasso: %+v", v)
	}
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(configText)
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepare(module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	checkTraceTransitions(t, m, v)
	checkFairLasso(t, m, v)
}

func TestTLCRegistersArePerCheck(t *testing.T) {
	set := "---- MODULE M ----\nEXTENDS TLC\nASSUME TLCSet(1, 7) /\\ TLCSet(\"s:key\", \"ok\") /\\ TLCGet(1) = 7 /\\ TLCGet(\"s:key\") = \"ok\" /\\ TLCGet(\"all\")[1] = <<7>> /\\ TLCGet(\"all:named\")[\"s:key\"] = <<\"ok\">>\n===="
	result, err := Check(set, "")
	if err != nil || result.Status != Success {
		t.Fatalf("got %+v, %v", result, err)
	}
	get := "---- MODULE M ----\nEXTENDS TLC\nASSUME TLCGet(1) = 7\n===="
	result, err = Check(get, "")
	if err != nil || result.Status != AssumptionViolation || result.Violation == nil || result.Violation.Message != "TLCGet(1) was undefined." {
		t.Fatalf("got %+v, %v", result, err)
	}
}

func TestPostconditionsRunAfterViolation(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\nInv == x = 0\nPost == PrintT(\"post\") /\\ FALSE\n===="
	var lines []string
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv POSTCONDITION Post", Options{OnPrint: func(s string) { lines = append(lines, s) }})
	if err != nil || result.Status != InvariantViolation || !reflect.DeepEqual(lines, []string{`"post"`}) {
		t.Fatalf("got %+v, %v, lines %q", result, err, lines)
	}
}
