package checker

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestJavaTLCSimulationActions(t *testing.T) {
	cases := []struct {
		name, definitions, next string
		want                    []string
	}{
		{"disjunction", "A == x' = 1\nB == x' \\in {2, 3}", "A \\/ B", []string{"ACTION\tA\t1", "ACTION\tB\t2\t3"}},
		{"quantified", "", "\\E n \\in {1, 2} : x' = n", []string{"ACTION\tNext\t1", "ACTION\tNext\t2"}},
		{"empty", "", "\\E n \\in {} : x' = n", []string{}},
		{"state dependent", "", "\\E n \\in {x} : x' = n", []string{"ACTION\tNext\t0"}},
		{"incomplete successor", "", "x = 0", []string{"ACTION\tNext\tnull"}},
		{"operator arguments", "Step(n) == x' = n", "Step(1) \\/ Step(2)", []string{"ACTION\tStep\t1", "ACTION\tStep\t2"}},
		{"nested", "Step(n) == x' = n", "\\E n \\in {1, 2} : Step(n) \\/ x' = 3", []string{"ACTION\tStep\t1", "ACTION\tNext\t3", "ACTION\tStep\t2", "ACTION\tNext\t3"}},
		{"let", "", "LET A == x' = 1\n                B == x' = 2\n            IN A \\/ B", []string{"ACTION\tA\t1", "ACTION\tB\t2"}},
		{"state dependent argument", "Step(n) == x' = n", "Step(x)", []string{"ACTION\tStep\t0"}},
		{"dependent bounds", "", "\\E i \\in {1, 2} : \\E j \\in {i, 3} : x' = j", []string{"ACTION\tNext\t1", "ACTION\tNext\t3", "ACTION\tNext\t2", "ACTION\tNext\t3"}},
		{"argument branches", "Step(n) == x' = n \\/ x' = 0", "Step(1) \\/ Step(2)", []string{"ACTION\tStep\t1", "ACTION\tStep\t0", "ACTION\tStep\t2", "ACTION\tStep\t0"}},
		{"let with parameters", "", "LET Step(n) == x' = n IN Step(1) \\/ Step(2)", []string{"ACTION\tStep\t1", "ACTION\tStep\t2"}},
		{"let in quantifier", "", "\\E n \\in {1, 2} : LET Step == x' = n IN Step \\/ x' = 0", []string{"ACTION\tStep\t1", "ACTION\tNext\t0", "ACTION\tStep\t2", "ACTION\tNext\t0"}},
	}
	jar := os.Getenv("TLC_JAR")
	classes := ""
	if jar != "" {
		classes = compileSimulationOracle(t, jar)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\n" + tc.definitions + "\nNext == " + tc.next + "\n====\n"
			got := simulationActionResults(t, source)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("actions = %q; want %q", got, tc.want)
			}
			if jar != "" {
				want := javaSimulationActions(t, jar, classes, source)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("tla-go actions %q; Java actions %q", got, want)
				}
			}
		})
	}
}

func simulationActionResults(t *testing.T, source string) []string {
	t.Helper()
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next")
	if err != nil {
		t.Fatal(err)
	}
	model, err := prepare(module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	model.ctx = context.Background()
	actions, err := model.simulationActions()
	if err != nil {
		t.Fatal(err)
	}
	zero, _ := value.Integer(0)
	result := make([]string, 0, len(actions))
	for _, action := range actions {
		row := "ACTION\t" + action.name
		worker := simulationWorker{model: model}
		states, err := worker.actionStates(action, evaluator.State{"x": zero})
		for _, state := range states {
			if v, ok := state["x"]; ok {
				row += "\t" + v.String()
			} else {
				row += "\tnull"
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, row)
	}
	return result
}

func compileSimulationOracle(t *testing.T, jar string) string {
	t.Helper()
	classes := t.TempDir()
	cmd := exec.Command("javac", "-cp", jar, "-d", classes, filepath.Join("testdata", "simulation", "Actions.java"), filepath.Join("testdata", "simulation", "RandomWalk.java"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile Java action oracle: %v\n%s", err, output)
	}
	return classes
}

func javaSimulationActions(t *testing.T, jar, classes, source string) []string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-cp", jar+string(os.PathListSeparator)+classes, "Actions")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Java action oracle: %v\n%s", err, output)
	}
	result := []string{}
	for line := range strings.Lines(string(output)) {
		if strings.HasPrefix(line, "ACTION\t") {
			result = append(result, strings.TrimSpace(line))
		}
	}
	return result
}

func TestSimulationActionCollectionCancellation(t *testing.T) {
	module, err := parser.ParseModule("---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == \\E n \\in {1, 2} : x' = n\n====")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next")
	if err != nil {
		t.Fatal(err)
	}
	model, err := prepare(module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	model.ctx = ctx
	if _, err := model.simulationActions(); !errors.Is(err, context.Canceled) {
		t.Fatalf("action collection error = %v; want context cancellation", err)
	}
}
