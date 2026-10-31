package checker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

func TestJavaTLCSimulationTraces(t *testing.T) {
	cases := []struct {
		name, definitions, cfg string
		status                 Status
		generated              int
	}{
		{"branching", "Init == x = 0\nNext == x' \\in {0, 1, 2}", "INIT Init NEXT Next CHECK_DEADLOCK FALSE", Success, 16},
		{"empty initial", "Init == x \\in {}\nNext == UNCHANGED x", "INIT Init NEXT Next", EvaluationFailure, 0},
		{"implied initial property", "Init == x = 0\nNext == UNCHANGED x\nP == x = 1", "INIT Init NEXT Next PROPERTY P", Success, 6},
		{"duplicate initials", "Init == x = 0 \\/ x = 0\nNext == UNCHANGED x", "INIT Init NEXT Next CHECK_DEADLOCK FALSE", Success, 7},
		{"initial invariant", "Init == x \\in {0, 1}\nNext == UNCHANGED x\nInv == x = 0", "INIT Init NEXT Next INVARIANT Inv", InvariantViolation, 2},
		{"successor invariant", "Init == x = 0\nNext == x' \\in {0, 1, 2}\nInv == x # 2", "INIT Init NEXT Next INVARIANT Inv", InvariantViolation, 4},
		{"deadlock", "Init == x = 0\nNext == /\\ x = 0 /\\ x' = 1", "INIT Init NEXT Next", Deadlock, 2},
		{"state constraint", "Init == x = 0\nNext == x' \\in {0, 1, 2}\nBound == x # 2", "INIT Init NEXT Next CONSTRAINT Bound CHECK_DEADLOCK FALSE", Success, 4},
		{"action constraint", "Init == x = 0\nNext == x' \\in {0, 1, 2}\nBound == x' # 2", "INIT Init NEXT Next ACTION_CONSTRAINT Bound CHECK_DEADLOCK FALSE", Success, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nVARIABLE x\n" + tc.definitions + "\n====\n"
			model := prepareSimulationTest(t, source, tc.cfg)
			result, err := model.simulate(1, 5, 1)
			if err != nil || result.Status != tc.status || result.GeneratedStates != tc.generated {
				t.Fatalf("Go simulation: %+v, %v; want status %s and %d generated", result, err, tc.status, tc.generated)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaSimulation(t, jar, source, tc.cfg)
				if java.status != tc.status || java.generated >= 0 && java.generated != result.GeneratedStates {
					t.Fatalf("Go: %+v; Java status %s, generated %d\n%s", result, java.status, java.generated, java.output)
				}
				if result.Violation != nil && len(java.trace) > 0 {
					var got []string
					for _, step := range result.Violation.Trace {
						got = append(got, step.State["x"].String())
					}
					if !reflect.DeepEqual(got, java.trace) {
						t.Fatalf("Go trace %q; Java trace %q\n%s", got, java.trace, java.output)
					}
				}
			}
		})
	}
}

func prepareSimulationTest(t *testing.T, source, configuration string) *model {
	t.Helper()
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(configuration)
	if err != nil {
		t.Fatal(err)
	}
	model, err := prepare(module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	model.ctx = context.Background()
	return model
}

type javaSimulationResult struct {
	exitCode  int
	status    Status
	generated int
	trace     []string
	output    string
}

func runJavaSimulation(t *testing.T, jar, source, cfg string) javaSimulationResult {
	t.Helper()
	dir := t.TempDir()
	for name, text := range map[string]string{"M.tla": source, "M.cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "1", "-depth", "5", "M.tla")
	cmd.Dir = dir
	output, _ := cmd.CombinedOutput()
	if cmd.ProcessState == nil {
		t.Fatalf("Java did not start: %s", output)
	}
	status := Success
	switch cmd.ProcessState.ExitCode() {
	case 11:
		status = Deadlock
	case 12:
		status = InvariantViolation
	case 13:
		status = LivenessViolation
	case 76, 255:
		status = EvaluationFailure
	case 0:
	default:
		t.Fatalf("Java exit %d\n%s", cmd.ProcessState.ExitCode(), output)
	}
	if strings.Contains(string(output), "@!@!@STARTMSG 2113:1") {
		status = EvaluationFailure
	}
	result := javaSimulationResult{exitCode: cmd.ProcessState.ExitCode(), status: status, generated: -1, output: string(output)}
	stats := regexp.MustCompile(`(?m): ([0-9]+) states generated, -1 distinct states found`).FindStringSubmatch(result.output)
	if len(stats) > 0 {
		result.generated, _ = strconv.Atoi(stats[1])
	}
	if strings.Contains(result.output, "@!@!@STARTMSG 2217:4") {
		for _, match := range regexp.MustCompile(`(?m)^x = ([^\r\n]+)$`).FindAllStringSubmatch(result.output, -1) {
			result.trace = append(result.trace, match[1])
		}
	}
	return result
}

func TestJavaTLCSimulationUpstreamModels(t *testing.T) {
	dir := filepath.Join("..", "cmd", "tla-go", "testdata", "java-tlc", "multiple-config", "multi-trace")
	sourceBytes, err := os.ReadFile(filepath.Join(dir, "BasicMultiTrace.tla"))
	if err != nil {
		t.Fatal(err)
	}
	source := strings.Replace(string(sourceBytes), "MODULE BasicMultiTrace", "MODULE M", 1)
	for _, tc := range []struct {
		name   string
		status Status
	}{
		{"MC", Success},
		{"MCInv", InvariantViolation},
		{"MCInvInitState", InvariantViolation},
		{"MCActionProp", LivenessViolation},
		{"MCWithConstraint", Success},
		{"MCWithActionConstraint", Success},
		{"MCBadInvInitState", EvaluationFailure},
		{"MCBadInvNonInitState", EvaluationFailure},
		{"MCActionPropBadEval", EvaluationFailure},
		{"MCUnderspecInit", EvaluationFailure},
		{"MCUnderspecNext", EvaluationFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfgBytes, err := os.ReadFile(filepath.Join(dir, tc.name+".cfg"))
			if err != nil {
				t.Fatal(err)
			}
			cfgText := string(cfgBytes) + "\nCHECK_DEADLOCK FALSE\n"
			module, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse(cfgText)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			session := newInstanceSession(module.Name)
			session.sourceUnits[module.Name] = module
			module, err = resolveExtensionsScoped(ctx, module, nil, session)
			if err != nil {
				t.Fatal(err)
			}
			module, err = resolveInstancesScoped(ctx, module, nil, session)
			if err != nil {
				t.Fatal(err)
			}
			model, err := prepareWithSources(module, cfg, Options{}, session.sourceUnits)
			if err != nil {
				t.Fatal(err)
			}
			model.ctx = ctx
			got, err := model.simulate(1, 5, 1)
			if err != nil || got.Status != tc.status {
				t.Fatalf("Go simulation: %+v, %v; want %s", got, err, tc.status)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaSimulation(t, jar, source, cfgText)
				if java.status != got.Status || java.generated >= 0 && java.generated != got.GeneratedStates {
					t.Fatalf("Go: %+v; Java status %s, generated %d\n%s", got, java.status, java.generated, java.output)
				}
				if got.Failure != nil {
					marker := fmt.Sprintf("@!@!@STARTMSG %d:1", got.Failure.ToolCode)
					if got.Failure.ExitCode != java.exitCode || !strings.Contains(java.output, marker) {
						t.Fatalf("Go failure %+v; Java exit %d\n%s", got.Failure, java.exitCode, java.output)
					}
				}
			}
		})
	}
}
